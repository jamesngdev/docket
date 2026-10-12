package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"docket/internal/appsvc"
	"docket/internal/config"
	"docket/internal/daemon"
	"docket/internal/paths"
	"docket/internal/project"
	"docket/internal/sh"
	"docket/internal/store"
)

// App is bound to the frontend: every exported method becomes a JS function.
type App struct {
	ctx context.Context
	mu  sync.Mutex
	dbs map[string]*store.DB // open project databases by repo path
}

func NewApp() *App { return &App{dbs: map[string]*store.DB{}} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, db := range a.dbs {
		_ = db.Close()
	}
}

// db returns the (cached) database of a project, migrating a legacy po-tui layout first.
func (a *App) db(repo string) (*store.DB, paths.Project, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p := paths.New(repo)
	if db, ok := a.dbs[p.Repo]; ok {
		return db, p, nil
	}
	if err := project.MigrateLegacy(p.Repo); err != nil {
		return nil, p, err
	}
	if _, err := os.Stat(p.DB()); err != nil {
		return nil, p, fmt.Errorf("%s is not a docket project", p.Repo)
	}
	db, err := store.Open(p.DB())
	if err != nil {
		return nil, p, err
	}
	a.dbs[p.Repo] = db
	return db, p, nil
}

// ProjectRow is one entry of the project list.
type ProjectRow struct {
	Repo, Name, Phase string
	Running, Done     int
	Total, OpenQ      int
	NeedsUser         int
	Error             string
}

// Projects lists registered projects with a short status.
func (a *App) Projects() ([]ProjectRow, error) {
	c, err := config.Load()
	if err != nil {
		return nil, err
	}
	rows := []ProjectRow{}
	for _, repo := range c.Projects {
		row := ProjectRow{Repo: repo, Name: repo}
		db, p, err := a.db(repo)
		if err != nil {
			row.Error = err.Error()
			rows = append(rows, row)
			continue
		}
		s, err := appsvc.Load(db, p, false)
		if err != nil {
			row.Error = err.Error()
		}
		row.Name, row.Phase = s.Project.Name, s.Project.ActivePhase
		row.Done, row.Total, row.OpenQ, row.NeedsUser = s.Counts.Done, s.Counts.Total, s.Counts.OpenQ, s.Counts.NeedsUser
		for _, r := range s.Runs {
			if r.Status == store.RunRunning {
				row.Running++
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// OpenProject makes sure the project's daemon runs (starting/upgrading it).
func (a *App) OpenProject(repo string) error {
	_, p, err := a.db(repo)
	if err != nil {
		return err
	}
	return daemon.EnsureRunning(p)
}

// Snapshot returns everything the project screens render.
func (a *App) Snapshot(repo string, withPhases bool) (appsvc.Snapshot, error) {
	db, p, err := a.db(repo)
	if err != nil {
		return appsvc.Snapshot{}, err
	}
	return appsvc.Load(db, p, withPhases)
}

// CreateProject runs the same setup as the TUI form and opens the project.
func (a *App) CreateProject(in project.Input) (string, error) {
	if in.Settings.Models == nil {
		def := store.DefaultSettings()
		in.Settings.Models = def.Models
	}
	repo, err := project.Create(in)
	if err != nil {
		return "", err
	}
	return repo, a.OpenProject(repo)
}

// RemoveProject unregisters a project (files are kept).
func (a *App) RemoveProject(repo string) error { return config.RemoveProject(repo) }

// Health reports which external tools are installed.
func (a *App) Health() map[string]bool {
	out := map[string]bool{}
	for _, bin := range []string{"claude", "gh", "git", "tmux", "docker", "codegraph"} {
		out[bin] = sh.Has(bin)
	}
	return out
}

// OpenInTerminal attaches Terminal.app to an agent's tmux session (phase 1;
// phase 2 replaces this with embedded xterm.js panes).
func (a *App) OpenInTerminal(session string) error {
	if session == "" {
		return fmt.Errorf("no session")
	}
	script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "tmux attach-session -t '=%s'"
end tell`, strings.ReplaceAll(session, `"`, ``))
	return exec.Command("osascript", "-e", script).Run()
}

// OpenURL opens a link (PR, docs) in the default browser.
func (a *App) OpenURL(url string) error { return exec.Command("open", url).Run() }

// OpenPath reveals a file in its default app (e.g. a docs Markdown file or an agent log).
func (a *App) OpenPath(path string) error { return exec.Command("open", path).Run() }
