package cli

import (
	"os"
	"path/filepath"
	"testing"

	"docket/internal/paths"
	"docket/internal/store"
)

func TestParse(t *testing.T) {
	a := parse([]string{"5", "wait_review", "--pr", "https://x/pull/1", "--flag", "--k=v"})
	if len(a.pos) != 2 || a.pos[1] != "wait_review" || a.flags["pr"] != "https://x/pull/1" || a.flags["flag"] != "true" || a.flags["k"] != "v" {
		t.Fatalf("%+v", a)
	}
}

// TestAgentFlow drives the agent-facing CLI against a real project database.
func TestAgentFlow(t *testing.T) {
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
	t.Setenv("DOCKET_RUN", "developer-1")
	_ = db.InsertRun(store.Run{ID: "developer-1", Role: "developer", Session: "s"})

	req := filepath.Join(t.TempDir(), "req.md")
	_ = os.WriteFile(req, []byte("Build it\n"), 0o644)
	mustRun := func(argv ...string) {
		t.Helper()
		if err := Run(argv); err != nil {
			t.Fatalf("%v: %v", argv, err)
		}
	}
	mustRun("question", "add", "--text", "Which DB?")
	mustRun("feature", "add", "--title", "Orders", "--description", "d")
	mustRun("ticket", "add", "--feature", "1", "--title", "Schema", "--requirements-file", req, "--acceptance", "- [ ] ok")
	mustRun("ticket", "add", "--feature", "1", "--title", "API", "--requirements", "r", "--acceptance", "a", "--depends", "1")
	if err := Run([]string{"ticket", "status", "2", "wait_review", "--pr", "u"}); err == nil {
		t.Fatal("a run must not change a ticket it does not own")
	}
	_ = db.UpdateTicketFields(2, map[string]any{"status": store.TProcessing, "locked_by": "developer-1"})
	if err := Run([]string{"ticket", "status", "2", "wait_review"}); err == nil {
		t.Fatal("wait_review without a PR must be rejected")
	}
	mustRun("ticket", "status", "2", "wait_review", "--pr", "https://github.com/o/r/pull/9")
	mustRun("ticket", "update", "1", "--title", "Schema v2", "--depends", "")
	if err := Run([]string{"ticket", "update", "1", "--depends", "1"}); err == nil {
		t.Fatal("self dependency must be rejected")
	}
	if t1, _ := db.Ticket(1); t1.Title != "Schema v2" {
		t.Fatalf("update not applied: %q", t1.Title)
	}
	mustRun("review", "submit", "2", "--result", "request_changes", "--summary", "add tests")
	mustRun("run", "done", "--summary", "PR opened")
	if err := Run([]string{"ticket", "add", "--feature", "1", "--title", "X", "--requirements", "r", "--acceptance", "a", "--depends", "42"}); err == nil {
		t.Fatal("unknown dependency must be rejected")
	}
	if err := Run([]string{"ticket", "outdated", "2"}); err == nil {
		t.Fatal("wait_review ticket must not be marked outdated")
	}
	tk, _ := db.Ticket(2)
	if tk.PRURL == "" || tk.ReviewState != store.RRequestChanges || tk.ReviewSummary != "add tests" {
		t.Fatalf("ticket not updated: %+v", tk)
	}
	t1, _ := db.Ticket(1)
	if t1.Requirements != "Build it" {
		t.Fatalf("requirements-file not read: %q", t1.Requirements)
	}
	r, _ := db.RunByID("developer-1")
	if !r.Callback || r.Summary != "PR opened" {
		t.Fatalf("run done not recorded: %+v", r)
	}
}
