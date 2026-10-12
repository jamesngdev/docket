package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// fixPath gives the GUI process the user's login-shell PATH (plus common tool
// dirs), because apps started from Finder/Dock only see /usr/bin:/bin:/usr/sbin:/sbin.
func fixPath() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	out, err := exec.CommandContext(ctx, shell, "-ilc", `printf "__DOCKET_PATH__%s" "$PATH"`).Output()
	login := ""
	if err == nil {
		if i := strings.LastIndex(string(out), "__DOCKET_PATH__"); i >= 0 {
			login = strings.TrimSpace(string(out)[i+len("__DOCKET_PATH__"):])
		}
	}
	home, _ := os.UserHomeDir()
	extra := []string{filepath.Join(home, ".local", "bin"), "/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, "go", "bin")}
	seen := map[string]bool{}
	var parts []string
	for _, p := range append(append(strings.Split(login, ":"), strings.Split(os.Getenv("PATH"), ":")...), extra...) {
		if p != "" && !seen[p] {
			seen[p] = true
			parts = append(parts, p)
		}
	}
	_ = os.Setenv("PATH", strings.Join(parts, ":"))
}
