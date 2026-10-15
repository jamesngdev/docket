package agent

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"docket/internal/store"
)

func TestSessionIDAndExists(t *testing.T) {
	id := NewSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(id) {
		t.Fatalf("not a UUID v4: %s", id)
	}
	home := t.TempDir()
	if SessionExists(home, id) || SessionExists(home, "") {
		t.Fatal("no transcript yet")
	}
	dir := filepath.Join(home, "projects", "-work-wt-3")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte("{}\n"), 0o644)
	if !SessionExists(home, id) {
		t.Fatal("transcript must be found")
	}
}

func TestResumeKickoff(t *testing.T) {
	dev := ResumeKickoff(RoleDeveloper, TaskImplement, "/ctx", 7)
	if !strings.Contains(dev, "Review feedback") || !strings.Contains(dev, "docket ticket status 7 wait_review") {
		t.Errorf("developer resume prompt:\n%s", dev)
	}
	if c := ResumeKickoff(RoleDeveloper, TaskResolveConflicts, "/ctx", 7); !strings.Contains(c, "resolve-conflicts") {
		t.Errorf("conflict resume prompt:\n%s", c)
	}
	if r := ResumeKickoff(RoleTester, "review", "/ctx", 7); !strings.Contains(r, "docket review submit 7") {
		t.Errorf("tester resume prompt:\n%s", r)
	}
}

func TestContextIncludesRepoNotesAndScope(t *testing.T) {
	md := TicketMarkdown(TicketInput{Project: store.Project{Settings: store.DefaultSettings()},
		Ticket:    store.Ticket{ID: 4, Title: "x", PlannedFiles: "api/,db/*.sql", GateSHA: "0123456789", RiskFlags: "large diff"},
		RepoNotes: "# Repo notes\n- test: go test ./...\n"})
	for _, want := range []string{"## Planned files", "api/,db/*.sql", "Gate: checks passed on 01234567 · risk flags: large diff", "## Repo notes", "go test ./..."} {
		if !strings.Contains(md, want) {
			t.Errorf("TICKET.md misses %q:\n%s", want, md)
		}
	}
}

func TestProjectMarkdownCompactsFinishedFeatures(t *testing.T) {
	md := ProjectMarkdown(TechleadInput{Project: store.Project{Settings: store.DefaultSettings()},
		Features: []store.Feature{
			{ID: 1, Title: "Done feature", Status: store.FApproved, Description: "LONG-DONE"},
			{ID: 2, Title: "Live feature", Status: store.FApproved, Description: "LONG-LIVE"},
			{ID: 3, Title: "Old feature", Status: store.FOutdated, Description: "LONG-OLD"},
		},
		Tickets: []store.Ticket{{ID: 1, FeatureID: 1, Status: store.TDone}, {ID: 2, FeatureID: 2, Status: store.TPending}},
	})
	if strings.Contains(md, "LONG-DONE") || strings.Contains(md, "LONG-OLD") || !strings.Contains(md, "LONG-LIVE") {
		t.Errorf("only unfinished features keep descriptions:\n%s", md)
	}
	if !strings.Contains(md, "- F1 [done] Done feature") || !strings.Contains(md, "- F3 [outdated] Old feature") {
		t.Errorf("finished features must stay listed by title:\n%s", md)
	}
}
