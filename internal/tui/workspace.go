package tui

import (
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/config"
	"docket/internal/daemon"
	"docket/internal/gitops"
	"docket/internal/paths"
	"docket/internal/project"
	"docket/internal/sh"
	"docket/internal/store"
	"docket/internal/tmux"
	"docket/internal/usage"
)

// snapshot is everything views render, reloaded from SQLite every tick.
type snapshot struct {
	project   store.Project
	questions []store.Question
	features  []store.Feature
	tickets   []store.Ticket
	runs      []store.Run
	docs      []store.Document
	third     []store.ThirdDoc
	activity  []store.Activity
	daemonUp  bool
	pane      string          // last lines of the selected agent's tmux pane
	phases    []string        // docs branches (loaded on the Docs view only)
	sessions  map[string]bool // live tmux sessions (question chats)
	discord   bool            // Discord Connect configured (bot token + project channel)
	err       error
}

type snapshotMsg snapshot

// view is one sidebar entry of the workspace.
type view interface {
	name() string
	badge(s *snapshot) int
	keys(ws *workspace, k tea.KeyMsg) tea.Cmd
	render(ws *workspace, w, h int) string
	hints() string
}

// workspace is the open-project screen.
type workspace struct {
	paths   paths.Project
	db      *store.DB
	snap    snapshot
	views   []view
	active  int
	editor  *editor
	cmdline textinput.Model
	cmdMode bool
	help    bool
	flash   flashMsg
	missing string // required binaries not on PATH, shown in the header
	agents  *agentsView
	pending []tea.Cmd // background work queued by editor callbacks

	usage      usage.Limits // Claude subscription usage (refreshed by refreshUsage)
	usageErr   error
	usageAt    time.Time
	usageBusy  bool       // a usage fetch is in flight
	usageShown [4]float64 // eased bar values per window (see usageRows)
	sideY      float64    // animated sidebar marker position (eases toward active)
}

func openWorkspace(repo string) (*workspace, error) {
	if err := project.MigrateLegacy(repo); err != nil {
		return nil, err
	}
	p := paths.New(repo)
	db, err := store.Open(p.DB())
	if err != nil {
		return nil, err
	}
	ws := &workspace{paths: p, db: db, cmdline: textinput.New(), agents: &agentsView{}}
	ws.cmdline.Prompt = ":"
	ws.views = []view{&dashboardView{}, &questionsView{}, &featuresView{}, &ticketsView{}, ws.agents, &docsView{}}
	var missing []string
	for _, bin := range []string{"gh", "docker", "tmux", "claude", "codegraph"} {
		if !sh.Has(bin) {
			missing = append(missing, bin)
		}
	}
	if len(missing) > 0 {
		ws.missing = sErr.Render("✗ missing " + strings.Join(missing, ", "))
	}
	if err := daemon.EnsureRunning(p); err != nil {
		ws.flash = flashMsg{text: "daemon: " + err.Error(), err: true}
	}
	return ws, nil
}

func (ws *workspace) close() { _ = ws.db.Close() }

// load reads a fresh snapshot in the background.
func (ws *workspace) load() tea.Cmd {
	db, p := ws.db, ws.paths
	session := ws.agents.selectedSession(&ws.snap, ws.views[ws.active] == ws.agents)
	wantPhases := ws.active == vDocs
	return func() tea.Msg {
		var s snapshot
		s.project, s.err = db.Project()
		s.questions, _ = db.Questions()
		s.features, _ = db.Features()
		s.tickets, _ = db.Tickets()
		s.runs, _ = db.Runs(60)
		s.docs, _ = db.Documents()
		s.third, _ = db.ThirdDocs()
		s.activity, _ = db.RecentActivity(12)
		s.daemonUp = daemon.Alive(p)
		s.sessions = tmux.List()
		if cfg, err := config.Load(); err == nil {
			s.discord = cfg.DiscordToken != "" && s.project.Settings.DiscordChannel != ""
		}
		if wantPhases {
			s.phases, _ = gitops.Branches(p.Docs())
		}
		if session != "" {
			s.pane = tmux.Capture(session, 6)
		}
		return snapshotMsg(s)
	}
}

// send issues a daemon command and flashes the outcome.
func (ws *workspace) send(cmd, arg, okText string) tea.Cmd {
	p := ws.paths
	return func() tea.Msg {
		if err := daemon.EnsureRunning(p); err != nil { // also replaces a daemon from an older binary
			return flashMsg{text: "daemon: " + err.Error(), err: true}
		}
		if err := daemon.Send(p, cmd, arg); err != nil {
			return flashMsg{text: cmd + ": " + err.Error(), err: true}
		}
		return flashMsg{text: okText}
	}
}

// attach suspends the TUI and attaches to an agent's tmux session.
func (ws *workspace) attach(session string) tea.Cmd {
	if session == "" || !tmux.Has(session) {
		return flash("attach", errNoSession)
	}
	return tea.ExecProcess(tmux.AttachCmd(session), func(err error) tea.Msg {
		if err != nil {
			return flashMsg{text: "tmux: " + err.Error(), err: true}
		}
		return flashMsg{text: "detached from " + session}
	})
}

// pager opens a file in less (end of file) and returns to the TUI on quit.
func (ws *workspace) pager(path string) tea.Cmd {
	return tea.ExecProcess(exec.Command("less", "-R", "+G", path), func(err error) tea.Msg {
		if err != nil {
			return flashMsg{text: "less: " + err.Error(), err: true}
		}
		return nil
	})
}

// dbDo runs a DB mutation and reloads.
func (ws *workspace) dbDo(okText string, fn func() error) tea.Cmd {
	if err := fn(); err != nil {
		return flash(okText, err)
	}
	return tea.Batch(flash(okText, nil), ws.load(), ws.send("kick", "", okText))
}

func (ws *workspace) ticketByID(id int64) (store.Ticket, bool) {
	for _, t := range ws.snap.tickets {
		if t.ID == id {
			return t, true
		}
	}
	return store.Ticket{}, false
}

func (ws *workspace) runByID(id string) (store.Run, bool) {
	for _, r := range ws.snap.runs {
		if r.ID == id {
			return r, true
		}
	}
	return store.Run{}, false
}
