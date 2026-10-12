package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/agent"
	"docket/internal/appsvc"
	"docket/internal/ipc"
)

// View indexes in workspace.views.
const (
	vDashboard = iota
	vQuestions
	vFeatures
	vTickets
	vAgents
	vDocs
)

// counts is the shared pipeline summary.
type counts = appsvc.Counts

func count(s *snapshot) counts {
	return appsvc.Count(s.project, len(s.docs), s.questions, s.features, s.tickets, s.runs)
}

// nextStep returns the label and action that unblock the pipeline.
func nextStep(ws *workspace) (string, func() tea.Cmd) {
	n := appsvc.NextStep(count(&ws.snap))
	views := map[string]int{"questions": vQuestions, "features": vFeatures, "tickets": vTickets, "agents": vAgents}
	switch n.Kind {
	case appsvc.NextRescan:
		return n.Label, func() tea.Cmd { return ws.send(ipc.CmdRescan, "", "docs scan started") }
	case appsvc.NextTechlead:
		return n.Label, func() tea.Cmd { return ws.send(ipc.CmdTechlead, n.Arg, "TechLead "+n.Arg+" started") }
	}
	return n.Label, func() tea.Cmd { ws.active = views[n.Arg]; return ws.load() }
}

// stepper renders ✓ Scan ── ● Questions ── … with a detail line.
func stepper(c counts) (string, string) {
	type st struct {
		name, mark, detail string
	}
	mark := func(done, current, running bool) string {
		switch {
		case running:
			return sAccent.Render("◐")
		case current:
			return sWarn.Render("●")
		case done:
			return sOK.Render("✓")
		}
		return sMuted.Render("○")
	}
	tl := c.TechleadTask
	steps := []st{
		{"Scan", mark(c.Scanned, !c.Scanned, false), fmt.Sprintf("%d docs", c.Docs)},
		{"Questions", mark(c.Scanned && c.OpenQ == 0, c.OpenQ > 0, tl == agent.TaskBootstrap || tl == agent.TaskDocDiff),
			fmt.Sprintf("%d/%d answered", c.AnsweredQ, c.AnsweredQ+c.OpenQ)},
		{"Features", mark(c.Approved > 0 && c.Drafts == 0, c.Drafts > 0, tl == agent.TaskFeatures),
			fmt.Sprintf("%d to approve", c.Drafts)},
		{"Tickets", mark(c.Total > 0 && c.Backlog == 0, c.Backlog > 0, tl == agent.TaskTickets),
			fmt.Sprintf("%d to approve", c.Backlog)},
		{"Run", mark(c.Total > 0 && c.Done == c.Total, c.NeedsUser > 0, c.Active > 0),
			fmt.Sprintf("%d/%d done", c.Done, c.Total)},
	}
	var top, bottom string
	for i, s := range steps {
		if i > 0 {
			top += sMuted.Render(" ── ")
			bottom += "    "
		}
		top += s.mark + " " + s.name
		bottom += padRight(s.detail, len(s.name)+2)
	}
	return top, sMuted.Render(bottom)
}
