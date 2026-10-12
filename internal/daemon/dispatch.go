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

// launch starts an agent in tmux and records the run.
func (d *Daemon) launch(p store.Project, role, task, runID string, ticketID int64, workdir string) error {
	var extra []extskills.Skill
	if extskills.RolesWithTeamSkills[role] {
		extra = d.teamSkills()
	}
	session, err := agent.Launch(agent.Spawn{Extra: extra,
		Paths: d.paths, Slug: p.Slug, Role: role, Model: p.Settings.Models[role], RunID: runID,
		TicketID: ticketID, Workdir: workdir, Prompt: agent.Kickoff(role, task, d.paths.RunContext(runID), ticketID),
		Token: d.token(),
	})
	if err != nil {
		return err
	}
	if err := d.db.InsertRun(store.Run{ID: runID, Role: role, Task: task, TicketID: ticketID, Session: session, Workdir: workdir}); err != nil {
		return err
	}
	d.db.Log(role, "started %s (%s)", runID, session)
	return nil
}

// teamSkills pulls the team skill repos (fresh on every Developer/TechLead start).
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

// startTesters reviews one wait_review ticket at a time.
func (d *Daemon) startTesters() {
	if d.runningRole(agent.RoleTester) > 0 {
		return
	}
	p, err := d.project()
	if err != nil {
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
		}
		return
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
	third, _ := d.db.ThirdDocs()
	return agent.WriteFiles(d.paths.RunContext(runID), map[string]string{
		"TICKET.md": agent.TicketMarkdown(agent.TicketInput{Project: p, Ticket: t, Feature: feature, Deps: deps, DevEnv: string(env),
			RecentMerged: recentMerged(all, t.ID)}),
		"3TH_DOCUMENT.md": agent.ThirdDocsMarkdown(third),
	})
}
