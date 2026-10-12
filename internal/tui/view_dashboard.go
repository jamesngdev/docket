package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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

func (dashboardView) render(ws *workspace, w, h int) string {
	s := &ws.snap
	c := count(s)
	var b strings.Builder
	b.WriteString(sTitle.Render("PIPELINE") + sMuted.Render("  phase ") + sAccent.Render(s.project.ActivePhase) +
		sMuted.Render("  (switch in Docs view or :phase <branch>)") + "\n\n")
	top, bottom := stepper(c)
	b.WriteString(" " + trunc(top, w-1) + "\n " + trunc(bottom, w-1) + "\n\n")
	label, _ := nextStep(ws)
	b.WriteString(" " + sAccent.Render("▶ Enter: "+label) + "\n\n")

	b.WriteString(sTitle.Render("BOARD") + "\n")
	byStatus := map[string]int{}
	for _, t := range s.tickets {
		byStatus[t.Status]++
	}
	var parts []string
	for _, st := range []string{store.TBacklog, store.TPending, store.TProcessing, store.TWaitReview, store.TDone} {
		parts = append(parts, fmt.Sprintf("%s %s %d", statusIcon(st), st, byStatus[st]))
	}
	b.WriteString(" " + trunc(strings.Join(parts, "   "), w-1) + "\n\n")

	b.WriteString(sTitle.Render("ACTIVITY") + "\n")
	used := strings.Count(b.String(), "\n")
	for i, a := range s.activity {
		if i >= h-used-1 {
			break
		}
		line := fmt.Sprintf("%s  %-10s %s", a.At.Format("15:04"), trunc(a.Actor, 10), a.Message)
		b.WriteString(" " + trunc(line, w-1) + "\n")
	}
	if len(s.activity) == 0 {
		b.WriteString(sMuted.Render(" no activity yet") + "\n")
	}
	return b.String()
}
