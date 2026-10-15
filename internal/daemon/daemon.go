// Package daemon is the per-project background orchestrator. It owns
// dispatch, tmux run lifecycle, the review loop, PR polling and TechLead
// tasks. All state lives in SQLite so a restart resumes where it left off.
package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"docket/internal/config"
	"docket/internal/gitops"
	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/project"
	"docket/internal/store"
)

const (
	tickEvery   = 5 * time.Second
	pollPREvery = 60 * time.Second
)

type Daemon struct {
	paths    paths.Project
	db       *store.DB
	actions  chan ipc.Request
	stop     chan struct{}
	lastPoll time.Time
	stopOnce sync.Once
	version  string // ipc.BinaryVersion at start; lets clients detect a stale daemon
	discord  discordState
	// ciPendingSince remembers when an auto-merge candidate was first seen with
	// CI still running (scheduler goroutine only; reset on restart is fine).
	ciPendingSince map[int64]time.Time
}

// Run blocks until shutdown. It is started by `docket daemon --project <repo>`.
func Run(repo string) error {
	if err := project.MigrateLegacy(repo); err != nil {
		return err
	}
	p := paths.New(repo)
	if err := p.EnsureLayout(); err != nil {
		return err
	}
	db, err := store.Open(p.DB())
	if err != nil {
		return err
	}
	defer db.Close()
	d := &Daemon{paths: p, db: db, actions: make(chan ipc.Request, 32), stop: make(chan struct{}),
		version: ipc.BinaryVersion()}
	ln, err := ipc.Serve(p.Socket(), d.handle)
	if err != nil {
		return fmt.Errorf("listen %s: %w", p.Socket(), err)
	}
	defer ln.Close()
	defer os.Remove(p.Socket())
	_ = os.WriteFile(p.PIDFile(), []byte(strconv.Itoa(os.Getpid())), 0o644)
	defer os.Remove(p.PIDFile())
	db.Log("daemon", "started (pid %d)", os.Getpid())

	// First start of a fresh project: scan docs and kick off TechLead bootstrap.
	if proj, err := db.Project(); err == nil && !db.AnyPhaseScanned() && proj.DocsRepoURL != "" {
		d.actions <- ipc.Request{Cmd: ipc.CmdRescan}
	}

	d.startQueuedTechlead()
	defer d.closeDiscord()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-sig:
			db.Log("daemon", "stopped by signal")
			return nil
		case <-d.stop:
			db.Log("daemon", "stopped")
			return nil
		case req := <-d.actions:
			d.do(req)
			d.tick()
		case <-ticker.C:
			d.tick()
		}
	}
}

// handle answers socket requests. Work is queued to the loop goroutine so
// slow operations (git, codegraph) never block a ping from the TUI.
func (d *Daemon) handle(req ipc.Request) ipc.Response {
	switch req.Cmd {
	case ipc.CmdPing:
		return ipc.Response{OK: true, Version: d.version}
	case ipc.CmdShutdown:
		d.stopOnce.Do(func() { close(d.stop) })
		return ipc.Response{OK: true}
	case ipc.CmdKick, ipc.CmdTechlead, ipc.CmdRescan, ipc.CmdStop, ipc.CmdRestart, ipc.CmdCompose, ipc.CmdPhase, ipc.CmdRedo, ipc.CmdCheckPRs:
		select {
		case d.actions <- req:
			return ipc.Response{OK: true}
		default:
			return ipc.Response{Error: "daemon busy, try again"}
		}
	}
	return ipc.Response{Error: "unknown command " + req.Cmd}
}

func (d *Daemon) do(req ipc.Request) {
	var err error
	switch req.Cmd {
	case ipc.CmdTechlead:
		err = d.startTechlead(req.Arg)
	case ipc.CmdRescan:
		err = d.rescanDocs()
	case ipc.CmdStop:
		err = d.stopRun(req.Arg, false)
	case ipc.CmdRestart:
		err = d.stopRun(req.Arg, true)
	case ipc.CmdCompose:
		err = d.composeUp()
	case ipc.CmdPhase:
		err = d.switchPhase(req.Arg)
	case ipc.CmdRedo:
		err = d.redoTicket(req.Arg)
	case ipc.CmdCheckPRs:
		d.lastPoll = time.Time{} // tick() right after do() polls immediately
	}
	if err != nil {
		d.db.Log("daemon", "%s %s failed: %v", req.Cmd, req.Arg, err)
		log.Printf("%s %s: %v", req.Cmd, req.Arg, err)
	}
}

// tick is one scheduling pass. Each step is independent and logs its own errors.
func (d *Daemon) tick() {
	d.reapRuns()
	d.reviewLoop()
	d.startTesters()
	d.dispatch()
	d.syncDiscord()
	if time.Since(d.lastPoll) >= pollPREvery {
		d.lastPoll = time.Now()
		d.pollPRs()
		_ = gitops.Fetch(d.paths.Docs()) // keeps the phase list (docs branches) fresh for the TUI
	}
}

func (d *Daemon) project() (store.Project, error) { return d.db.Project() }

func (d *Daemon) token() string {
	c, _ := config.Load()
	return c.OAuthToken
}
