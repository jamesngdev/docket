package gate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"docket/internal/sh"
)

// Input is everything the gate needs about one ticket's change.
type Input struct {
	Worktree string   // the Developer's ticket worktree
	Branch   string   // ticket branch (must be pushed)
	Target   string   // PR target branch the diff is taken against
	Planned  string   // ticket's planned files (comma-separated), "" = no scope check
	PRBody   string   // current PR body, "" = skip the template check
	Env      []string // KEY=value lines of the shared dev environment
	Config   Config
}

// Result of a passed gate.
type Result struct {
	SHA   string   // commit the checks passed on
	Risks []string // empty = low risk: eligible for the fast auto-merge path
}

// Error lists every reason the gate failed, ready to show to the agent.
type Error struct{ Problems []string }

func (e *Error) Error() string {
	return "wait_review gate failed — fix these, push, and run the same command again:\n- " + strings.Join(e.Problems, "\n- ")
}

// Run validates the change. Cheap git checks run first so an agent never waits
// for the test suite just to learn it forgot to push.
func Run(in Input) (Result, error) {
	git := func(args ...string) (string, error) { return sh.RunTimeout(2*time.Minute, in.Worktree, "git", args...) }
	// Untracked (not ignored) files count too: checks would pass on a file the PR does not contain.
	if out, err := git("status", "--porcelain"); err != nil {
		return Result{}, err
	} else if out != "" {
		return Result{}, &Error{[]string{"the worktree is not clean (commit the change, delete scratch files or write them " +
			"outside the worktree, e.g. $TMPDIR):\n" + tail(out, 20)}}
	}
	_, _ = git("fetch", "-q", "origin", in.Branch, in.Target)
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return Result{}, err
	}
	if remote, _ := git("rev-parse", "origin/"+in.Branch); remote != head {
		return Result{}, &Error{[]string{fmt.Sprintf("HEAD %s is not pushed to origin/%s: git push first", short(head), in.Branch)}}
	}
	base := "origin/" + in.Target + "...HEAD"
	changes, err := changedFiles(git, base)
	if err != nil {
		return Result{}, err
	}
	if len(changes) == 0 {
		return Result{}, &Error{[]string{"the branch has no changes against " + in.Target}}
	}
	diff, err := git("diff", "-U0", base)
	if err != nil {
		return Result{}, err
	}
	problems := blockingFiles(changes)
	secrets, risks := scanAddedLines(diff)
	problems = append(problems, secrets...)
	problems = append(problems, prBodyProblems(in.PRBody)...)
	if len(problems) > 0 {
		return Result{}, &Error{problems}
	}
	deadline := time.Now().Add(Budget)
	for _, c := range in.Config.Checks {
		left := time.Until(deadline)
		if left <= 0 {
			return Result{}, &Error{[]string{fmt.Sprintf("checks exceeded the %s gate budget before %q; make checks.yaml faster "+
				"(TechLead: run a unit subset, leave the rest to CI)", Budget, c.Name)}}
		}
		if out, err := runCheck(in.Worktree, c, in.Env, c.timeout(left)); err != nil {
			return Result{}, &Error{[]string{fmt.Sprintf("check %q (%s) failed: %v\n%s", c.Name, c.Run, err, tail(out, 80))}}
		}
	}
	lines, _ := diffLines(git, base)
	risks = append(risks, riskFlags(changes, lines, in.Config.MaxDiffLines, in.Planned)...)
	if len(in.Config.Checks) == 0 {
		risks = append(risks, "no verify checks configured (.docket/checks.yaml)")
	}
	return Result{SHA: head, Risks: risks}, nil
}

func changedFiles(git func(...string) (string, error), base string) ([]Change, error) {
	out, err := git("diff", "--name-status", base)
	if err != nil {
		return nil, err
	}
	var cs []Change
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 {
			continue
		}
		cs = append(cs, Change{Status: f[0][:1], Path: f[len(f)-1]}) // renames: keep the new path
	}
	return cs, nil
}

// diffLines sums added+deleted lines, ignoring binary files.
func diffLines(git func(...string) (string, error), base string) (int, error) {
	out, err := git("diff", "--numstat", base)
	n := 0
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		a, _ := strconv.Atoi(f[0])
		d, _ := strconv.Atoi(f[1])
		n += a + d
	}
	return n, err
}

// runCheck runs one verify command with the shared dev env and returns its
// combined output. It runs in its own process group so a timeout also kills
// test runners and servers it spawned (they would otherwise hold the output
// pipe open and keep ports busy for the next attempt).
func runCheck(dir string, c Check, env []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Run)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("timed out after %s", timeout.Round(time.Second))
	}
	return out.String(), err
}

// EnvLines parses a KEY=value env file, skipping blanks and comments.
func EnvLines(content string) []string {
	var out []string
	for _, l := range strings.Split(content, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || !strings.Contains(l, "=") {
			continue
		}
		out = append(out, strings.TrimPrefix(l, "export "))
	}
	return out
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = append([]string{fmt.Sprintf("… (%d earlier lines)", len(lines)-n)}, lines[len(lines)-n:]...)
	}
	return strings.Join(lines, "\n")
}

func short(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}
