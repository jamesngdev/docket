package daemon

import (
	"testing"

	"docket/internal/agent"
	"docket/internal/store"
)

func TestGateCheck(t *testing.T) {
	open := []store.Question{{Status: store.QOpen}}
	answered := []store.Question{{Status: store.QAnswered}, {Status: store.QSkipped}}
	draft := []store.Feature{{Status: store.FDraft}, {Status: store.FApproved}}
	approved := []store.Feature{{Status: store.FApproved}, {Status: store.FOutdated}}

	cases := []struct {
		task string
		qs   []store.Question
		fs   []store.Feature
		ok   bool
	}{
		{agent.TaskBootstrap, open, nil, true},
		{agent.TaskFeatures, open, nil, false},
		{agent.TaskFeatures, answered, nil, true},
		{agent.TaskTickets, answered, draft, false},
		{agent.TaskTickets, answered, approved, true},
		{agent.TaskTickets, answered, nil, false},
		{"nope", nil, nil, false},
	}
	for _, c := range cases {
		if err := gateCheck(c.task, c.qs, c.fs); (err == nil) != c.ok {
			t.Errorf("%s: err=%v want ok=%v", c.task, err, c.ok)
		}
	}
}
