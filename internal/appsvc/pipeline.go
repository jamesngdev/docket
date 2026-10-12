// Package appsvc is the UI-agnostic service layer shared by the TUI and the
// desktop app: pipeline counts, the "next step" decision and ticket rules.
package appsvc

import (
	"fmt"

	"docket/internal/agent"
	"docket/internal/store"
)

// Counts summarises project state for the pipeline stepper.
type Counts struct {
	Scanned                                 bool
	Docs, OpenQ, AnsweredQ                  int
	Drafts, Approved, ApprovedNoTickets     int
	Backlog, Active, Done, Total, NeedsUser int
	TechleadTask                            string // running TechLead task, "" if idle
}

// Count derives Counts from active-phase data plus the run list.
func Count(p store.Project, docs int, qs []store.Question, fs []store.Feature, ts []store.Ticket, runs []store.Run) Counts {
	c := Counts{Scanned: p.DocsCommit != "", Docs: docs}
	for _, q := range qs {
		if q.Status == store.QOpen {
			c.OpenQ++
		} else {
			c.AnsweredQ++
		}
	}
	hasTickets := map[int64]bool{}
	for _, t := range ts {
		if t.Status == store.TOutdated {
			continue
		}
		hasTickets[t.FeatureID] = true
		c.Total++
		switch t.Status {
		case store.TBacklog:
			c.Backlog++
		case store.TProcessing, store.TWaitReview:
			c.Active++
		case store.TDone:
			c.Done++
		}
		if t.NeedsUser {
			c.NeedsUser++
		}
	}
	for _, f := range fs {
		switch f.Status {
		case store.FDraft:
			c.Drafts++
		case store.FApproved:
			c.Approved++
			if !hasTickets[f.ID] {
				c.ApprovedNoTickets++
			}
		}
	}
	for _, r := range runs {
		if r.Role == agent.RoleTechLead && r.Status == store.RunRunning {
			c.TechleadTask = r.Task
		}
	}
	return c
}

// Next-step kinds.
const (
	NextView     = "view"     // Arg = view name: questions|features|tickets|agents
	NextTechlead = "techlead" // Arg = TechLead task
	NextRescan   = "rescan"
)

// Next is the single action that unblocks the pipeline.
type Next struct {
	Label, Kind, Arg string
}

// NextStep picks the next action in pipeline order.
func NextStep(c Counts) Next {
	switch {
	case c.TechleadTask != "":
		return Next{"TechLead running (" + c.TechleadTask + ")", NextView, "agents"}
	case !c.Scanned:
		return Next{"scan docs", NextRescan, ""}
	case c.OpenQ > 0:
		return Next{fmt.Sprintf("answer %d questions", c.OpenQ), NextView, "questions"}
	case c.Drafts == 0 && c.Approved == 0:
		return Next{"run TechLead: features", NextTechlead, agent.TaskFeatures}
	case c.Drafts > 0:
		return Next{fmt.Sprintf("approve %d features", c.Drafts), NextView, "features"}
	case c.ApprovedNoTickets > 0:
		return Next{"run TechLead: tickets", NextTechlead, agent.TaskTickets}
	case c.Backlog > 0:
		return Next{fmt.Sprintf("approve %d tickets", c.Backlog), NextView, "tickets"}
	case c.NeedsUser > 0:
		return Next{fmt.Sprintf("%d tickets need you", c.NeedsUser), NextView, "tickets"}
	}
	return Next{"watch agents", NextView, "agents"}
}

// ApproveTicket: backlog → pending; on a flagged ticket clear the flag so the
// daemon resumes (review loop gets a fresh round budget).
func ApproveTicket(db *store.DB, t store.Ticket) error {
	if t.Status == store.TBacklog {
		return db.UpdateTicketFields(t.ID, map[string]any{"status": store.TPending, "needs_user": 0})
	}
	if !t.NeedsUser {
		return nil
	}
	fields := map[string]any{"needs_user": 0}
	if t.ReviewState == store.RError {
		fields["review_state"] = ""
	}
	if t.ReviewState == store.RRequestChanges {
		fields["review_rounds"] = 0
	}
	return db.UpdateTicketFields(t.ID, fields)
}
