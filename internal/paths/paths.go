// Package paths resolves every on-disk location docket uses for a project
// (the `.docket/` folder inside the user's repo checkout) and globally.
package paths

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
)

// DirName is the per-project data folder created inside the repo checkout.
const DirName = ".docket"

// Project groups all derived paths for one project checkout.
type Project struct {
	Repo string // absolute path of the user's repo checkout
}

func New(repo string) Project {
	abs, err := filepath.Abs(repo)
	if err != nil {
		abs = repo
	}
	return Project{Repo: abs}
}

func (p Project) Root() string        { return filepath.Join(p.Repo, DirName) }
func (p Project) DB() string          { return filepath.Join(p.Root(), "docket.sqlite") }
func (p Project) Docs() string        { return filepath.Join(p.Root(), "docs") }
func (p Project) Worktrees() string   { return filepath.Join(p.Root(), "worktrees") }
func (p Project) Logs() string        { return filepath.Join(p.Root(), "logs") }
func (p Project) Context() string     { return filepath.Join(p.Root(), "context") }
func (p Project) ClaudeHomes() string { return filepath.Join(p.Root(), "claude-home") }
func (p Project) MCPConfig() string   { return filepath.Join(p.Root(), "mcp.json") }
func (p Project) ComposeFile() string { return filepath.Join(p.Root(), "docker-compose.yml") }
func (p Project) DevDBEnv() string    { return filepath.Join(p.Root(), "dev-db.env") }
func (p Project) RepoNotes() string   { return filepath.Join(p.Root(), "REPO.md") }     // shared repo knowledge for every agent
func (p Project) Checks() string      { return filepath.Join(p.Root(), "checks.yaml") } // commands the wait_review gate runs
func (p Project) PIDFile() string     { return filepath.Join(p.Root(), "daemon.pid") }
func (p Project) DaemonLog() string   { return filepath.Join(p.Logs(), "daemon.log") }
func (p Project) ClaudeHome(role string) string {
	return filepath.Join(p.ClaudeHomes(), role)
}
func (p Project) Worktree(name string) string { return filepath.Join(p.Worktrees(), name) }
func (p Project) RunContext(runID string) string {
	return filepath.Join(p.Context(), runID)
}
func (p Project) RunLog(runID string) string { return filepath.Join(p.Logs(), runID+".log") }

// Socket returns the daemon's unix socket path. macOS limits socket paths to
// ~104 bytes, so long repo paths fall back to a hashed path in the temp dir.
func (p Project) Socket() string {
	s := filepath.Join(p.Root(), "daemon.sock")
	if len(s) < 100 {
		return s
	}
	h := sha1.Sum([]byte(p.Repo))
	return filepath.Join(os.TempDir(), "docket-"+hex.EncodeToString(h[:6])+".sock")
}

// EnsureLayout creates every folder under `.docket/`.
func (p Project) EnsureLayout() error {
	for _, d := range []string{p.Root(), p.Worktrees(), p.Logs(), p.Context(), p.ClaudeHomes()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// LegacyGlobalConfigDir is where po-tui (docket's former name) kept its config.
func LegacyGlobalConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "po-tui")
}

// GlobalConfigDir is ~/.config/docket (never inside a repo).
func GlobalConfigDir() string {
	if d := os.Getenv("DOCKET_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "docket")
}
