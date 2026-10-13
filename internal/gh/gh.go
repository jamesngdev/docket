// Package gh queries pull request state through the authenticated `gh` CLI.
package gh

import (
	"os"
	"strings"
	"time"

	"docket/internal/sh"
)

// PRState returns OPEN, MERGED or CLOSED for a PR URL.
func PRState(repo, url string) (string, error) {
	out, err := sh.RunTimeout(30*time.Second, repo, "gh", "pr", "view", url, "--json", "state", "-q", ".state")
	return strings.ToUpper(strings.TrimSpace(out)), err
}

// PRStatus returns the PR state (OPEN/MERGED/CLOSED) and mergeability
// (MERGEABLE/CONFLICTING/UNKNOWN — UNKNOWN while GitHub is still computing).
func PRStatus(repo, url string) (state, mergeable string, err error) {
	out, err := sh.RunTimeout(30*time.Second, repo, "gh", "pr", "view", url, "--json", "state,mergeable",
		"-q", `.state + " " + .mergeable`)
	if err != nil {
		return "", "", err
	}
	f := strings.Fields(strings.ToUpper(out))
	if len(f) < 2 {
		return strings.Join(f, ""), "UNKNOWN", nil
	}
	return f[0], f[1], nil
}

// MergePR squash-merges a PR and deletes its remote branch. It runs outside
// any checkout so gh never tries to delete a local branch that a worktree uses.
func MergePR(url string) error {
	_, err := sh.RunTimeout(2*time.Minute, os.TempDir(), "gh", "pr", "merge", url, "--squash", "--delete-branch")
	return err
}

// PRBranch returns the head branch name of a PR URL.
func PRBranch(repo, url string) (string, error) {
	return sh.RunTimeout(30*time.Second, repo, "gh", "pr", "view", url, "--json", "headRefName", "-q", ".headRefName")
}
