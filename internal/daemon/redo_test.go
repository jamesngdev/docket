package daemon

import (
	"testing"

	"docket/internal/paths"
	"docket/internal/store"
)

func TestRedoTicket(t *testing.T) {
	p := paths.New(t.TempDir())
	_ = p.EnsureLayout()
	db, err := store.Open(p.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.CreateProject(store.Project{Name: "x", RepoPath: p.Repo, ActivePhase: "main"})
	d := &Daemon{paths: p, db: db}

	id, _ := db.AddTicket(store.Ticket{Title: "API"})
	_ = db.UpdateTicketFields(id, map[string]any{"status": store.TWaitReview, "branch": "ticket/1-api", "pr_url": "u",
		"review_state": store.RRequestChanges, "review_rounds": 2, "needs_user": 1, "locked_by": "gone-run"})
	if err := d.redoTicket("1"); err != nil {
		t.Fatal(err)
	}
	got, _ := db.Ticket(id)
	if got.Status != store.TPending || got.LockedBy != "" || got.NeedsUser || got.ReviewRounds != 0 ||
		got.ReviewState != "" || got.Branch != "ticket/1-api" || got.PRURL != "u" {
		t.Fatalf("redo must reset review and keep branch/PR: %+v", got)
	}
	_ = db.SetTicketStatus(id, store.TDone)
	if err := d.redoTicket("1"); err == nil {
		t.Fatal("merged tickets must not be redone")
	}
}
