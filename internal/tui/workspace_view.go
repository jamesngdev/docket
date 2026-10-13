package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"docket/internal/agent"
	"docket/internal/store"
)

const sidebarW = 17

// viewIcons are the sidebar glyphs, in workspace.views order.
var viewIcons = []string{"◆", "?", "✦", "▤", "◉", "≡"}

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
	sep := sRule.Render(strings.Repeat("│\n", max(bodyH-1, 0)) + "│")
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
	left := shimmer(" ◆ docket ", sTitle) + " " + sBold.Render(s.project.Name) + " " + pill("⎇ "+orDash(s.project.ActivePhase), colAccent, colSelBg)
	agents := sMuted.Render(fmt.Sprintf("○ idle · dev 0/%d", s.project.Settings.MaxParallelAgents))
	if running > 0 {
		agents = sAccent.Render(spin(frame)+fmt.Sprintf(" %d running", running)) +
			sMuted.Render(fmt.Sprintf(" · dev %d/%d", devs, s.project.Settings.MaxParallelAgents))
	}
	right := []string{agents}
	if s.project.Settings.AutoMerge {
		right = append(right, sOK.Render("⇢ auto-merge"))
	}
	if s.discord {
		right = append(right, sInfo.Render("◉ discord"))
	}
	if u := ws.usageBrief(); u != "" {
		right = append(right, u)
	}
	if ws.missing != "" {
		right = append(right, ws.missing)
	}
	if !s.daemonUp {
		right = append(right, sErr.Render(pulse("✗", "·")+" daemon down"))
	}
	r := strings.Join(right, sRule.Render("  │  ")) + " "
	gap := w - 2 - ansi.StringWidth(left) - ansi.StringWidth(r)
	if gap < 1 {
		return trunc(left+" "+r, w-2)
	}
	return left + strings.Repeat(" ", gap) + r
}

// sidebar lists the views; a marker glides to the active one when switching.
func (ws *workspace) sidebar() string {
	marker := int(math.Round(ws.sideY))
	var b strings.Builder
	b.WriteString("\n")
	for i, v := range ws.views {
		label := fmt.Sprintf("%s %s", viewIcons[i], v.name())
		badge := ""
		if n := v.badge(&ws.snap); n > 0 {
			badge = fmt.Sprint(n)
		}
		bar := " "
		if i == marker {
			bar = sAccent.Render("▌")
		}
		row := bar + sMuted.Render(fmt.Sprintf(" %-*s", sidebarW-4, label)) + sWarn.Bold(true).Render(badge)
		if i == ws.active {
			row = bar + sSelRow.Width(sidebarW-2).Render(fmt.Sprintf(" %-*s%s", sidebarW-4, label, badge))
		}
		b.WriteString(mark(fmt.Sprintf("view:%d", i), row) + "\n")
	}
	b.WriteString("\n" + sRule.Render(" "+strings.Repeat("─", sidebarW-3)) + "\n")
	for _, k := range []string{"? help", ": command", "q projects"} {
		key, _, _ := strings.Cut(k, " ")
		b.WriteString(mark("key:"+key, sMuted.Render(" "+k)) + "\n")
	}
	return b.String()
}

func (ws *workspace) statusBar(w int) string {
	sep := sRule.Render(strings.Repeat("─", w-2))
	if ws.cmdMode {
		return sep + "\n" + ws.cmdline.View()
	}
	label, _ := nextStep(ws)
	cta := mark("cta:bar", pill("n ▶ "+trunc(label, 40), colOnAcc, colAccent))
	line := cta + "  " + keyHints(ws.views[ws.active].hints(), w-6-ansi.StringWidth(cta))
	if ws.editor != nil {
		line = pill("INSERT", colOnAcc, colOK) + "  " + keyHints("Ctrl-S save · Esc cancel", w-14)
	}
	if t := toast(ws.flash, w-4); t != "" {
		line = " " + t + "\n" + line
	}
	if ws.snap.err != nil {
		line = sErr.Render(trunc("db: "+ws.snap.err.Error(), w-4)) + "\n" + line
	}
	return sep + "\n" + line
}

func helpText() string {
	return section("KEYS", "press any key to close", 70) + `

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
  :automerge on|off       TechLead reviews and squash-merges Tester-approved PRs
  :discord <id>|off       Backend Support answers @mentions in this Discord channel (token: docket discord set-token)
  q                       back to projects · Ctrl-C quit`
}
