package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docket/internal/store"
)

func TestManifestSkillsExist(t *testing.T) {
	m, err := Manifest()
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{RoleTechLead, RoleDeveloper, RoleTester, RoleAdvisor} {
		if len(m[role]) == 0 {
			t.Fatalf("role %s has no skills", role)
		}
		for _, s := range m[role] {
			if _, err := skillDir(role, s); err != nil {
				t.Error(err)
			}
		}
	}
}

func TestBuildHomeIsolation(t *testing.T) {
	home := filepath.Join(t.TempDir(), "developer")
	// Simulate foreign content and an existing login that must survive.
	_ = os.MkdirAll(filepath.Join(home, "skills", "user-junk"), 0o755)
	_ = os.MkdirAll(filepath.Join(home, "commands"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"oauthAccount":{"x":1}}`), 0o600)

	if err := BuildHome(home, RoleDeveloper, "sonnet", "/work/wt", nil); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(home, "skills"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	got := strings.Join(names, ",")
	if strings.Contains(got, "user-junk") || strings.Contains(got, "generate-tickets") || !strings.Contains(got, "implement-ticket") {
		t.Fatalf("developer skills wrong: %s", got)
	}
	if _, err := os.Stat(filepath.Join(home, "commands")); !os.IsNotExist(err) {
		t.Fatal("foreign commands dir must be removed")
	}
	var cfg map[string]any
	b, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	_ = json.Unmarshal(b, &cfg)
	if cfg["oauthAccount"] == nil || cfg["hasCompletedOnboarding"] != true {
		t.Fatalf("login must be preserved and onboarding marked: %v", cfg)
	}
	proj := cfg["projects"].(map[string]any)["/work/wt"].(map[string]any)
	if proj["hasTrustDialogAccepted"] != true {
		t.Fatal("workdir must be trusted")
	}
	// Second build is a no-op for skills (hash unchanged) and still valid.
	if err := BuildHome(home, RoleDeveloper, "sonnet", "/work/wt2", nil); err != nil {
		t.Fatal(err)
	}
}

func TestTicketMarkdownIncludesReview(t *testing.T) {
	md := TicketMarkdown(TicketInput{
		Project: store.Project{Settings: store.DefaultSettings()},
		Ticket: store.Ticket{ID: 7, Title: "API", ReviewState: store.RRequestChanges, ReviewSummary: "fix nil check",
			ReviewRounds: 1, Acceptance: "- [ ] 201"},
		DevEnv: "DATABASE_URL=postgres://localhost:55432/app",
	})
	for _, want := range []string{"# Ticket #7: API", "PR target: develop", "fix nil check", "DATABASE_URL", "- [ ] 201"} {
		if !strings.Contains(md, want) {
			t.Errorf("TICKET.md missing %q", want)
		}
	}
}

func TestSessionName(t *testing.T) {
	if got := SessionName("demo", RoleDeveloper, 5, "x"); got != "docket-demo-developer-5" {
		t.Fatal(got)
	}
	if got := SessionName("demo", RoleTechLead, 0, "techlead-abc"); got != "docket-demo-techlead-techlead-abc" {
		t.Fatal(got)
	}
}
