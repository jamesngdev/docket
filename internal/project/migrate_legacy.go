package project

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"docket/internal/gitops"
	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/sh"
)

// The tool was called "po-tui" before it became "docket". Projects created back
// then keep their data in <repo>/.po-tui/po.sqlite; MigrateLegacy moves it to
// <repo>/.docket/docket.sqlite once, the first time any docket command opens it.
const (
	legacyDirName = ".po-tui"
	legacyDBName  = "po.sqlite"
)

// MigrateLegacy is a no-op unless <repo>/.po-tui exists and <repo>/.docket does not.
// It refuses while agents started by the old version are still running, because
// their worktrees live inside the folder being moved.
func MigrateLegacy(repo string) error {
	p := paths.New(repo)
	old := filepath.Join(p.Repo, legacyDirName)
	if _, err := os.Stat(old); err != nil {
		return nil
	}
	if _, err := os.Stat(p.Root()); err == nil {
		return nil
	}
	stopLegacyDaemon(p.Repo, old)
	if n := legacyRunningRuns(filepath.Join(old, legacyDBName)); n > 0 {
		return fmt.Errorf("%d agents from the old po-tui version are still running in %s; stop them (tmux kill-session) and retry", n, repo)
	}
	if err := os.Rename(old, p.Root()); err != nil {
		return fmt.Errorf("move %s to %s: %w", old, p.Root(), err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := filepath.Join(p.Root(), legacyDBName+suffix)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, p.DB()+suffix); err != nil {
				return err
			}
		}
	}
	// git still records the old worktree paths; repair them after the move.
	if entries, err := os.ReadDir(p.Worktrees()); err == nil {
		args := []string{"worktree", "repair"}
		for _, e := range entries {
			if e.IsDir() {
				args = append(args, filepath.Join(p.Worktrees(), e.Name()))
			}
		}
		if len(args) > 2 {
			_, _ = sh.Run(p.Repo, "git", args...)
		}
	}
	return gitops.ExcludeDataDir(p.Repo, paths.DirName+"/")
}

// stopLegacyDaemon asks a po-tui daemon (same protocol, old socket path) to exit.
func stopLegacyDaemon(repo, oldRoot string) {
	sock := filepath.Join(oldRoot, "daemon.sock")
	if len(sock) >= 100 {
		h := sha1.Sum([]byte(repo))
		sock = filepath.Join(os.TempDir(), "po-tui-"+hex.EncodeToString(h[:6])+".sock")
	}
	if ipc.Call(sock, ipc.Request{Cmd: ipc.CmdShutdown}) != nil {
		return
	}
	for i := 0; i < 50 && ipc.Call(sock, ipc.Request{Cmd: ipc.CmdPing}) == nil; i++ {
		time.Sleep(100 * time.Millisecond)
	}
}

func legacyRunningRuns(dbPath string) int {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return 0
	}
	defer db.Close()
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM runs WHERE status='running'`).Scan(&n)
	return n
}
