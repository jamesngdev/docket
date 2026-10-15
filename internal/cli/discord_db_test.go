package cli

import (
	"os"
	"testing"

	"docket/internal/paths"
	"docket/internal/store"
)

// TestDiscordDB sets, rejects and clears the dev database of Backend Support.
// DEVDB_LIVE_PG (a reachable postgres URL) also exercises the connect check.
func TestDiscordDB(t *testing.T) {
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
	setting := func() string { proj, _ := db.Project(); return proj.Settings.SupportDB }

	if Run([]string{"discord", "db", "mysql://u:p@h/db"}) == nil || setting() != "" {
		t.Fatal("unsupported scheme must be rejected")
	}
	if Run([]string{"discord", "db", "postgres://u:p@127.0.0.1:1/db?connect_timeout=2"}) == nil || setting() != "" {
		t.Fatal("unreachable database must not be saved")
	}
	if live := os.Getenv("DEVDB_LIVE_PG"); live != "" {
		if err := Run([]string{"discord", "db", live}); err != nil || setting() != live {
			t.Fatalf("live set: %v", err)
		}
	}
	if err := Run([]string{"discord", "db", "off"}); err != nil || setting() != "" {
		t.Fatalf("off: %v %q", err, setting())
	}
}
