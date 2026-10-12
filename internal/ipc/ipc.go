// Package ipc is the tiny newline-delimited JSON protocol between the TUI /
// agent CLI and the per-project daemon over a unix socket.
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Commands understood by the daemon.
const (
	CmdPing     = "ping"
	CmdKick     = "kick"     // re-run the scheduling loop now
	CmdTechlead = "techlead" // Arg = task
	CmdRescan   = "rescan"   // pull docs, diff, run TechLead if changed
	CmdStop     = "stop"     // Arg = run id
	CmdRestart  = "restart"  // Arg = run id
	CmdCompose  = "compose"  // docker compose up the shared dev stack
	CmdShutdown = "shutdown"
	CmdPhase    = "phase"    // Arg = docs branch to make the active phase
	CmdRedo     = "redo"     // Arg = ticket id: stop its agents and re-dispatch on the same branch
	CmdCheckPRs = "checkprs" // poll PR state + merge conflicts now
)

type Request struct {
	Cmd string `json:"cmd"`
	Arg string `json:"arg,omitempty"`
}

type Response struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Version string `json:"version,omitempty"` // binary identity of the daemon (set on ping)
}

// Serve listens on sock and answers each request with handle.
func Serve(sock string, handle func(Request) Response) (net.Listener, error) {
	_ = os.Remove(sock) // stale socket from a crashed daemon
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(10 * time.Second))
				var req Request
				if err := json.NewDecoder(bufio.NewReader(c)).Decode(&req); err != nil {
					return
				}
				_ = json.NewEncoder(c).Encode(handle(req))
			}(c)
		}
	}()
	return ln, nil
}

// Call sends one request and waits for the response.
func Call(sock string, req Request) error {
	_, err := CallResp(sock, req)
	return err
}

// CallResp is Call that also returns the daemon's response.
func CallResp(sock string, req Request) (Response, error) {
	var resp Response
	c, err := net.DialTimeout("unix", sock, 2*time.Second)
	if err != nil {
		return resp, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewEncoder(c).Encode(req); err != nil {
		return resp, err
	}
	if err := json.NewDecoder(c).Decode(&resp); err != nil {
		return resp, err
	}
	if !resp.OK {
		return resp, errors.New(resp.Error)
	}
	return resp, nil
}

// BinaryVersion identifies the running executable (path + size + mtime), so a
// client can tell when the daemon was started from an older docket binary.
func BinaryVersion() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	st, err := os.Stat(exe)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", exe, st.Size(), st.ModTime().UnixNano())
}
