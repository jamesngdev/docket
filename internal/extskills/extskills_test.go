package extskills

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSyncPullsLatest uses a local git repo as the "remote" skill repository.
func TestSyncPullsLatest(t *testing.T) {
	remote := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=a@b", "-c", "user.name=t"}, args...)...)
		cmd.Dir = remote
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "master")
	write(t, filepath.Join(remote, "skills/security/SKILL.md"), "---\nname: security\ndescription: Audit code. Use when asked.\n---\n# x\n")
	write(t, filepath.Join(remote, "skills/security/scripts/__pycache__/a.pyc"), "junk")
	write(t, filepath.Join(remote, "skills/notaskill/readme.txt"), "no SKILL.md")
	git("add", ".")
	git("commit", "-q", "-m", "one")

	cache := t.TempDir()
	skills, errs := Sync([]string{remote}, cache)
	if len(errs) > 0 || len(skills) != 1 || skills[0].Name != "security" || !strings.HasPrefix(skills[0].Description, "Audit code") {
		t.Fatalf("first sync: %+v %v", skills, errs)
	}

	write(t, filepath.Join(remote, "skills/llms-txt-sync/SKILL.md"), "---\nname: llms-txt-sync\ndescription: Sync docs.\n---\n")
	git("add", ".")
	git("commit", "-q", "-m", "two")
	skills, errs = Sync([]string{remote}, cache)
	if len(errs) > 0 || len(skills) != 2 {
		t.Fatalf("second sync must pull the new skill: %+v %v", skills, errs)
	}
	if !strings.Contains(PromptSection(skills, "developer"), "- llms-txt-sync: Sync docs.") {
		t.Fatal("prompt must list pulled skills")
	}
	if tl := PromptSection(skills, "techlead"); !strings.Contains(tl, "Team skills: <names>") || !strings.Contains(tl, "- security:") {
		t.Fatal("TechLead prompt must ask tickets to name the team skills to apply")
	}
}

func TestSyncFallsBackToCache(t *testing.T) {
	cache := t.TempDir()
	dir := cacheDir(cache, "https://example.invalid/team/skills")
	write(t, filepath.Join(dir, "review/SKILL.md"), "---\nname: review\ndescription: d\n---\n")
	skills, errs := Sync([]string{"https://example.invalid/team/skills"}, cache)
	if len(errs) == 0 || len(skills) != 1 {
		t.Fatalf("unreachable repo must log an error but still use the cached skills: %+v %v", skills, errs)
	}
}
