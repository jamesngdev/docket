package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"docket/internal/ipc"
	"docket/internal/store"
)

type dashboardView struct{}

func (dashboardView) name() string        { return "Dashboard" }
func (dashboardView) badge(*snapshot) int { return 0 }
func (dashboardView) hints() string       { return "Enter continue · r rescan docs" }

func (dashboardView) keys(ws *workspace, k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "enter":
		_, run := nextStep(ws)
		return run()
	case "r":
		return ws.send(ipc.CmdRescan, "", "docs rescan requested")
	}
	return nil
}

// boardStatuses are the ticket columns shown on the dashboard, with their colours.
var boardStatuses = []struct {
	status string
	color  lipgloss.TerminalColor
}{
	{store.TBacklog, colMuted}, {store.TPending, colWarn}, {store.TProcessing, colAccent},
	{store.TWaitReview, colInfo}, {store.TDone, colOK},
}

func (dashboardView) render(ws *workspace, w, h int) string {
	s := &ws.snap
	c := count(s)
	var b strings.Builder
	b.WriteString(section("PIPELINE", "phase "+orDash(s.project.ActivePhase)+" · switch in Docs or :phase <branch>", w) + "\n\n")
	top, bottom := stepper(c)
	b.WriteString(" " + trunc(top, w-1) + "\n " + trunc(bottom, w-1) + "\n\n")
	label, _ := nextStep(ws)
	b.WriteString(" " + mark("cta:dash", pill(pulse("▶", "▷")+" Enter  "+label, colOnAcc, colAccent)) + "\n\n")

	b.WriteString(section("BOARD", fmt.Sprintf("%d tickets", c.total), w) + "\n")
	b.WriteString(" " + ticketBar(s.tickets, min(w-2, 60)) + "\n ")
	byStatus := map[string]int{}
	for _, t := range s.tickets {
		byStatus[t.Status]++
	}
	var legend []string
	for _, st := range boardStatuses {
		legend = append(legend, mark("board:"+st.status, lipgloss.NewStyle().Foreground(st.color).Render("■")+" "+sMuted.Render(st.status)+" "+sBold.Render(fmt.Sprint(byStatus[st.status]))))
	}
	b.WriteString(trunc(strings.Join(legend, "   "), w-1) + "\n\n")

	b.WriteString(ws.usageSection(w) + "\n")

	b.WriteString(section("ACTIVITY", "", w) + "\n")
	used := strings.Count(b.String(), "\n")
	for i, a := range s.activity {
		if i >= h-used-1 {
			break
		}
		line := sMuted.Render(a.At.Format("15:04")) + "  " + actorStyle(a.Actor).Render(fmt.Sprintf("%-10s", trunc(a.Actor, 10))) + " " + a.Message
		b.WriteString(" " + trunc(line, w-1) + "\n")
	}
	if len(s.activity) == 0 {
		b.WriteString(sMuted.Render(" ◌ no activity yet") + "\n")
	}
	return b.String()
}

// ticketBar is a stacked bar of ticket counts per board status.
func ticketBar(ts []store.Ticket, width int) string {
	n := map[string]int{}
	total := 0
	for _, t := range ts {
		for _, st := range boardStatuses {
			if t.Status == st.status {
				n[st.status]++
				total++
			}
		}
	}
	if total == 0 {
		return sRule.Render(strings.Repeat("━", width))
	}
	// Floor each share, then hand the rounding remainder to the biggest column.
	cells, used, big := map[string]int{}, 0, ""
	for _, st := range boardStatuses {
		cells[st.status] = n[st.status] * width / total
		used += cells[st.status]
		if n[st.status] > n[big] {
			big = st.status
		}
	}
	cells[big] += width - used
	var b strings.Builder
	for _, st := range boardStatuses {
		b.WriteString(lipgloss.NewStyle().Foreground(st.color).Render(strings.Repeat("━", cells[st.status])))
	}
	return b.String()
}
