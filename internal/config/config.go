// Package config manages the global docket config file
// (~/.config/docket/config.yaml): registered projects and the agent OAuth token.
package config

import (
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"

	"docket/internal/paths"
)

type Config struct {
	OAuthToken string   `yaml:"oauth_token,omitempty"`
	Projects   []string `yaml:"projects"`
	// TeamSkillRepos are git repos of team skills pulled before every Developer
	// and TechLead run. Unset = extskills.DefaultRepos; [] = none.
	TeamSkillRepos *[]string `yaml:"team_skill_repos,omitempty"`
	// LegacySkillRepos is the old key name, still honoured when set.
	LegacySkillRepos *[]string `yaml:"developer_skill_repos,omitempty"`
}

// SkillCacheDir holds shallow clones of the skill repos.
func SkillCacheDir() string { return filepath.Join(paths.GlobalConfigDir(), "skill-sources") }

func file() string { return filepath.Join(paths.GlobalConfigDir(), "config.yaml") }

// Load returns the config; a missing file yields an empty config.
func Load() (Config, error) {
	migrateLegacyDir()
	var c Config
	b, err := os.ReadFile(file())
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, yaml.Unmarshal(b, &c)
}

// migrateLegacyDir moves ~/.config/po-tui (the tool's former name) to
// ~/.config/docket once, keeping registered projects, token and skill caches.
func migrateLegacyDir() {
	if os.Getenv("DOCKET_CONFIG_DIR") != "" {
		return
	}
	if _, err := os.Stat(paths.GlobalConfigDir()); err == nil {
		return
	}
	if _, err := os.Stat(paths.LegacyGlobalConfigDir()); err == nil {
		_ = os.MkdirAll(filepath.Dir(paths.GlobalConfigDir()), 0o700)
		_ = os.Rename(paths.LegacyGlobalConfigDir(), paths.GlobalConfigDir())
	}
}

// Save writes the config with 0600 permissions since it may hold a token.
func Save(c Config) error {
	if err := os.MkdirAll(paths.GlobalConfigDir(), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(file(), b, 0o600)
}

// AddProject registers a repo path (idempotent).
func AddProject(repo string) error {
	c, err := Load()
	if err != nil {
		return err
	}
	if !slices.Contains(c.Projects, repo) {
		c.Projects = append(c.Projects, repo)
	}
	return Save(c)
}

// RemoveProject unregisters a repo path; files on disk are untouched.
func RemoveProject(repo string) error {
	c, err := Load()
	if err != nil {
		return err
	}
	c.Projects = slices.DeleteFunc(c.Projects, func(p string) bool { return p == repo })
	return Save(c)
}

func SetToken(tok string) error {
	c, err := Load()
	if err != nil {
		return err
	}
	c.OAuthToken = tok
	return Save(c)
}
