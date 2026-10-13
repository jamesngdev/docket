package daemon

import (
	"fmt"
	"strings"
	"time"

	"docket/internal/agent"
	"docket/internal/store"
)

// mergeRecheck is how long a PR TechLead left open (e.g. CI still running)
// waits before it is offered to TechLead again.
const mergeRecheck = 15 * time.Minute

// mergeReady reports whether a ticket's PR may be offered for auto-merge:
// Tester-approved, in review, nothing running on it, not flagged, no conflict.
func mergeReady(t store.Ticket) bool {
	return t.Status == store.TWaitReview && t.PRURL != "" && t.ReviewState == store.RApprove &&
		t.LockedBy == "" && !t.NeedsUser && !t.PRConflict
}

// startAutoMerge hands mergeable, approved PRs to a TechLead "merge" run when
// auto-merge is on and TechLead is idle (otherwise the next PR poll retries).
func (d *Daemon) startAutoMerge(p store.Project, ready []store.Ticket) {
	if !p.Settings.AutoMerge || len(ready) == 0 || d.runningRole(agent.RoleTechLead) > 0 {
		return
	}
	var ids []string
	for _, t := range ready {
		_ = d.db.UpdateTicketFields(t.ID, map[string]any{"merge_checked_at": time.Now().Unix()})
		ids = append(ids, fmt.Sprintf("#%d", t.ID))
	}
	if err := d.startTechlead(agent.TaskMerge); err != nil {
		d.db.Log("techlead", "auto-merge: %v", err)
		return
	}
	d.db.Log("techlead", "auto-merge review started for %s", strings.Join(ids, ", "))
}

// mergeMarkdown is MERGE.md: the PRs TechLead must decide on, with everything
// it needs to compare the change against the ticket.
func mergeMarkdown(p store.Project, ts []store.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Auto-merge candidates\n\nPRs target `%s`. Each was approved by the Tester and GitHub reported no conflicts.\n\n", p.Settings.PRTarget)
	n := 0
	for _, t := range ts {
		if !mergeReady(t) {
			continue
		}
		n++
		fmt.Fprintf(&b, "## Ticket #%d: %s\n- PR: %s\n- Branch: %s · Feature: F%d\n\n### Requirements\n%s\n\n### Acceptance criteria\n%s\n\n### Tester review\n%s\n\n",
			t.ID, t.Title, t.PRURL, t.Branch, t.FeatureID, t.Requirements, t.Acceptance, t.ReviewSummary)
	}
	if n == 0 {
		b.WriteString("_No ticket is ready any more — finish with docket run done._\n")
	}
	return b.String()
}
