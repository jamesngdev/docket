package support

import (
	"fmt"
	"strings"
	"time"

	"docket/internal/sh"
	"docket/internal/store"
)

// ticketsMarkdown renders every ticket in full (what `docket show ticket` prints),
// since Backend Support has no shell to ask for it.
func ticketsMarkdown(ts []store.Ticket) string {
	var b strings.Builder
	b.WriteString("# Tickets (active phase)\n\n")
	if len(ts) == 0 {
		b.WriteString("_No tickets yet._\n")
	}
	for _, t := range ts {
		fmt.Fprintf(&b, "## #%d %s\n- Status: %s · Feature: F%d · Depends on: %s\n", t.ID, t.Title, t.Status, t.FeatureID, orDefault(store.JoinIDs(t.DependsOn), "–"))
		if t.Branch != "" || t.PRURL != "" {
			fmt.Fprintf(&b, "- Branch: %s · PR: %s\n", orDefault(t.Branch, "–"), orDefault(t.PRURL, "–"))
		}
		if t.ReviewState != "" {
			fmt.Fprintf(&b, "- Review: %s (round %d)\n", t.ReviewState, t.ReviewRounds)
		}
		fmt.Fprintf(&b, "\n### Requirements\n%s\n\n### Acceptance criteria\n%s\n\n", t.Requirements, t.Acceptance)
		if t.ReviewSummary != "" {
			fmt.Fprintf(&b, "### Last review\n%s\n\n", t.ReviewSummary)
		}
	}
	return b.String()
}

// gitLogMarkdown is the recent history of the base branch (files touched per commit).
func gitLogMarkdown(wt, base string) string {
	out, err := sh.RunTimeout(30*time.Second, wt, "git", "log", "-50", "--date=short", "--stat=120,20",
		"--format=### %h %ad %an%n%s%n")
	if err != nil {
		out = "_git log unavailable: " + err.Error() + "_"
	}
	return fmt.Sprintf("# Recent commits on %s (newest first)\n\n%s\n", base, out)
}
