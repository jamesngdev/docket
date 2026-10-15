package daemon

import (
	"fmt"
	"os"
	"strconv"

	"docket/internal/agent"
	"docket/internal/extskills"
	"docket/internal/gh"
	"docket/internal/gitops"
	"docket/internal/store"
)

// launch starts an agent in tmux and records the run. Ticket runs of a
// Developer or Tester resume that role's previous Claude session on the ticket
// when it still exists, so review rounds, redos and conflict fixes keep the
// knowledge of the earlier run instead of re-exploring the codebase.
func (d *Daemon) launch(p store.Project, role, task, runID string, ticketID int64, workdir string) error {
	var extra []extskills.Skill
	if extskills.RolesWithTeamSkills[role] {
		extra = d.teamSkills()
	}
	ctxDir := d.paths.RunContext(runID)
	s := agent.Spawn{Extra: extra,
		Paths: d.paths, Slug: p.Slug, Role: role, Model: d.modelFor(p, role, ticketID), RunID: runID,
		TicketID: ticketID, Workdir: workdir, Prompt: agent.Kickoff(role, task, ctxDir, ticketID),
		Token: d.token(), ClaudeSession: agent.NewSessionID(),
	}
	if d.resumable(role, task, ticketID) {
		if prev := d.db.LastClaudeSession(role, ticketID); agent.SessionExists(d.paths.ClaudeHome(role), prev) {
			s.ClaudeSession, s.Resume = prev, true
			s.Prompt = agent.ResumeKickoff(role, task, ctxDir, ticketID)
		}
	}
	session, err := agent.Launch(s)
	if err != nil {
		return err
	}
	if err := d.db.InsertRun(store.Run{ID: runID, Role: role, Task: task, TicketID: ticketID, Session: session, Workdir: workdir,
		ClaudeSession: s.ClaudeSession}); err != nil {
		return err
	}
	resumed := ""
	if s.Resume {
		resumed = ", resumed session"
	}
	d.db.Log(role, "started %s (%s%s)", runID, session, resumed)
	return nil
}

// resumable reports whether this ticket run is a follow-up the previous
// session of the role should continue: a Developer addressing review feedback,
// a change request or merge conflicts, or a Tester re-reviewing after it
// requested changes. Anything else (first run, retry after a failure, reset
// review) starts a fresh session with the full kickoff.
func (d *Daemon) resumable(role, task string, ticketID int64) bool {
	if ticketID == 0 {
		return false
	}
	t, err := d.db.Ticket(ticketID)
	if err != nil {
		return false
	}
	switch role {
	case agent.RoleDeveloper:
		return task == agent.TaskResolveConflicts || t.ChangeRequest != "" ||
			(t.ReviewState == store.RRequestChanges && t.ReviewSummary != "")
	case agent.RoleTester:
		return t.ReviewRounds > 0
	}
	return false
}

// modelFor picks the run's model: a ticket's own model (set by TechLead for
// unusually easy or hard tickets) wins over the project's per-role default.
func (d *Daemon) modelFor(p store.Project, role string, ticketID int64) string {
	if role == agent.RoleDeveloper && ticketID > 0 {
		if t, err := d.db.Ticket(ticketID); err == nil && t.Model != "" {
			return t.Model
		}
	}
	return p.Settings.Models[role]
}

// teamSkills pulls the team skill repos (at most every 10 minutes per repo).
func (d *Daemon) teamSkills() []extskills.Skill {
	skills, errs := extskills.Load()
	for _, err := range errs {
		d.db.Log("skills", "team skills: %v (using cached copy if any)", err)
	}
	return skills
}

func (d *Daemon) runningRole(role string) int {
	runs, _ := d.db.RunningRuns()
	n := 0
	for _, r := range runs {
		if r.Role == role {
			n++
		}
	}
	return n
}

// dispatch starts Developers for dependency-free pending tickets up to the slot limit.
func (d *Daemon) dispatch() {
	p, err := d.project()
	if err != nil {
		return
	}
	free := p.Settings.MaxParallelAgents - d.runningRole(agent.RoleDeveloper)
	if free <= 0 {
		return
	}
	all, err := d.db.AllTickets()
	if err != nil {
		return
	}
	for _, t := range store.Dispatchable(all, p.ActivePhase) {
		if free == 0 {
			return
		}
		if err := d.startDeveloper(p, t, all); err != nil {
			d.db.Log("dispatch", "ticket #%d: %v", t.ID, err)
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TPending, "locked_by": "", "needs_user": 1})
			continue
		}
		free--
	}
}

