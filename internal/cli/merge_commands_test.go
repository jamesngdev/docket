package cli

import (
	"strconv"
	"strings"
	"testing"

	"docket/internal/paths"
	"docket/internal/store"
)

// TestTicketMergeGuards checks every refusal path before gh is ever called.
func TestTicketMergeGuards(t *testing.T) {
	repo := t.TempDir()
	p := paths.New(repo)
	if err := p.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(p.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.CreateProject(store.Project{Name: "Demo", RepoPath: repo})
	t.Setenv("DOCKET_PROJECT", repo)
	t.Setenv("DOCKET_RUN", "techlead-1")
	fid, _ := db.AddFeature("F", "d", "")
	id, _ := db.AddTicket(store.Ticket{FeatureID: fid, Title: "T", Requirements: "r", Acceptance: "a"})
	expect := func(want string) {
		t.Helper()
		err := Run([]string{"ticket", "merge", strconv.FormatInt(id, 10)})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("want error containing %q, got %v", want, err)
		}
	}
	expect("auto-merge is off")
	if err := Run([]string{"automerge", "on"}); err != nil {
		t.Fatal(err)
	}
	expect("without an open PR")
	_ = db.UpdateTicketFields(id, map[string]any{"status": store.TWaitReview, "pr_url": "https://x/pull/1"})
	expect("not approved")
	_ = db.UpdateTicketFields(id, map[string]any{"review_state": store.RApprove, "pr_conflict": 1})
	expect("merge conflicts")
}
