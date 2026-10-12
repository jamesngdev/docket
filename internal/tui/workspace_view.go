package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"docket/internal/agent"
	"docket/internal/store"
)

const sidebarW = 15

func (ws *workspace) view(w, h int) string {
	header := ws.header(w)
	status := ws.statusBar(w)
	bodyH := h - lipgloss.Height(header) - lipgloss.Height(status) - 2
	mainW := w - sidebarW - 5
	var main string
	switch {
	case ws.editor != nil:
		main = ws.editor.view(mainW, bodyH)
	case ws.help:
		main = helpText()
	default:
		main = ws.views[ws.active].render(ws, mainW, bodyH)
	}
	side := lipgloss.NewStyle().Width(sidebarW).Height(bodyH).Render(ws.sidebar())
	sep := sMuted.Render(strings.Repeat("│\n", max(bodyH-1, 0)) + "│")
	body := lipgloss.JoinHorizontal(lipgloss.Top, side, sep, " ", lipgloss.NewStyle().Width(mainW).Height(bodyH).MaxHeight(bodyH).Render(main))
	return sFrame.Width(w - 2).Render(lipgloss.JoinVertical(lipgloss.Left, header, body, status))
}

func (ws *workspace) header(w int) string {
	s := &ws.snap
	running, devs := 0, 0
	for _, r := range s.runs {
		if r.Status == store.RunRunning {
			running++
			if r.Role == agent.RoleDeveloper {
				devs++
			}
		}
	}
	d := sOK.Render("daemon ✓")
	if !s.daemonUp {
		d = sErr.Render("daemon ✗")
	}
	left := sTitle.Render(" docket ") + "─ " + sBold.Render(s.project.Name) + sMuted.Render(" · phase ") + sAccent.Render(s.project.ActivePhase)
	right := fmt.Sprintf("agents %d (dev %d/%d)  %s  %s ", running, devs, s.project.Settings.MaxParallelAgents, ws.health, d)
	gap := w - 4 - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return trunc(left+" "+right, w-4)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (ws *workspace) sidebar() string {
	var b strings.Builder
	for i, v := range ws.views {
		label := fmt.Sprintf("%d %s", i+1, v.name())
		if n := v.badge(&ws.snap); n > 0 {
			label = fmt.Sprintf("%-11s%s", label, sWarn.Render(fmt.Sprint(n)))
		}
		if i == ws.active {
			b.WriteString(sSelected.Render("▸ "+label) + "\n")
		} else {
			b.WriteString("  " + label + "\n")
		}
	}
	return b.String()
}

func (ws *workspace) statusBar(w int) string {
	sep := sMuted.Render(strings.Repeat("─", w-4))
	if ws.cmdMode {
		return sep + "\n" + ws.cmdline.View()
	}
	label, _ := nextStep(ws)
	hint := ws.views[ws.active].hints() + " · n " + label + " · : cmd · ? help · q projects"
	if ws.editor != nil {
		hint = "-- INSERT --  Ctrl-S save · Esc cancel"
	}
	line := sMuted.Render(trunc(hint, w-4))
	if ws.flash.text != "" {
		st := sOK
		if ws.flash.err {
			st = sErr
		}
		line = st.Render(trunc(ws.flash.text, w-4)) + "\n" + line
	}
	if ws.snap.err != nil {
		line = sErr.Render(trunc("db: "+ws.snap.err.Error(), w-4)) + "\n" + line
	}
	return sep + "\n" + line
}

func helpText() string {
	return sTitle.Render("Keys") + `

  Tab / Shift-Tab / 1-6   switch view
  j k  g G                move · top · bottom
  h l                     ticket status filter
  Enter                   open / primary action (attach agent)
  e t C d                 edit field (answer/description · title · acceptance · deps)
  c  X                    chat with an agent about the question/ticket · end that chat
  N                       new ticket: describe it to TechLead, it creates the ticket(s) in backlog
  a  A                    approve · approve all
  x                       reject / stop agent
  s                       skip question
  r                       rescan docs / restart agent
  o  L                    open PR in browser · open agent log
  n                       run the next pipeline step
  :                       command (:phase <branch> :rescan :dispatch :techlead <task> :compose :daemon stop :project :q)
  q                       back to projects · Ctrl-C quit

press any key to close`
}
