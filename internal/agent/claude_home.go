// Package agent prepares and launches isolated Claude Code agents:
// a docket-owned CLAUDE_CONFIG_DIR per role, context files per run,
// and the tmux session hosting the `claude` process.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"

	"gopkg.in/yaml.v3"

	"docket/internal/extskills"
	"docket/skills"
)

// Roles known to docket.
const (
	RoleTechLead  = "techlead"
	RoleDeveloper = "developer"
	RoleTester    = "tester"
	RoleSupport   = "support" // read-only Backend Support answering Discord
)

// Manifest maps role → allowed skill names (skills/manifest.yaml).
func Manifest() (map[string][]string, error) {
	b, err := skills.FS.ReadFile("manifest.yaml")
	if err != nil {
		return nil, err
	}
	m := map[string][]string{}
	return m, yaml.Unmarshal(b, &m)
}

// skillDir finds a skill in the bundle: common/<name> first, then <role>/<name>.
func skillDir(role, name string) (string, error) {
	for _, group := range []string{"common", role} {
		p := path.Join(group, name)
		if _, err := fs.Stat(skills.FS, path.Join(p, "SKILL.md")); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("skill %q for role %q not found in bundle", name, role)
}

// BuildHome (re)creates the isolated config dir for a role. Skills are copied
// only from the embedded bundle; user-level folders (commands, plugins, hooks,
// agents) are wiped so nothing foreign survives. Login state in .claude.json
// and credentials are preserved.
//
// extra are external team skills (developer runs) copied next to the bundle.
func BuildHome(home, role, model, workdir string, extra []extskills.Skill) error {
	m, err := Manifest()
	if err != nil {
		return err
	}
	names, ok := m[role]
	if !ok {
		return fmt.Errorf("role %q missing in skills manifest", role)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	if err := syncSkills(home, role, names, extra); err != nil {
		return err
	}
	for _, d := range []string{"agents", "commands", "plugins", "output-styles", "hooks"} {
		_ = os.RemoveAll(filepath.Join(home, d))
	}
	if err := os.WriteFile(filepath.Join(home, "CLAUDE.md"), []byte(RoleInstructions(role)), 0o644); err != nil {
		return err
	}
	settings := map[string]any{
		"model":                             model,
		"permissions":                       permissionsFor(role),
		"skipDangerousModePermissionPrompt": true,
		"includeCoAuthoredBy":               false,
	}
	if err := writeJSON(filepath.Join(home, "settings.json"), settings); err != nil {
		return err
	}
	return markOnboarded(home, workdir)
}

// permissionsFor returns the role's permission settings. Workers run unattended
// with bypass; Backend Support runs headless in default mode, where anything
// outside its allowlist is denied (nobody can approve a prompt), and secrets
// stay unreadable even inside its working directory.
func permissionsFor(role string) map[string]any {
	if role != RoleSupport {
		return map[string]any{"defaultMode": "bypassPermissions"}
	}
	return map[string]any{
		"defaultMode": "default",
		"deny": []string{
			"Read(**/.env)", "Read(**/.env.*)", "Read(**/*.pem)", "Read(**/*.key)",
			"Read(**/.docket/claude-home/**)", "Read(**/.credentials.json)",
		},
	}
}

// skillFile is one file to place under <home>/skills/.
type skillFile struct {
	dst  string
	mode os.FileMode
	data []byte
}

// syncSkills writes the role's bundled skills plus external team skills,
// skipping work when the combined content hash is unchanged. Bundled skills
// win on a name clash so docket's own workflow can't be overridden.
func syncSkills(home, role string, names []string, extra []extskills.Skill) error {
	sort.Strings(names)
	h := sha256.New()
	var files []skillFile
	add := func(dst string, mode os.FileMode, b []byte) {
		files = append(files, skillFile{dst, mode, b})
		h.Write([]byte(dst))
		h.Write(b)
	}
	own := map[string]bool{}
	for _, n := range names {
		own[n] = true
		dir, err := skillDir(role, n)
		if err != nil {
			return err
		}
		err = fs.WalkDir(skills.FS, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, p)
			b, err := skills.FS.ReadFile(p)
			if err == nil {
				add(filepath.Join(n, rel), 0o644, b)
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	for _, sk := range extra {
		if own[sk.Name] {
			continue
		}
		err := filepath.WalkDir(sk.Dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p != sk.Dir && extskills.Skip(d.Name()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(sk.Dir, p)
			add(filepath.Join(sk.Name, rel), info.Mode().Perm()|0o644, b)
			return nil
		})
		if err != nil {
			return fmt.Errorf("external skill %s: %w", sk.Name, err)
		}
	}
	sum := hex.EncodeToString(h.Sum(nil))
	marker := filepath.Join(home, ".docket-skills-hash")
	skillsDir := filepath.Join(home, "skills")
	if b, err := os.ReadFile(marker); err == nil && string(b) == sum {
		if _, err := os.Stat(skillsDir); err == nil {
			return nil
		}
	}
	if err := os.RemoveAll(skillsDir); err != nil {
		return err
	}
	for _, f := range files {
		dst := filepath.Join(skillsDir, f.dst)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, f.data, f.mode); err != nil {
			return err
		}
	}
	return os.WriteFile(marker, []byte(sum), 0o644)
}

// markOnboarded merges onboarding/trust flags into <home>/.claude.json so the
// interactive session never blocks on first-run dialogs.
func markOnboarded(home, workdir string) error {
	p := filepath.Join(home, ".claude.json")
	cfg := map[string]any{}
	if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &cfg); err != nil {
			// A running claude may be mid-write; never overwrite (and lose) its login state.
			return fmt.Errorf("unreadable %s, retry later: %w", p, err)
		}
	}
	cfg["hasCompletedOnboarding"] = true
	cfg["bypassPermissionsModeAccepted"] = true
	projects, _ := cfg["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	entry, _ := projects[workdir].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	entry["hasTrustDialogAccepted"] = true
	entry["hasCompletedProjectOnboarding"] = true
	projects[workdir] = entry
	cfg["projects"] = projects
	return writeJSON(p, cfg)
}

func writeJSON(p string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	// Write to a temp file and rename so concurrent readers never see a partial file.
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// WriteMCPConfig writes the only MCP config agents load: codegraph, plus
// Pencil (design files) when the Pencil app is installed.
func WriteMCPConfig(p string) error {
	servers := map[string]any{
		"codegraph": map[string]any{"type": "stdio", "command": "codegraph", "args": []string{"serve", "--mcp"}},
	}
	if pencil := pencilServer(); pencil != nil {
		servers["pencil"] = pencil
	}
	return writeJSON(p, map[string]any{"mcpServers": servers})
}

// pencilServer returns the Pencil MCP server definition: the user's own
// (~/.claude.json mcpServers.pencil) if configured, else the binary bundled
// with Pencil.app. nil when Pencil is not installed.
func pencilServer() map[string]any {
	home, _ := os.UserHomeDir()
	if b, err := os.ReadFile(filepath.Join(home, ".claude.json")); err == nil {
		var cfg struct {
			MCPServers map[string]map[string]any `json:"mcpServers"`
		}
		if json.Unmarshal(b, &cfg) == nil && cfg.MCPServers["pencil"] != nil {
			return cfg.MCPServers["pencil"]
		}
	}
	arch := map[string]string{"arm64": "arm64", "amd64": "x64"}[runtime.GOARCH]
	bin := "/Applications/Pencil.app/Contents/Resources/app.asar.unpacked/out/mcp-server-darwin-" + arch
	if _, err := os.Stat(bin); runtime.GOOS != "darwin" || err != nil {
		return nil
	}
	return map[string]any{"type": "stdio", "command": bin, "args": []string{"--app", "desktop"}}
}
