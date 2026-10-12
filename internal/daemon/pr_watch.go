package daemon

import (
	"strconv"
	"time"

	"docket/internal/agent"
	"docket/internal/gh"
	"docket/internal/gitops"
	"docket/internal/store"
)

const maxConflictAttempts = 2

// pollPRs checks every open ticket PR: merged → done; closed → needs user;
// conflicting → auto-resolve with a Developer run on the ticket's branch.
func (d *Daemon) pollPRs() {
	p, err := d.project()
	if err != nil {
		return
	}
	all, _ := d.db.AllTickets()
	merged := false
	for _, t := range all {
		if t.Status != store.TWaitReview || t.PRURL == "" || t.LockedBy != "" {
			continue // an agent is still working on it; poll again later
		}
		state, mergeable, err := gh.PRStatus(p.RepoPath, t.PRURL)
		if err != nil {
			continue
		}
		switch state {
		case "MERGED":
			merged = true
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TDone, "locked_by": "", "needs_user": 0, "pr_conflict": 0})
			gitops.RemoveWorktree(p.RepoPath, d.paths.Worktree(strconv.FormatInt(t.ID, 10)))
			d.db.Log("github", "PR merged; ticket #%d done", t.ID)
		case "CLOSED":
			if !t.NeedsUser {
				_ = d.db.UpdateTicketFields(t.ID, map[string]any{"needs_user": 1})
				d.db.Log("github", "PR closed without merge; ticket #%d needs user", t.ID)
			}
		case "OPEN":
			d.handleMergeable(p, t, all, mergeable)
		}
	}
	if merged {
		// The base branch moved: re-check the other PRs soon (GitHub needs a few
		// seconds to recompute mergeability, so not on this very tick).
		d.lastPoll = time.Now().Add(-pollPREvery + 15*time.Second)
	}
}

func (d *Daemon) handleMergeable(p store.Project, t store.Ticket, all []store.Ticket, mergeable string) {
	switch mergeable {
	case "MERGEABLE":
		if t.PRConflict || t.ConflictAttempts > 0 {
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"pr_conflict": 0, "conflict_attempts": 0})
		}
	case "CONFLICTING":
		if !t.PRConflict {
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"pr_conflict": 1})
			d.db.Log("github", "PR of ticket #%d has merge conflicts", t.ID)
		}
		if t.NeedsUser {
			return
		}
		if t.ConflictAttempts >= maxConflictAttempts {
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"needs_user": 1})
			d.db.Log("github", "ticket #%d: %d auto-resolve attempts failed; needs user", t.ID, t.ConflictAttempts)
			return
		}
		if d.runningRole(agent.RoleDeveloper) >= p.Settings.MaxParallelAgents {
			return // retried on the next poll when a slot is free
		}
		_ = d.db.UpdateTicketFields(t.ID, map[string]any{"conflict_attempts": t.ConflictAttempts + 1})
		t.PRConflict = true
		if err := d.startDeveloperTask(p, t, all, agent.TaskResolveConflicts); err != nil {
			d.db.Log("github", "resolve conflicts #%d: %v", t.ID, err)
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"status": store.TWaitReview, "locked_by": "", "needs_user": 1})
		}
	}
	// UNKNOWN: GitHub is still computing; the next poll decides.
}
