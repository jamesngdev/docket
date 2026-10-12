// Package tmux manages the detached tmux sessions that host agent processes.
package tmux

import (
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"docket/internal/sh"
)

// Spec describes a session to launch.
type Spec struct {
	Name    string
	Dir     string
	Env     map[string]string
	Command string // run through `sh -c`
	LogFile string // pane output is piped here continuously
}

func Start(s Spec) error {
	args := []string{"new-session", "-d", "-s", s.Name, "-c", s.Dir, "-x", "220", "-y", "50"}
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+s.Env[k])
	}
	args = append(args, "sh", "-c", s.Command)
	if _, err := sh.Run(s.Dir, "tmux", args...); err != nil {
		return err
	}
	if s.LogFile != "" {
		_, _ = sh.Run("", "tmux", "pipe-pane", "-o", "-t", "="+s.Name+":", "cat >> "+Quote(s.LogFile))
	}
	return nil
}

// List returns the names of all tmux sessions (empty when no server runs).
func List() map[string]bool {
	out, err := sh.Run("", "tmux", "list-sessions", "-F", "#{session_name}")
	res := map[string]bool{}
	if err != nil {
		return res
	}
	for _, n := range strings.Split(out, "\n") {
		if n != "" {
			res[n] = true
		}
	}
	return res
}

func Has(name string) bool {
	return exec.Command("tmux", "has-session", "-t", "="+name).Run() == nil
}

func Kill(name string) {
	_ = exec.Command("tmux", "kill-session", "-t", "="+name).Run()
}

// Capture returns the last n non-empty-trailing lines of the session's pane.
func Capture(name string, n int) string {
	out, err := sh.Run("", "tmux", "capture-pane", "-p", "-t", "="+name+":", "-S", "-"+strconv.Itoa(n*4))
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(out, "\n "), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// AttachCmd builds the command the TUI hands to tea.ExecProcess. TMUX is
// cleared so attaching also works when docket itself runs inside tmux.
func AttachCmd(name string) *exec.Cmd {
	c := exec.Command("tmux", "attach-session", "-t", "="+name)
	c.Env = append(os.Environ(), "TMUX=")
	return c
}

// Quote single-quotes s for safe use inside `sh -c` strings.
func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
