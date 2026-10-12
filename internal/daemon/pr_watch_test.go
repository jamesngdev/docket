package daemon

import (
	"testing"

	"docket/internal/paths"
	"docket/internal/store"
)

func TestHandleMergeable(t *testing.T) {
	p := paths.New(t.TempDir())
	_ = p.EnsureLayout()
	db, _ := store.Open(p.DB())
	defer db.Close()
	_ = db.CreateProject(store.Project{Name: "x", RepoPath: p.Repo, ActivePhase: "main"})
	d := &Daemon{paths: p, db: db}
	proj, _ := db.Project()
	id, _ := db.AddTicket(store.Ticket{Title: "API"})
	_ = db.UpdateTicketFields(id, map[string]any{"status": store.TWaitReview, "pr_url": "u", "pr_conflict": 1, "conflict_attempts": 2})

	tk, _ := db.Ticket(id)
	d.handleMergeable(proj, tk, nil, "CONFLICTING") // attempts exhausted → user
	if tk, _ = db.Ticket(id); !tk.NeedsUser || tk.LockedBy != "" {
		t.Fatalf("exhausted attempts must flag the user: %+v", tk)
	}
	d.handleMergeable(proj, tk, nil, "MERGEABLE")
	if tk, _ = db.Ticket(id); tk.PRConflict || tk.ConflictAttempts != 0 {
		t.Fatalf("mergeable PR must clear conflict state: %+v", tk)
	}
	d.handleMergeable(proj, tk, nil, "UNKNOWN") // no-op
}
