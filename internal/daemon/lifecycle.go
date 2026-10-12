package daemon

import (
	"fmt"

	"docket/internal/agent"
	"docket/internal/gitops"
	"docket/internal/store"
	"docket/internal/tmux"
)

// reapRuns finalizes runs: callback received ⇒ close the session and mark done;
// session gone without callback ⇒ failed.
func (d *Daemon) reapRuns() {
	runs, err := d.db.RunningRuns()
	if err != nil {
		return
	}
	for _, r := range runs {
		alive := tmux.Has(r.Session)
		if alive && r.Callback {
			tmux.Kill(r.Session)
			alive = false
		}
		if alive {
			continue
		}
		status := store.RunFailed
		if r.Callback {
			status = store.RunDone
		}
		_ = d.db.FinishRun(r.ID, status)
		d.db.Log(r.Role, "%s %s %s", r.ID, status, r.Summary)
		d.onFinished(r, r.Callback)
	}
}

func (d *Daemon) onFinished(r store.Run, ok bool) {
	switch r.Role {
	case agent.RoleTechLead:
		if ok && r.Task == agent.TaskBootstrap {
			if err := d.composeUp(); err != nil {
				d.db.Log("docker", "compose up: %v", err)
			}
		}
		d.startQueuedTechlead()
	case agent.RoleDeveloper:
		t, err := d.db.Ticket(r.TicketID)
		if err != nil || t.LockedBy != r.ID {
			return // ticket merged, re-dispatched or edited meanwhile
		}
		if ok && t.Status == store.TWaitReview {
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"locked_by": "", "review_state": ""})
			return
		}
		if r.Task == agent.TaskResolveConflicts {
			// The PR still exists: keep it in review, but the user has to look at it.
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TWaitReview, "locked_by": "", "needs_user": 1})
			d.db.Log("developer", "ticket #%d: conflicts not resolved automatically (needs user)", t.ID)
			return
		}
		// No PR: back to pending, keep the worktree, flag for the user to avoid a crash loop.
		_ = d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TPending, "locked_by": "", "needs_user": 1})
		d.db.Log("developer", "ticket #%d returned to pending without PR (needs user)", t.ID)
	case agent.RoleTester:
		p, _ := d.project()
		gitops.RemoveWorktree(p.RepoPath, d.paths.Worktree(fmt.Sprintf("review-%d", r.TicketID)))
		t, err := d.db.Ticket(r.TicketID)
		if err != nil || t.LockedBy != r.ID {
			return
		}
		fields := map[string]any{"locked_by": ""}
		if t.ReviewState == "" {
			fields["review_state"], fields["needs_user"] = store.RError, 1
		}
		_ = d.db.UpdateTicketFields(t.ID, fields)
	}
}

// reviewLoop re-dispatches the Developer when the Tester requested changes.
func (d *Daemon) reviewLoop() {
	p, err := d.project()
	if err != nil {
		return
	}
	all, _ := d.db.AllTickets()
	for _, t := range all {
		if t.Status != store.TWaitReview || t.ReviewState != store.RRequestChanges || t.NeedsUser || t.LockedBy != "" {
			continue
		}
		if t.ReviewRounds >= p.Settings.MaxReviewRounds {
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"needs_user": 1})
			d.db.Log("review", "ticket #%d reached %d review rounds; waiting for user", t.ID, t.ReviewRounds)
			continue
		}
		if d.runningRole(agent.RoleDeveloper) >= p.Settings.MaxParallelAgents {
			return
		}
		t.ReviewRounds++
		_ = d.db.UpdateTicketFields(t.ID, map[string]any{"review_rounds": t.ReviewRounds})
		if err := d.startDeveloper(p, t, all); err != nil {
			d.db.Log("review", "re-dispatch #%d: %v", t.ID, err)
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TWaitReview, "review_rounds": t.ReviewRounds - 1,
				"needs_user": 1, "locked_by": ""})
		}
	}
}

// stopRun kills a run. restart=false parks the ticket (developer → backlog);
// restart=true makes it immediately eligible again.
func (d *Daemon) stopRun(id string, restart bool) error {
	r, err := d.db.RunByID(id)
	if err != nil {
		return fmt.Errorf("run %s: %w", id, err)
	}
	running := r.Status == store.RunRunning
	if running {
		tmux.Kill(r.Session)
		_ = d.db.FinishRun(r.ID, store.RunStopped)
		d.db.Log(r.Role, "%s stopped by user", r.ID)
	}
	if r.Role == agent.RoleTechLead {
		if restart {
			return d.startTechlead(r.Task) // re-running any TechLead task is safe
		}
		d.startQueuedTechlead()
		return nil
	}
	if !running {
		return fmt.Errorf("run %s already finished (%s); approve the ticket with a to resume it", r.ID, r.Status)
	}
	if t, err := d.db.Ticket(r.TicketID); err != nil || t.LockedBy != r.ID {
		return nil // ticket no longer belongs to this run
	}
	switch r.Role {
	case agent.RoleDeveloper:
		status := store.TBacklog
		if restart {
			status = store.TPending
		}
		return d.db.UpdateTicketFields(r.TicketID, map[string]any{"status": status, "locked_by": "", "needs_user": 0})
	case agent.RoleTester:
		p, _ := d.project()
		gitops.RemoveWorktree(p.RepoPath, d.paths.Worktree(fmt.Sprintf("review-%d", r.TicketID)))
		fields := map[string]any{"locked_by": "", "needs_user": 1}
		if restart {
			fields["needs_user"], fields["review_state"] = 0, ""
		}
		return d.db.UpdateTicketFields(r.TicketID, fields)
	}
	return nil
}
