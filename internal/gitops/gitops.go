// Package gitops wraps the git operations docket needs: clone, pull,
// worktrees for tickets, and info/exclude management.
package gitops

import (
	"os"
	"path/filepath"
	"strings"

	"docket/internal/sh"
)

func Clone(url, dest string) error {
	_, err := sh.Run("", "git", "clone", url, dest)
	return err
}

func IsRepo(dir string) bool {
	_, err := sh.Run(dir, "git", "rev-parse", "--git-dir")
	return err == nil
}

// Pull fast-forwards the current branch; failures (no remote, diverged) are returned.
func Pull(dir string) error {
	_, err := sh.Run(dir, "git", "pull", "--ff-only", "-q")
	return err
}

func Fetch(dir string) error {
	_, err := sh.Run(dir, "git", "fetch", "-q", "origin")
	return err
}

func Head(dir string) (string, error) { return sh.Run(dir, "git", "rev-parse", "HEAD") }

// CheckoutBase switches the main checkout to base, creating it from origin if needed.
func CheckoutBase(dir, base string) error {
	if _, err := sh.Run(dir, "git", "checkout", "-q", base); err == nil {
		return nil
	}
	_, err := sh.Run(dir, "git", "checkout", "-q", "-b", base, "origin/"+base)
	return err
}

// ExcludeDataDir appends entry (e.g. `.docket/`) to the repo's info/exclude
// file so app data is never committed and the user's .gitignore is untouched.
func ExcludeDataDir(repo, entry string) error {
	p, err := sh.Run(repo, "git", "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(repo, p)
	}
	b, _ := os.ReadFile(p)
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == entry {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + entry + "\n")
	return err
}

// startPoint prefers origin/<ref> and falls back to the local ref.
func startPoint(repo, ref string) string {
	if _, err := sh.Run(repo, "git", "rev-parse", "--verify", "-q", "origin/"+ref); err == nil {
		return "origin/" + ref
	}
	return ref
}

// AddTicketWorktree creates (or reuses) a worktree on `branch` started from base.
func AddTicketWorktree(repo, wt, branch, base string) error {
	if _, err := os.Stat(wt); err == nil {
		return nil // re-dispatch on the same branch (review loop)
	}
	_ = Fetch(repo)
	if _, err := sh.Run(repo, "git", "rev-parse", "--verify", "-q", branch); err == nil {
		_, err = sh.Run(repo, "git", "worktree", "add", wt, branch)
		return err
	}
	_, err := sh.Run(repo, "git", "worktree", "add", "-b", branch, wt, startPoint(repo, base))
	return err
}

// AddDetachedWorktree checks out a PR branch read-only for the Tester.
func AddDetachedWorktree(repo, wt, branch string) error {
	RemoveWorktree(repo, wt)
	_, _ = sh.Run(repo, "git", "fetch", "-q", "origin", branch)
	_, err := sh.Run(repo, "git", "worktree", "add", "--detach", wt, startPoint(repo, branch))
	return err
}

// RemoveWorktree is best effort: the dir may already be gone.
func RemoveWorktree(repo, wt string) {
	if _, err := os.Stat(wt); err != nil {
		return
	}
	if _, err := sh.Run(repo, "git", "worktree", "remove", "--force", wt); err != nil {
		_ = os.RemoveAll(wt)
		_, _ = sh.Run(repo, "git", "worktree", "prune")
	}
}

// Branches lists local and origin branches of a repo (deduplicated, no HEAD).
func Branches(dir string) ([]string, error) {
	out, err := sh.Run(dir, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var res []string
	for _, b := range strings.Split(out, "\n") {
		b = strings.TrimPrefix(strings.TrimSpace(b), "origin/")
		if b == "" || b == "HEAD" || b == "origin" || seen[b] {
			continue
		}
		seen[b] = true
		res = append(res, b)
	}
	return res, nil
}

// CurrentBranch returns the checked-out branch name.
func CurrentBranch(dir string) (string, error) {
	return sh.Run(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
}
