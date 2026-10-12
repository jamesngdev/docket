package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/project"
)

// Alive reports whether the project's daemon answers a ping.
func Alive(p paths.Project) bool {
	return ipc.Call(p.Socket(), ipc.Request{Cmd: ipc.CmdPing}) == nil
}

// EnsureRunning starts a detached daemon for the project if none answers.
// The daemon survives the TUI (new session, stdio to the daemon log).
//
// A daemon started from an older docket binary (e.g. before `make install`) is
// stopped and replaced, so new commands are never rejected as unknown.
// Agents in tmux are unaffected; the new daemon re-attaches by session name.
func EnsureRunning(p paths.Project) error {
	if err := project.MigrateLegacy(p.Repo); err != nil {
		return err
	}
	if resp, err := ipc.CallResp(p.Socket(), ipc.Request{Cmd: ipc.CmdPing}); err == nil {
		if resp.Version == ipc.BinaryVersion() {
			return nil
		}
		_ = Stop(p)
		for i := 0; i < 50 && Alive(p); i++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err := p.EnsureLayout(); err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(p.DaemonLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(bin, "daemon", "--project", p.Repo)
	cmd.Dir = p.Repo
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if Alive(p) {
			return nil
		}
	}
	return fmt.Errorf("daemon did not come up; see %s", p.DaemonLog())
}

// Stop asks the daemon to exit. Agent tmux sessions keep running.
func Stop(p paths.Project) error {
	return ipc.Call(p.Socket(), ipc.Request{Cmd: ipc.CmdShutdown})
}

// Send is a convenience wrapper used by the TUI and CLI.
func Send(p paths.Project, cmd, arg string) error {
	return ipc.Call(p.Socket(), ipc.Request{Cmd: cmd, Arg: arg})
}
