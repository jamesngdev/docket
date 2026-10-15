// Package extskills pulls team skill repositories (e.g. github.com/jamesngdev/be-skills)
// before every Developer run so those agents always get the latest version.
// A skill is any directory containing SKILL.md, under <repo>/skills/ or the repo root.
package extskills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"docket/internal/config"
	"docket/internal/sh"
)

// DefaultRepos is used when the global config does not list any.
var DefaultRepos = []string{"https://github.com/jamesngdev/be-skills"}

// RolesWithTeamSkills are the agent roles that receive team skills.
var RolesWithTeamSkills = map[string]bool{"developer": true, "techlead": true}

// Load pulls the configured team skill repos (at most every pullEvery per repo).
func Load() ([]Skill, []error) {
	repos := DefaultRepos
	if c, err := config.Load(); err == nil {
		switch {
		case c.TeamSkillRepos != nil:
			repos = *c.TeamSkillRepos
		case c.LegacySkillRepos != nil:
			repos = *c.LegacySkillRepos
		}
	}
	return Sync(repos, config.SkillCacheDir())
}

// Skill is one external skill on disk.
type Skill struct {
	Name, Description, Dir, Source string
}

var nonSlug = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// cacheDir maps a repo URL to its local clone under root.
func cacheDir(root, url string) string {
	name := strings.Trim(nonSlug.ReplaceAllString(strings.TrimSuffix(url, ".git"), "-"), "-")
	return filepath.Join(root, name)
}

// Sync clones or fast-forwards every repo (shallow), then returns all skills.
// A failed pull falls back to the cached copy; errors are returned for logging.
func Sync(urls []string, cacheRoot string) ([]Skill, []error) {
	var skills []Skill
	var errs []error
	seen := map[string]bool{}
	for _, url := range urls {
		dir := cacheDir(cacheRoot, url)
		if err := pull(url, dir); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", url, err))
		}
		found, err := Scan(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", url, err))
			continue
		}
		for _, s := range found {
			if seen[s.Name] {
				errs = append(errs, fmt.Errorf("skill %q defined twice; keeping the first", s.Name))
				continue
			}
			seen[s.Name] = true
			s.Source = url
			skills = append(skills, s)
		}
	}
	return skills, errs
}

// pullEvery bounds how often a team skill repo is pulled: agents start in
// bursts (parallel Developers, review rounds), and a few minutes of staleness
// is fine for skills while a pull per spawn costs seconds each time.
const pullEvery = 10 * time.Minute

func pull(url, dir string) error {
	stamp := filepath.Join(dir, ".git", "docket-pulled-at")
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		if fi, err := os.Stat(stamp); err == nil && time.Since(fi.ModTime()) < pullEvery {
			return nil
		}
		if _, err := sh.RunTimeout(time.Minute, dir, "git", "pull", "--ff-only", "-q"); err != nil {
			return err
		}
		return os.WriteFile(stamp, nil, 0o644)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if _, err := sh.RunTimeout(2*time.Minute, "", "git", "clone", "-q", "--depth", "1", url, dir); err != nil {
		return err
	}
	return os.WriteFile(stamp, nil, 0o644)
}

// Scan finds skill directories (containing SKILL.md) in <dir>/skills/* and <dir>/*.
func Scan(dir string) ([]Skill, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	var out []Skill
	for _, base := range []string{filepath.Join(dir, "skills"), dir} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || Skip(e.Name()) {
				continue
			}
			sd := filepath.Join(base, e.Name())
			md := filepath.Join(sd, "SKILL.md")
			if _, err := os.Stat(md); err != nil {
				continue
			}
			name, desc := frontmatter(md)
			if name == "" {
				name = e.Name()
			}
			out = append(out, Skill{Name: name, Description: desc, Dir: sd})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Skip reports names never copied into an agent home (VCS, caches, bytecode).
func Skip(name string) bool {
	return strings.HasPrefix(name, ".") || name == "__pycache__" || name == "node_modules"
}

// frontmatter reads name/description from the leading --- block of SKILL.md.
func frontmatter(path string) (name, desc string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	inBlock := false
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			if inBlock {
				break
			}
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			switch strings.TrimSpace(k) {
			case "name":
				name = v
			case "description":
				desc = v
			}
		}
	}
	return name, desc
}

// PromptSection tells the agent which team skills it has and how to use them.
// Developers apply them to their change; TechLead writes tickets that respect
// them and says which ones each ticket's Developer must apply.
func PromptSection(skills []Skill, role string) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	if role == "techlead" {
		b.WriteString("\n\nTEAM SKILLS (kept up to date by docket) encode the team's engineering standards. Read the SKILL.md of\n")
		b.WriteString("each one that relates to your task. When you create or update features/tickets:\n")
		b.WriteString("- Fold their rules into requirements and acceptance criteria (e.g. security checks, docs/llms.txt updates).\n")
		b.WriteString("- End every ticket's requirements with a line \"Team skills: <names>\" naming the skills its Developer must\n")
		b.WriteString("  apply (or \"Team skills: none\").\n")
		b.WriteString("- Do not run long audits unless your task needs their findings to write correct tickets.\n")
	} else {
		b.WriteString("\n\nTEAM SKILLS (kept up to date by docket). Before opening/updating the PR, apply EVERY skill below whose\n")
		b.WriteString("description matches your change (always those named in the ticket's \"Team skills\" line), following its\n")
		b.WriteString("SKILL.md. In the PR body add a \"Team skills\" section listing each skill as applied (with the outcome)\n")
		b.WriteString("or skipped (with a one-line reason).\n")
	}
	for _, s := range skills {
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
	}
	return b.String()
}
