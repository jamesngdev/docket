package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/config"
	"docket/internal/ipc"
)

var errNoSession = errors.New("agent session is not running")

// animate advances per-frame effects: the sliding sidebar marker and usage bars.
func (ws *workspace) animate() {
	ws.sideY = ease(ws.sideY, float64(ws.active), 0.45)
	ws.animateUsage()
}

func (ws *workspace) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case usageMsg:
		ws.usageBusy, ws.usageErr = false, msg.err
		if msg.err == nil { // on error (e.g. 429) keep showing the last good values
			ws.usage, usageCache.limits = msg.limits, msg.limits
		}
		return nil
	case snapshotMsg:
		ws.snap = snapshot(msg)
		return nil
	case flashMsg:
		if msg.text != "" {
			msg.at = time.Now()
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
	case "discord":
		return ws.setDiscordChannel(arg)
	case "automerge":
		return ws.setAutoMerge(arg)
	case "daemon":
		if arg == "stop" {
			return ws.send(ipc.CmdShutdown, "", "daemon stopped (agents keep running)")
		}
		return ws.send(ipc.CmdPing, "", "daemon is alive")
	}
	return flash("command", fmt.Errorf("unknown :%s (try :phase <branch> :rescan :dispatch :techlead <task> :compose :discord <channel-id|off> :automerge on|off :daemon stop :project :q)", f[0]))
}

// setDiscordChannel points Backend Support at a Discord channel ("off" disables it).
func (ws *workspace) setDiscordChannel(arg string) tea.Cmd {
	if arg == "" {
		return flash("discord", fmt.Errorf("usage :discord <channel-id> | :discord off (bot token: docket discord set-token <token>)"))
	}
	ch := arg
	if ch == "off" {
		ch = ""
	} else if strings.Trim(ch, "0123456789") != "" {
		return flash("discord", fmt.Errorf("channel id must be numeric (Discord Developer Mode → right-click channel → Copy Channel ID)"))
	}
	proj, err := ws.db.Project()
	if err != nil {
		return flash("discord", err)
	}
	proj.Settings.DiscordChannel = ch
	msg := "Backend Support now answers @mentions in Discord channel " + ch
	if ch == "" {
		msg = "Discord Connect off"
	} else if cfg, _ := config.Load(); cfg.DiscordToken == "" {
		msg += " — set the bot token: docket discord set-token <token>"
	}
	return ws.dbDo(msg, func() error { return ws.db.SaveSettings(proj.Settings) })
}

// setAutoMerge turns TechLead auto-merge of Tester-approved PRs on or off.
func (ws *workspace) setAutoMerge(arg string) tea.Cmd {
	if arg != "on" && arg != "off" {
		return flash("automerge", fmt.Errorf("usage :automerge on|off"))
	}
	proj, err := ws.db.Project()
	if err != nil {
		return flash("automerge", err)
	}
	proj.Settings.AutoMerge = arg == "on"
	msg := "auto-merge off: approved PRs wait for you"
	if proj.Settings.AutoMerge {
		msg = "auto-merge on: TechLead reviews and squash-merges Tester-approved PRs"
	}
	return ws.dbDo(msg, func() error { return ws.db.SaveSettings(proj.Settings) })
}
