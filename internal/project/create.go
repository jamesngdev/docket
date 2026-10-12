// Package project creates a new docket project: clones repos, prepares the
// `.docket/` folder, creates the database and registers it globally.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"docket/internal/config"
	"docket/internal/gitops"
	"docket/internal/paths"
	"docket/internal/store"
)

// Input is what the "New project" form collects.
type Input struct {
	Name, Description, RepoURL, LocalPath, DocsRepoURL string
	Phase                                              string // docs branch to start with; "" = docs default branch
	Settings                                           store.Settings
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// Create runs every setup step and returns the absolute repo path.
// codegraph indexing and the TechLead bootstrap are left to the daemon.
func Create(in Input) (string, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.LocalPath) == "" {
		return "", fmt.Errorf("name and local path are required")
	}
	repo, err := filepath.Abs(expandHome(strings.TrimSpace(in.LocalPath)))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(repo); os.IsNotExist(err) {
		if in.RepoURL == "" {
			return "", fmt.Errorf("%s does not exist and no GitHub repo URL given", repo)
		}
		if err := gitops.Clone(in.RepoURL, repo); err != nil {
			return "", err
		}
	}
	if !gitops.IsRepo(repo) {
		return "", fmt.Errorf("%s is not a git repository", repo)
	}
	p := paths.New(repo)
	if _, err := os.Stat(p.DB()); err == nil {
		return "", fmt.Errorf("a docket project already exists in %s", repo)
	}
	if err := p.EnsureLayout(); err != nil {
		return "", err
	}
	if err := gitops.ExcludeDataDir(repo, paths.DirName+"/"); err != nil {
		return "", fmt.Errorf("exclude %s from git: %w", paths.DirName, err)
	}
	_ = gitops.Fetch(repo)
	_ = gitops.CheckoutBase(repo, in.Settings.BaseBranch) // best effort; daemon retries before TechLead runs
	phase := strings.TrimSpace(in.Phase)
	if in.DocsRepoURL != "" {
		if err := gitops.Clone(in.DocsRepoURL, p.Docs()); err != nil {
			return "", fmt.Errorf("clone docs: %w", err)
		}
		if phase == "" {
			phase, _ = gitops.CurrentBranch(p.Docs())
		} else if err := gitops.CheckoutBase(p.Docs(), phase); err != nil {
			return "", fmt.Errorf("docs phase %q: %w", phase, err)
		}
	}
	if phase == "" {
		phase = "default"
	}
	db, err := store.Open(p.DB())
	if err != nil {
		return "", err
	}
	defer db.Close()
	err = db.CreateProject(store.Project{
		Name: in.Name, Description: in.Description, RepoURL: in.RepoURL, RepoPath: repo,
		DocsRepoURL: in.DocsRepoURL, Settings: in.Settings, ActivePhase: phase,
	})
	if err != nil {
		return "", err
	}
	db.Log("project", "created %s", in.Name)
	return repo, config.AddProject(repo)
}
