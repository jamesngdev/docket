// Package gh queries pull request state through the authenticated `gh` CLI.
package gh

import (
	"encoding/json"
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
// A non-empty head makes GitHub refuse the merge if the PR moved past that commit.
func MergePR(url, head string) error {
	args := []string{"pr", "merge", url, "--squash", "--delete-branch"}
	if head != "" {
		args = append(args, "--match-head-commit", head)
	}
	_, err := sh.RunTimeout(2*time.Minute, os.TempDir(), "gh", args...)
	return err
}

// PRBranch returns the head branch name of a PR URL.
func PRBranch(repo, url string) (string, error) {
	return sh.RunTimeout(30*time.Second, repo, "gh", "pr", "view", url, "--json", "headRefName", "-q", ".headRefName")
}

// PRBody returns the PR description ("" when gh cannot read it).
func PRBody(repo, url string) string {
	out, _ := sh.RunTimeout(30*time.Second, repo, "gh", "pr", "view", url, "--json", "body", "-q", ".body")
	return out
}

// PRHeadAndChecks returns the PR head commit and the state of its CI checks:
// "pass" (all passed or the repo has none), "pending" or "fail".
func PRHeadAndChecks(repo, url string) (head, checks string, err error) {
	out, err := sh.RunTimeout(30*time.Second, repo, "gh", "pr", "view", url, "--json", "headRefOid,statusCheckRollup")
	if err != nil {
		return "", "", err
	}
	var v struct {
		HeadRefOid        string      `json:"headRefOid"`
		StatusCheckRollup []CheckItem `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return "", "", err
	}
	return v.HeadRefOid, rollup(v.StatusCheckRollup), nil
}

// CheckItem is one entry of a PR's statusCheckRollup (a CheckRun or a StatusContext).
type CheckItem struct {
	Status     string `json:"status"`     // CheckRun: QUEUED, IN_PROGRESS, COMPLETED
	Conclusion string `json:"conclusion"` // CheckRun: SUCCESS, FAILURE, NEUTRAL, SKIPPED, …
	State      string `json:"state"`      // StatusContext: SUCCESS, PENDING, FAILURE, ERROR, EXPECTED
}

// rollup folds CI checks into pass / pending / fail; no checks is a pass.
func rollup(items []CheckItem) string {
	state := "pass"
	for _, c := range items {
		switch {
		case c.State != "":
			switch strings.ToUpper(c.State) {
			case "SUCCESS":
			case "PENDING", "EXPECTED":
				state = worse(state, "pending")
			default:
				state = "fail"
			}
		case strings.ToUpper(c.Status) != "COMPLETED":
			state = worse(state, "pending")
		default:
			switch strings.ToUpper(c.Conclusion) {
			case "SUCCESS", "NEUTRAL", "SKIPPED":
			default:
				state = "fail"
			}
		}
	}
	return state
}

func worse(cur, next string) string {
	if cur == "fail" {
		return cur
	}
	return next
}
