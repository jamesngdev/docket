package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/ipc"
	"docket/internal/store"
)

type agentsView struct{ cursor int }

func (agentsView) name() string { return "Agents" }
func (agentsView) badge(s *snapshot) int {
	n := 0
	for _, r := range s.runs {
		if r.Status == store.RunRunning {
			n++
		}
	}
	return n
}
func (agentsView) hints() string {
	return "Enter attach (tmux, Ctrl-b d to return) · x stop · r restart · L log"
}

// selectedSession is the running session whose pane is previewed.
func (v *agentsView) selectedSession(s *snapshot, active bool) string {
	if !active || len(s.runs) == 0 {
		return ""
	}
	r := s.runs[clampCursor(v.cursor, len(s.runs))]
	if r.Status != store.RunRunning {
		return ""
	}
	return r.Session
}

func (v *agentsView) keys(ws *workspace, k tea.KeyMsg) tea.Cmd {
	runs := ws.snap.runs
	if c, ok := moveCursor(k.String(), v.cursor, len(runs)); ok {
		v.cursor = c
		return ws.load()
	}
	if len(runs) == 0 {
		return nil
	}
	r := runs[clampCursor(v.cursor, len(runs))]
	switch k.String() {
	case "enter":
		return ws.attach(r.Session)
	case "x":
		return ws.send(ipc.CmdStop, r.ID, "stopping "+r.ID)
	case "r":
		return ws.send(ipc.CmdRestart, r.ID, "restarting "+r.ID)
	case "L":
		return ws.pager(ws.paths.RunLog(r.ID))
	}
	return nil
}

func (v *agentsView) render(ws *workspace, w, h int) string {
	runs := ws.snap.runs
	v.cursor = clampCursor(v.cursor, len(runs))
	var rows []string
	for _, r := range runs {
		what := r.Task
		if r.TicketID > 0 {
			what = fmt.Sprintf("#%d", r.TicketID)
			if t, ok := ws.ticketByID(r.TicketID); ok {
				what += " " + t.Title
			}
		}
		end := time.Now()
		if !r.EndedAt.IsZero() {
			end = r.EndedAt
		}
		dur := end.Sub(r.StartedAt).Round(time.Minute)
		rows = append(rows, fmt.Sprintf("%s %-22s %-10s %-30s %6s  %s", statusIcon(r.Status), r.ID, r.Role, trunc(what, 30),
			fmtDur(dur), sMuted.Render(r.Session+" "+r.Summary)))
	}
	detail := ""
	if len(runs) > 0 {
		r := runs[v.cursor]
		detail = fmt.Sprintf("%s · %s · %s\nworkdir %s\n", r.ID, r.Role, r.Status, r.Workdir)
		if ws.snap.pane != "" {
			detail += "\n" + sMuted.Render("last lines of pane:") + "\n" + ws.snap.pane
		}
	}
	return listDetail(fmt.Sprintf("AGENTS   running %d · history %d", v.badge(&ws.snap), len(runs)), rows, v.cursor, w, h, detail)
}

func fmtDur(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}
