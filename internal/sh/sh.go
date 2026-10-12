// Package sh is a tiny wrapper for running external commands with useful errors.
package sh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Run executes name+args in dir and returns trimmed stdout. On failure the
// error includes stderr so callers can surface it directly.
func Run(dir, name string, args ...string) (string, error) {
	return RunTimeout(10*time.Minute, dir, name, args...)
}

func RunTimeout(d time.Duration, dir, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return "", fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, msg)
	}
	return strings.TrimSpace(out.String()), nil
}

// Has reports whether a binary is on PATH.
func Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
