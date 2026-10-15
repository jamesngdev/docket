package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	_ = db.InsertRun(store.Run{ID: "developer-1", Role: "developer", Session: "s", Workdir: pushedTicketBranch(t, "ticket/2-api")})
	// The gate reads the PR body through gh: an unauthenticated gh fails fast and the check is skipped.
	t.Setenv("GH_CONFIG_DIR", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

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
	if err := Run([]string{"ticket", "add", "--feature", "1", "--title", "Schema", "--requirements", "r", "--acceptance", "- [ ] ok", "--files", "db/"}); err == nil ||
		!strings.Contains(err.Error(), "verify:") {
		t.Fatalf("acceptance items without verify: must be rejected, got %v", err)
	}
	if err := Run([]string{"ticket", "add", "--feature", "1", "--title", "Schema", "--requirements", "r", "--acceptance", "- [ ] ok — verify: test T"}); err == nil {
		t.Fatal("ticket add without --files must be rejected")
	}
	mustRun("ticket", "add", "--feature", "1", "--title", "Schema", "--requirements-file", req, "--acceptance", "- [ ] ok — verify: test T",
		"--files", "db/, migrations/*.sql", "--model", "haiku")
	mustRun("ticket", "add", "--feature", "1", "--title", "API", "--requirements", "r", "--acceptance", "- [ ] a — verify: curl /x",
		"--files", "api/", "--depends", "1")
	if t1, _ := db.Ticket(1); t1.PlannedFiles != "db/,migrations/*.sql" || t1.Model != "haiku" {
		t.Fatalf("files/model not stored: %+v", t1)
	}
	if err := Run([]string{"ticket", "update", "1", "--model", "gpt-4"}); err == nil {
		t.Fatal("unknown model must be rejected")
	}
	if err := Run([]string{"ticket", "status", "2", "wait_review", "--pr", "u"}); err == nil {
		t.Fatal("a run must not change a ticket it does not own")
	}
	_ = db.UpdateTicketFields(2, map[string]any{"status": store.TProcessing, "locked_by": "developer-1"})
	if err := Run([]string{"ticket", "status", "2", "wait_review"}); err == nil {
		t.Fatal("wait_review without a PR must be rejected")
	}
	_ = db.UpdateTicketFields(2, map[string]any{"branch": "ticket/2-api"})
	mustRun("ticket", "status", "2", "wait_review", "--pr", "https://github.com/o/r/pull/9")
	if t2, _ := db.Ticket(2); t2.GateSHA == "" || !strings.Contains(t2.RiskFlags, "no verify checks") {
		t.Fatalf("gate result not recorded: sha=%q risks=%q", t2.GateSHA, t2.RiskFlags)
	}
	mustRun("note", "add", "--text", "tests need  DB_URL")
	if b, _ := os.ReadFile(p.RepoNotes()); !strings.Contains(string(b), "- tests need DB_URL (developer,") {
		t.Fatalf("note not appended: %q", b)
	}
	mustRun("ticket", "update", "1", "--title", "Schema v2", "--depends", "")
	if err := Run([]string{"ticket", "update", "1", "--depends", "1"}); err == nil {
		t.Fatal("self dependency must be rejected")
	}
	if t1, _ := db.Ticket(1); t1.Title != "Schema v2" {
		t.Fatalf("update not applied: %q", t1.Title)
	}
	mustRun("review", "submit", "2", "--result", "request_changes", "--summary", "add tests")
	mustRun("run", "done", "--summary", "PR opened")
	if err := Run([]string{"ticket", "add", "--feature", "1", "--title", "X", "--requirements", "r", "--acceptance", "- [ ] a — verify: x",
		"--files", "x/", "--depends", "42"}); err == nil {
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

// pushedTicketBranch returns a clone whose branch (one committed file under
// api/) is pushed to a local bare origin that also has develop.
func pushedTicketBranch(t *testing.T, branch string) string {
	t.Helper()
	remote, work := t.TempDir(), t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=a@b", "-c", "user.name=t"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git(remote, "init", "-q", "--bare", "-b", "develop")
	git(work, "init", "-q", "-b", "develop")
	git(work, "remote", "add", "origin", remote)
	_ = os.WriteFile(filepath.Join(work, "README.md"), []byte("x"), 0o644)
	git(work, "add", ".")
	git(work, "commit", "-q", "-m", "init")
	git(work, "push", "-q", "-u", "origin", "develop")
	git(work, "checkout", "-q", "-b", branch)
	_ = os.MkdirAll(filepath.Join(work, "api"), 0o755)
	_ = os.WriteFile(filepath.Join(work, "api", "x.go"), []byte("package api\n"), 0o644)
	git(work, "add", ".")
	git(work, "commit", "-q", "-m", "feat: api")
	git(work, "push", "-q", "-u", "origin", branch)
	return work
}
