package daemon

import (
	"fmt"
	"strconv"

	"docket/internal/agent"
	"docket/internal/gitops"
	"docket/internal/store"
	"docket/internal/tmux"
)

// redoTicket reworks an in-flight ticket after TechLead changed it in a ticket
// chat: running Developer/Tester runs are stopped, review state is reset and the
// ticket goes back to pending. Dispatch then restarts the Developer on the SAME
// branch/worktree (existing PR is reused) with the change request in TICKET.md.
func (d *Daemon) redoTicket(arg string) error {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return fmt.Errorf("redo: bad ticket id %q", arg)
	}
	t, err := d.db.Ticket(id)
	if err != nil {
		return err
	}
	switch t.Status {
	case store.TDone:
		return fmt.Errorf("ticket #%d is merged; create a follow-up ticket instead", id)
	case store.TOutdated:
		return fmt.Errorf("ticket #%d is outdated", id)
	case store.TBacklog, store.TPending:
		return nil // nothing started yet: the edited text is picked up on dispatch
	}
	if t.LockedBy != "" {
		if r, err := d.db.RunByID(t.LockedBy); err == nil && r.Status == store.RunRunning {
			tmux.Kill(r.Session)
			_ = d.db.FinishRun(r.ID, store.RunStopped)
			if r.Role == agent.RoleTester {
				p, _ := d.project()
				gitops.RemoveWorktree(p.RepoPath, d.paths.Worktree(fmt.Sprintf("review-%d", id)))
			}
			d.db.Log(r.Role, "%s stopped: ticket #%d is being reworked", r.ID, id)
		}
	}
	if err := d.db.UpdateTicketFields(id, map[string]any{
		"status": store.TPending, "locked_by": "", "needs_user": 0,
		"review_state": "", "review_summary": "", "review_rounds": 0,
	}); err != nil {
		return err
	}
	d.db.Log("techlead", "ticket #%d sent back for rework on %s", id, t.Branch)
	return nil
}
