package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"docket/internal/paths"
	"docket/internal/store"
)

func TestMigrateLegacy(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	old := filepath.Join(repo, legacyDirName)
	_ = os.MkdirAll(old, 0o755)
	db, err := store.Open(filepath.Join(old, legacyDBName))
	if err != nil {
		t.Fatal(err)
	}
	_ = db.CreateProject(store.Project{Name: "Legacy", RepoPath: repo, ActivePhase: "main"})
	_ = db.InsertRun(store.Run{ID: "r1", Role: "developer", Session: "s"})
	db.Close()

	if err := MigrateLegacy(repo); err == nil {
		t.Fatal("must refuse while an old agent run is still marked running")
	}
	db, _ = store.Open(filepath.Join(old, legacyDBName))
	_ = db.FinishRun("r1", store.RunDone)
	db.Close()

	if err := MigrateLegacy(repo); err != nil {
		t.Fatal(err)
	}
	p := paths.New(repo)
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("legacy folder must be moved")
	}
	db, err = store.Open(p.DB())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if proj, err := db.Project(); err != nil || proj.Name != "Legacy" {
		t.Fatalf("data must survive the move: %+v %v", proj, err)
	}
	if err := MigrateLegacy(repo); err != nil {
		t.Fatal("second call must be a no-op")
	}
}