func (d *Daemon) startDeveloper(p store.Project, t store.Ticket, all []store.Ticket) error {
	return d.startDeveloperTask(p, t, all, agent.TaskImplement)
}

// startDeveloperTask runs a Developer on the ticket's branch for task
// (implement, or resolve-conflicts on an open PR).
func (d *Daemon) startDeveloperTask(p store.Project, t store.Ticket, all []store.Ticket, task string) error {
	if t.Branch == "" {
		t.Branch = fmt.Sprintf("ticket/%d-%s", t.ID, store.Slugify(t.Title))
	}
	wt := d.paths.Worktree(strconv.FormatInt(t.ID, 10))
	if err := gitops.AddTicketWorktree(p.RepoPath, wt, t.Branch, p.Settings.BaseBranch); err != nil {
		return err
	}
	runID := agent.NewRunID(agent.RoleDeveloper)
	if err := d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TProcessing, "locked_by": runID, "branch": t.Branch}); err != nil {
		return err
	}
	if err := d.writeTicketContext(p, t, all, runID); err != nil {
		return err
	}
	return d.launch(p, agent.RoleDeveloper, task, runID, t.ID, wt)
}

// startTesters reviews wait_review tickets in parallel, up to the same slot
// limit as Developers, so reviews never become the pipeline's bottleneck.
func (d *Daemon) startTesters() {
	p, err := d.project()
	if err != nil {
		return
	}
	free := p.Settings.MaxParallelAgents - d.runningRole(agent.RoleTester)
	if free <= 0 {
		return
	}
	all, _ := d.db.AllTickets() // finish reviews of earlier phases too
	for _, t := range all {
		if t.Status != store.TWaitReview || t.ReviewState != "" || t.PRURL == "" || t.NeedsUser || t.LockedBy != "" {
			continue
		}
		if err := d.startTester(p, t, all); err != nil {
			d.db.Log("tester", "ticket #%d: %v", t.ID, err)
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"locked_by": "", "review_state": store.RError, "needs_user": 1})
			continue
		}
		if free--; free == 0 {
			return
		}
	}
}

func (d *Daemon) startTester(p store.Project, t store.Ticket, all []store.Ticket) error {
	branch := t.Branch
	if branch == "" {
		b, err := gh.PRBranch(p.RepoPath, t.PRURL)
		if err != nil {
			return err
		}
		branch = b
	}
	wt := d.paths.Worktree(fmt.Sprintf("review-%d", t.ID))
	if err := gitops.AddDetachedWorktree(p.RepoPath, wt, branch); err != nil {
		return err
	}
	runID := agent.NewRunID(agent.RoleTester)
	if err := d.db.UpdateTicketFields(t.ID, map[string]any{"locked_by": runID}); err != nil {
		return err
	}
	if err := d.writeTicketContext(p, t, all, runID); err != nil {
		return err
	}
	return d.launch(p, agent.RoleTester, "review", runID, t.ID, wt)
}

// recentMerged lists up to 10 most recently created done tickets (likely sources of conflicts).
func recentMerged(all []store.Ticket, exclude int64) []store.Ticket {
	var out []store.Ticket
	for i := len(all) - 1; i >= 0 && len(out) < 10; i-- {
		if all[i].Status == store.TDone && all[i].ID != exclude {
			out = append(out, all[i])
		}
	}
	return out
}

func (d *Daemon) writeTicketContext(p store.Project, t store.Ticket, all []store.Ticket, runID string) error {
	feature, _ := d.db.Feature(t.FeatureID)
	var deps []store.Ticket
	for _, o := range all {
		for _, id := range t.DependsOn {
			if o.ID == id {
				deps = append(deps, o)
			}
		}
	}
	env, _ := os.ReadFile(d.paths.DevDBEnv())
	notes, _ := os.ReadFile(d.paths.RepoNotes())
	third, _ := d.db.ThirdDocs()
	return agent.WriteFiles(d.paths.RunContext(runID), map[string]string{
		"TICKET.md": agent.TicketMarkdown(agent.TicketInput{Project: p, Ticket: t, Feature: feature, Deps: deps, DevEnv: string(env),
			RecentMerged: recentMerged(all, t.ID), RepoNotes: string(notes)}),
		"3TH_DOCUMENT.md": agent.ThirdDocsMarkdown(third),
	})
}
