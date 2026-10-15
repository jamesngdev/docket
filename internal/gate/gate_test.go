package gate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoWithPushedBranch makes a clone of a bare "origin" with branch develop
// and a pushed ticket branch adding the given files. Returns the clone dir.
func repoWithPushedBranch(t *testing.T, files map[string]string) (string, func(...string) string) {
	t.Helper()
	remote, work := t.TempDir(), t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=a@b", "-c", "user.name=t"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git := func(args ...string) string { return run(work, args...) }
	run(remote, "init", "-q", "--bare", "-b", "develop")
	git("init", "-q", "-b", "develop")
	git("remote", "add", "origin", remote)
	write(t, work, "README.md", "x\n")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	git("push", "-q", "-u", "origin", "develop")
	git("checkout", "-q", "-b", "ticket/1-x")
	for p, c := range files {
		write(t, work, p, c)
	}
	git("add", ".")
	git("commit", "-q", "-m", "feat: x")
	git("push", "-q", "-u", "origin", "ticket/1-x")
	return work, git
}

func write(t *testing.T, dir, p, c string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
	if err := os.WriteFile(filepath.Join(dir, p), []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
}

func input(wt string) Input {
	return Input{Worktree: wt, Branch: "ticket/1-x", Target: "develop", Planned: "internal/orders/",
		PRBody: "## Summary\nx\n## Acceptance criteria\n- [x] a\n## Tests\ngo test ok\n",
		Config: Config{Checks: []Check{{Name: "ok", Run: "test -n \"$DB_URL\""}}, MaxDiffLines: 600},
		Env:    []string{"DB_URL=postgres://localhost:55432/x"}}
}

func TestGatePassesLowRisk(t *testing.T) {
	wt, git := repoWithPushedBranch(t, map[string]string{"internal/orders/orders.go": "package orders\n"})
	res, err := Run(input(wt))
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA != git("rev-parse", "HEAD") || len(res.Risks) != 0 {
		t.Fatalf("want low-risk pass on HEAD, got %+v", res)
	}
}

func TestGateBlocks(t *testing.T) {
	cases := map[string]struct {
		files  map[string]string
		mutate func(*Input, func(...string) string, string)
		want   string
	}{
		"not pushed": {map[string]string{"internal/orders/a.go": "x"}, func(in *Input, git func(...string) string, wt string) {
			write(t, wt, "internal/orders/b.go", "y")
			git("add", ".")
			git("commit", "-q", "-m", "more")
		}, "not pushed"},
		"uncommitted": {map[string]string{"internal/orders/a.go": "x"}, func(in *Input, _ func(...string) string, wt string) {
			write(t, wt, "internal/orders/a.go", "changed")
		}, "not clean"},
		"untracked": {map[string]string{"internal/orders/a.go": "x"}, func(in *Input, _ func(...string) string, wt string) {
			write(t, wt, "internal/orders/forgot.go", "package orders")
		}, "forgot.go"},
		"slow check": {map[string]string{"internal/orders/a.go": "x"}, func(in *Input, _ func(...string) string, _ string) {
			in.Config.Checks = []Check{{Name: "slow", Run: "sleep 30 & sleep 30; wait", Timeout: "300ms"}}
		}, "timed out"},
		"env file":    {map[string]string{".env": "A=1"}, nil, "env files"},
		"docket data": {map[string]string{".docket/x": "1"}, nil, "docket data"},
		"secret":      {map[string]string{"internal/orders/k.go": "const k = \"sk-ant-abcdefghijklmnopqrstuvwxyz\""}, nil, "Anthropic token"},
		"pr body": {map[string]string{"internal/orders/a.go": "x"}, func(in *Input, _ func(...string) string, _ string) {
			in.PRBody = "## Summary\nonly a summary"
		}, "Acceptance"},
		"failing check": {map[string]string{"internal/orders/a.go": "x"}, func(in *Input, _ func(...string) string, _ string) {
			in.Config.Checks = []Check{{Name: "test", Run: "echo boom; exit 3"}}
		}, "boom"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			wt, git := repoWithPushedBranch(t, c.files)
			in := input(wt)
			if c.mutate != nil {
				c.mutate(&in, git, wt)
			}
			_, err := Run(in)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestGateRiskFlags(t *testing.T) {
	wt, _ := repoWithPushedBranch(t, map[string]string{
		"internal/orders/orders.go":      "package orders\n",
		"internal/orders/orders_test.go": "package orders\n", // tests never count as out of scope
		"migrations/001_orders.sql":      "create table x();\n",
		"internal/auth/token.go":         "package auth\n",
		"go.mod":                         "module x\n",
	})
	in := input(wt)
	in.Config.Checks = nil
	res, err := Run(in)
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(res.Risks, " | ")
	for _, want := range []string{"migration: migrations/001_orders.sql", "auth/security: internal/auth/token.go",
		"dependencies: go.mod", "outside planned files: internal/auth/token.go, migrations/001_orders.sql", "no verify checks"} {
		if !strings.Contains(all, want) {
			t.Errorf("risk %q missing in %q", want, all)
		}
	}
	if strings.Contains(all, "orders_test.go") {
		t.Errorf("test file flagged out of scope: %q", all)
	}
}

func TestSecretMarkerAllowsFixtures(t *testing.T) {
	wt, _ := repoWithPushedBranch(t, map[string]string{
		"internal/orders/fixture_test.go": "const k = \"AKIAABCDEFGHIJKLMNOP\" // gate:allow-secret fake key\n"})
	if _, err := Run(input(wt)); err != nil {
		t.Fatalf("marked fixture must pass: %v", err)
	}
}

func TestInPlanned(t *testing.T) {
	planned := "internal/orders/, cmd/main.go, migrations/*.sql, web/**"
	for p, want := range map[string]bool{
		"internal/orders/a.go": true, "internal/orders2/a.go": false, "cmd/main.go": true, "cmd/other.go": false,
		"migrations/001.sql": true, "migrations/sub/001.sql": false, "web/src/app.tsx": true,
	} {
		if got := InPlanned(p, planned); got != want {
			t.Errorf("InPlanned(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil || len(c.Checks) != 0 {
		t.Fatalf("missing file must be an empty config: %+v %v", c, err)
	}
	f := filepath.Join(t.TempDir(), "checks.yaml")
	_ = os.WriteFile(f, []byte("checks:\n  - name: t\n    run: go test ./...\n    timeout: 2m\n"), 0o644)
	c, err = LoadConfig(f)
	if err != nil || len(c.Checks) != 1 || c.MaxDiffLines != DefaultMaxDiffLines || c.Checks[0].timeout(Budget).Minutes() != 2 {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestEnvLines(t *testing.T) {
	got := EnvLines("# c\nA=1\n\nexport B=2\njunk\n")
	if strings.Join(got, ",") != "A=1,B=2" {
		t.Fatalf("%v", got)
	}
}
