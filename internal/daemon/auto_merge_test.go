package daemon

import (
	"strings"
	"testing"

	"docket/internal/agent"
	"docket/internal/store"
)

func TestMergeReady(t *testing.T) {
	ok := store.Ticket{ID: 1, Status: store.TWaitReview, PRURL: "https://x/pull/1", ReviewState: store.RApprove}
	if !mergeReady(ok) {
		t.Fatal("approved, unlocked, mergeable ticket must be ready")
	}
	for name, mut := range map[string]func(*store.Ticket){
		"not approved":  func(t *store.Ticket) { t.ReviewState = store.RRequestChanges },
		"no review yet": func(t *store.Ticket) { t.ReviewState = "" },
		"agent running": func(t *store.Ticket) { t.LockedBy = "developer-x" },
		"flagged":       func(t *store.Ticket) { t.NeedsUser = true },
		"conflict":      func(t *store.Ticket) { t.PRConflict = true },
		"no PR":         func(t *store.Ticket) { t.PRURL = "" },
		"already done":  func(t *store.Ticket) { t.Status = store.TDone },
	} {
		tk := ok
		mut(&tk)
		if mergeReady(tk) {
			t.Errorf("%s: must not be ready", name)
		}
	}
}

func TestMergeMarkdownListsOnlyReadyTickets(t *testing.T) {
	p := store.Project{Settings: store.DefaultSettings()}
	md := mergeMarkdown(p, []store.Ticket{
		{ID: 1, Title: "Login API", Status: store.TWaitReview, PRURL: "https://x/pull/1", ReviewState: store.RApprove, Acceptance: "returns JWT"},
		{ID: 2, Title: "Refresh", Status: store.TWaitReview, PRURL: "https://x/pull/2", ReviewState: store.RRequestChanges},
	})
	if !strings.Contains(md, "Ticket #1: Login API") || !strings.Contains(md, "returns JWT") {
		t.Errorf("ready ticket missing:\n%s", md)
	}
	if strings.Contains(md, "#2") {
		t.Errorf("unapproved ticket listed:\n%s", md)
	}
	if gateCheck(agent.TaskMerge, nil, nil) != nil {
		t.Error("merge task must not be gated")
	}
}
