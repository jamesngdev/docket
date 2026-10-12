package tui

import (
	"strings"
	"testing"

	"docket/internal/agent"
	"docket/internal/store"
)

func TestNextStep(t *testing.T) {
	ws := &workspace{}
	label := func() string { l, _ := nextStep(ws); return l }

	if l := label(); l != "scan docs" {
		t.Fatalf("fresh project: %q", l)
	}
	ws.snap.project.DocsCommit = "abc"
	ws.snap.runs = []store.Run{{Role: agent.RoleTechLead, Status: store.RunRunning, Task: agent.TaskBootstrap}}
	if l := label(); !strings.HasPrefix(l, "TechLead running") {
		t.Fatalf("techlead running: %q", l)
	}
	ws.snap.runs = nil
	ws.snap.questions = []store.Question{{Status: store.QOpen}}
	if l := label(); l != "answer 1 questions" {
		t.Fatalf("open questions: %q", l)
	}
	ws.snap.questions[0].Status = store.QAnswered
	if l := label(); l != "run TechLead: features" {
		t.Fatalf("answered: %q", l)
	}
	ws.snap.features = []store.Feature{{ID: 1, Status: store.FDraft}}
	if l := label(); l != "approve 1 features" {
		t.Fatalf("draft: %q", l)
	}
	ws.snap.features[0].Status = store.FApproved
	if l := label(); l != "run TechLead: tickets" {
		t.Fatalf("approved without tickets: %q", l)
	}
	ws.snap.tickets = []store.Ticket{{ID: 1, FeatureID: 1, Status: store.TBacklog}}
	if l := label(); l != "approve 1 tickets" {
		t.Fatalf("backlog: %q", l)
	}
	ws.snap.tickets[0].Status = store.TPending
	if l := label(); l != "watch agents" {
		t.Fatalf("running: %q", l)
	}
}
