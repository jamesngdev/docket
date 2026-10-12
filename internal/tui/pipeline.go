package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/agent"
	"docket/internal/ipc"
	"docket/internal/store"
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

// counts summarises the snapshot for the pipeline stepper and next action.
type counts struct {
	scanned                              bool
	docs, openQ, answeredQ               int
	drafts, approved, approvedNoTickets  int
	backlog, active, done, total, needUs int
	techleadTask                         string // running TechLead task, "" if idle
}

func count(s *snapshot) counts {
	c := counts{scanned: s.project.DocsCommit != "", docs: len(s.docs)}
	for _, q := range s.questions {
		if q.Status == store.QOpen {
			c.openQ++
		} else {
			c.answeredQ++
		}
	}
	hasTickets := map[int64]bool{}
	for _, t := range s.tickets {
		if t.Status == store.TOutdated {
			continue
		}
		hasTickets[t.FeatureID] = true
		c.total++
		switch t.Status {
		case store.TBacklog:
			c.backlog++
		case store.TProcessing, store.TWaitReview:
			c.active++
		case store.TDone:
			c.done++
		}
		if t.NeedsUser {
			c.needUs++
		}
	}
	for _, f := range s.features {
		switch f.Status {
		case store.FDraft:
			c.drafts++
		case store.FApproved:
			c.approved++
			if !hasTickets[f.ID] {
				c.approvedNoTickets++
			}
		}
	}
	for _, r := range s.runs {
		if r.Role == agent.RoleTechLead && r.Status == store.RunRunning {
			c.techleadTask = r.Task
		}
	}
	return c
}

// nextStep returns the label and action that unblock the pipeline.
func nextStep(ws *workspace) (string, func() tea.Cmd) {
	c := count(&ws.snap)
	goTo := func(v int) func() tea.Cmd {
		return func() tea.Cmd { ws.active = v; return ws.load() }
	}
	techlead := func(task string) func() tea.Cmd {
		return func() tea.Cmd { return ws.send(ipc.CmdTechlead, task, "TechLead "+task+" started") }
	}
	switch {
	case c.techleadTask != "":
		return "TechLead running (" + c.techleadTask + ")", goTo(vAgents)
	case !c.scanned:
		return "scan docs", func() tea.Cmd { return ws.send(ipc.CmdRescan, "", "docs scan started") }
	case c.openQ > 0:
		return fmt.Sprintf("answer %d questions", c.openQ), goTo(vQuestions)
	case c.drafts == 0 && c.approved == 0:
		return "run TechLead: features", techlead(agent.TaskFeatures)
	case c.drafts > 0:
		return fmt.Sprintf("approve %d features", c.drafts), goTo(vFeatures)
	case c.approvedNoTickets > 0:
		return "run TechLead: tickets", techlead(agent.TaskTickets)
	case c.backlog > 0:
		return fmt.Sprintf("approve %d tickets", c.backlog), goTo(vTickets)
	case c.needUs > 0:
		return fmt.Sprintf("%d tickets need you", c.needUs), goTo(vTickets)
	}
	return "watch agents", goTo(vAgents)
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
	tl := c.techleadTask
	steps := []st{
		{"Scan", mark(c.scanned, !c.scanned, false), fmt.Sprintf("%d docs", c.docs)},
		{"Questions", mark(c.scanned && c.openQ == 0, c.openQ > 0, tl == agent.TaskBootstrap || tl == agent.TaskDocDiff),
			fmt.Sprintf("%d/%d answered", c.answeredQ, c.answeredQ+c.openQ)},
		{"Features", mark(c.approved > 0 && c.drafts == 0, c.drafts > 0, tl == agent.TaskFeatures),
			fmt.Sprintf("%d to approve", c.drafts)},
		{"Tickets", mark(c.total > 0 && c.backlog == 0, c.backlog > 0, tl == agent.TaskTickets),
			fmt.Sprintf("%d to approve", c.backlog)},
		{"Run", mark(c.total > 0 && c.done == c.total, c.needUs > 0, c.active > 0),
			fmt.Sprintf("%d/%d done", c.done, c.total)},
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
