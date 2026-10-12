package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/ipc"
)

var errNoSession = errors.New("agent session is not running")

func (ws *workspace) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case snapshotMsg:
		ws.snap = snapshot(msg)
		return nil
	case flashMsg:
		if msg.text != "" {
			ws.flash = msg
		}
		return nil
	case tea.KeyMsg:
		return ws.key(msg)
	case chatStartedMsg:
		return tea.Batch(ws.load(), ws.attach(msg.session))
	}
	if ws.editor != nil {
		return ws.editor.update(ws, msg)
	}
	if ws.cmdMode {
		var cmd tea.Cmd
		ws.cmdline, cmd = ws.cmdline.Update(msg)
		return cmd
	}
	return nil
}

func (ws *workspace) key(k tea.KeyMsg) tea.Cmd {
	if ws.editor != nil {
		return ws.editor.update(ws, k)
	}
	if ws.cmdMode {
		switch k.String() {
		case "esc":
			ws.cmdMode = false
			return nil
		case "enter":
			ws.cmdMode = false
			return ws.runCommand(strings.TrimSpace(ws.cmdline.Value()))
		}
		var cmd tea.Cmd
		ws.cmdline, cmd = ws.cmdline.Update(k)
		return cmd
	}
	if ws.help {
		ws.help = false
		return nil
	}
	switch s := k.String(); s {
	case ":":
		ws.cmdMode = true
		ws.cmdline.SetValue("")
		return ws.cmdline.Focus()
	case "?":
		ws.help = true
		return nil
	case "q":
		return func() tea.Msg { return toPickerMsg{} }
	case "tab":
		ws.active = (ws.active + 1) % len(ws.views)
		return ws.load()
	case "shift+tab":
		ws.active = (ws.active + len(ws.views) - 1) % len(ws.views)
		return ws.load()
	case "1", "2", "3", "4", "5", "6":
		ws.active = int(s[0] - '1')
		return ws.load()
	case "n":
		_, run := nextStep(ws)
		return run()
	}
	return ws.views[ws.active].keys(ws, k)
}

// runCommand executes a `:` command line.
func (ws *workspace) runCommand(line string) tea.Cmd {
	f := strings.Fields(line)
	if len(f) == 0 {
		return nil
	}
	arg := ""
	if len(f) > 1 {
		arg = f[1]
	}
	switch f[0] {
	case "q", "quit":
		return tea.Quit
	case "p", "project":
		return func() tea.Msg { return toPickerMsg{} }
	case "phase":
		if arg == "" {
			return flash("phase", fmt.Errorf("usage :phase <docs branch>"))
		}
		return ws.send(ipc.CmdPhase, arg, "switching to phase "+arg)
	case "rescan":
		return ws.send(ipc.CmdRescan, "", "docs rescan requested")
	case "dispatch":
		return ws.send(ipc.CmdKick, "", "dispatch pass requested")
	case "techlead":
		return ws.send(ipc.CmdTechlead, arg, "TechLead "+arg+" requested")
	case "compose":
		return ws.send(ipc.CmdCompose, "", "docker compose up requested")
	case "daemon":
		if arg == "stop" {
			return ws.send(ipc.CmdShutdown, "", "daemon stopped (agents keep running)")
		}
		return ws.send(ipc.CmdPing, "", "daemon is alive")
	}
	return flash("command", fmt.Errorf("unknown :%s (try :phase <branch> :rescan :dispatch :techlead <task> :compose :daemon stop :project :q)", f[0]))
}
