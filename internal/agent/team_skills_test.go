package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docket/internal/extskills"
)

func TestBuildHomeWithTeamSkills(t *testing.T) {
	src := t.TempDir()
	_ = os.MkdirAll(filepath.Join(src, "security", "scripts", "__pycache__"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "security", "SKILL.md"), []byte("---\nname: security\n---\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "security", "scripts", "scan.py"), []byte("print(1)"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "security", "scripts", "__pycache__", "x.pyc"), []byte("junk"), 0o644)
	_ = os.MkdirAll(filepath.Join(src, "open-pr"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "open-pr", "SKILL.md"), []byte("EVIL OVERRIDE"), 0o644)
	extra := []extskills.Skill{{Name: "security", Dir: filepath.Join(src, "security")}, {Name: "open-pr", Dir: filepath.Join(src, "open-pr")}}

	home := filepath.Join(t.TempDir(), "developer")
	if err := BuildHome(home, RoleDeveloper, "sonnet", "/w", extra); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(home, "skills", "security", "scripts", "scan.py")); err != nil || st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("team skill script must be copied and stay executable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "skills", "security", "scripts", "__pycache__")); !os.IsNotExist(err) {
		t.Fatal("__pycache__ must not be copied")
	}
	b, _ := os.ReadFile(filepath.Join(home, "skills", "open-pr", "SKILL.md"))
	if strings.Contains(string(b), "EVIL") {
		t.Fatal("bundled docket skills must win over team skills with the same name")
	}
}
