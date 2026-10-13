// Package usage reads the Claude subscription rate-limit usage (5-hour session
// and weekly windows) for the account docket's agents run on.
//
// It only ever READS the current access token: it never refreshes it, because
// OAuth refresh tokens rotate and refreshing outside Claude Code would log the
// user out. An expired token just yields an error until Claude refreshes it.
package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const endpoint = "https://api.anthropic.com/api/oauth/usage"

// Window is one rate-limit window.
type Window struct {
	Percent  float64   `json:"utilization"`
	ResetsAt time.Time `json:"resets_at"`
}

// Limits is the subset of the usage response docket shows.
type Limits struct {
	Session *Window `json:"five_hour"`
	Weekly  *Window `json:"seven_day"`
	Opus    *Window `json:"seven_day_opus"`
	Sonnet  *Window `json:"seven_day_sonnet"`
}

// Fetch returns current usage. token is docket's configured agent token; when
// empty the user's own Claude Code login is used.
func Fetch(ctx context.Context, token string) (Limits, error) {
	var l Limits
	if token == "" {
		t, err := loginToken()
		if err != nil {
			return l, err
		}
		token = t
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return l, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return l, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return l, fmt.Errorf("usage API: %s", resp.Status)
	}
	return l, json.NewDecoder(resp.Body).Decode(&l)
}

// loginToken reads the access token of the default Claude Code login:
// macOS keychain item first, then ~/.claude/.credentials.json.
func loginToken() (string, error) {
	var raw []byte
	if runtime.GOOS == "darwin" {
		raw, _ = exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	}
	if len(raw) == 0 {
		home, _ := os.UserHomeDir()
		raw, _ = os.ReadFile(filepath.Join(home, ".claude", ".credentials.json"))
	}
	var c struct {
		OAuth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &c); err != nil || c.OAuth.AccessToken == "" {
		return "", errors.New("no Claude login found")
	}
	return c.OAuth.AccessToken, nil
}
