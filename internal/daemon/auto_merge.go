package daemon

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"docket/internal/agent"
	"docket/internal/gh"
	"docket/internal/store"
)

// mergeRecheck is how long a PR TechLead left open (e.g. CI still running)
// waits before it is offered to TechLead again.
const mergeRecheck = 15 * time.Minute

// ciPendingLimit is how long an approved PR may wait for CI before TechLead is
// asked anyway (a required check that never reports would block it forever).
const ciPendingLimit = 30 * time.Minute

// mergeReady reports whether a ticket's PR may be offered for auto-merge:
// Tester-approved, in review, nothing running on it, not flagged, no conflict.
func mergeReady(t store.Ticket) bool {
	return t.Status == store.TWaitReview && t.PRURL != "" && t.ReviewState == store.RApprove &&
		t.LockedBy == "" && !t.NeedsUser && !t.PRConflict
}

// startAutoMerge decides on mergeable, approved PRs when auto-merge is on.
// PRs whose CI is still running wait for a later poll. Low-risk PRs (gate
// passed on the PR head, no risk flags, CI green) are squash-merged directly;
// the rest go to one TechLead "merge" run. Reports whether anything merged.
func (d *Daemon) startAutoMerge(p store.Project, ready []store.Ticket) bool {
	if !p.Settings.AutoMerge || len(ready) == 0 {
		return false
	}
	merged := false
	var review []string
	if d.ciPendingSince == nil {
		d.ciPendingSince = map[int64]time.Time{}
	}
	for _, t := range ready {
		head, checks, err := gh.PRHeadAndChecks(p.RepoPath, t.PRURL)
		if err != nil {
			d.db.Log("github", "auto-merge: CI state of ticket #%d: %v", t.ID, err)
			continue
		}
		if checks == "pending" {
			since, seen := d.ciPendingSince[t.ID]
			if !seen {
				d.ciPendingSince[t.ID] = time.Now()
			}
			if !seen || time.Since(since) < ciPendingLimit {
				continue // decided on a later poll, without spending a TechLead run
			}
			d.db.Log("github", "ticket #%d: CI pending for over %s; asking TechLead", t.ID, ciPendingLimit)
		}
		delete(d.ciPendingSince, t.ID)
		if fastMergeable(p.Settings, t, head, checks) {
			if err := gh.MergePR(t.PRURL, head); err != nil {
				d.db.Log("github", "fast merge of ticket #%d failed: %v (sending to TechLead)", t.ID, err)
			} else {
				d.db.Log("github", "ticket #%d merged directly: approved, gate passed, CI green, no risk flags", t.ID)
				d.markMerged(p, t)
				merged = true
				continue
			}
		}
		review = append(review, fmt.Sprintf("#%d", t.ID))
	}
	if len(review) == 0 || d.runningRole(agent.RoleTechLead) > 0 {
		return merged
	}
	for _, t := range ready {
		if slices.Contains(review, fmt.Sprintf("#%d", t.ID)) {
			_ = d.db.UpdateTicketFields(t.ID, map[string]any{"merge_checked_at": time.Now().Unix()})
		}
	}
	if err := d.startTechlead(agent.TaskMerge); err != nil {
		d.db.Log("techlead", "auto-merge: %v", err)
		return merged
	}
	d.db.Log("techlead", "auto-merge review started for %s", strings.Join(review, ", "))
	return merged
}

// fastMergeable reports whether an approved PR can skip the TechLead review:
// the gate passed on exactly the PR head, it raised no risk flag and CI is green.
func fastMergeable(s store.Settings, t store.Ticket, head, checks string) bool {
	return s.MergeReview != store.MergeReviewAll && t.GateSHA != "" && t.GateSHA == head &&
		t.RiskFlags == "" && checks == "pass"
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
		risks := t.RiskFlags
		if risks == "" {
			risks = "none (sent here because CI failed, the PR changed after the gate, or merge review is \"all\")"
		}
		fmt.Fprintf(&b, "## Ticket #%d: %s\n- PR: %s\n- Branch: %s · Feature: F%d\n- Risk flags (focus your review here): %s\n\n### Requirements\n%s\n\n### Acceptance criteria\n%s\n\n### Tester review\n%s\n\n",
			t.ID, t.Title, t.PRURL, t.Branch, t.FeatureID, risks, t.Requirements, t.Acceptance, t.ReviewSummary)
	}
	if n == 0 {
		b.WriteString("_No ticket is ready any more — finish with docket run done._\n")
	}
	return b.String()
}
