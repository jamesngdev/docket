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

func TestFastMergeable(t *testing.T) {
	s := store.DefaultSettings()
	low := store.Ticket{GateSHA: "abc", RiskFlags: ""}
	if !fastMergeable(s, low, "abc", "pass") {
		t.Fatal("gate passed on head, no risks, CI green: must merge directly")
	}
	for name, c := range map[string]struct {
		s      store.Settings
		t      store.Ticket
		head   string
		checks string
	}{
		"risk flags":        {s, store.Ticket{GateSHA: "abc", RiskFlags: "migration: m.sql"}, "abc", "pass"},
		"pushed after gate": {s, low, "def", "pass"},
		"no gate":           {s, store.Ticket{}, "", "pass"},
		"CI failed":         {s, low, "abc", "fail"},
		"review all":        {store.Settings{MergeReview: store.MergeReviewAll}, low, "abc", "pass"},
	} {
		if fastMergeable(c.s, c.t, c.head, c.checks) {
			t.Errorf("%s: must go to TechLead", name)
		}
	}
}

func TestMergeMarkdownShowsRiskFlags(t *testing.T) {
	md := mergeMarkdown(store.Project{Settings: store.DefaultSettings()}, []store.Ticket{
		{ID: 3, Title: "Drop column", Status: store.TWaitReview, PRURL: "u", ReviewState: store.RApprove, RiskFlags: "migration: m.sql"},
	})
	if !strings.Contains(md, "Risk flags (focus your review here): migration: m.sql") {
		t.Errorf("risk flags missing:\n%s", md)
	}
}

func TestResumable(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir + "/d.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.CreateProject(store.Project{Name: "p", RepoPath: dir})
	id, _ := db.AddTicket(store.Ticket{Title: "t"})
	d := &Daemon{db: db}
	if d.resumable(agent.RoleDeveloper, agent.TaskImplement, id) || d.resumable(agent.RoleTester, "review", id) {
		t.Fatal("first runs start fresh")
	}
	if !d.resumable(agent.RoleDeveloper, agent.TaskResolveConflicts, id) {
		t.Fatal("conflict fix continues the Developer session")
	}
	_ = db.UpdateTicketFields(id, map[string]any{"review_state": store.RRequestChanges, "review_summary": "fix x", "review_rounds": 1})
	if !d.resumable(agent.RoleDeveloper, agent.TaskImplement, id) || !d.resumable(agent.RoleTester, "review", id) {
		t.Fatal("a review round continues both sessions")
	}
}
