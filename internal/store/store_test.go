package store

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDispatchable(t *testing.T) {
	all := []Ticket{
		{ID: 1, Status: TDone},
		{ID: 2, Status: TPending, DependsOn: []int64{1}},    // deps done → yes
		{ID: 3, Status: TPending, DependsOn: []int64{1, 4}}, // #4 not done → no
		{ID: 4, Status: TProcessing},                        // not pending
		{ID: 5, Status: TPending, NeedsUser: true},          // flagged → no
		{ID: 6, Status: TBacklog},                           // not approved
		{ID: 7, Status: TPending, DependsOn: []int64{99}},   // missing dep → no
		{ID: 8, Status: TPending, LockedBy: "developer-x"},  // locked → no
		{ID: 9, Status: TPending, Phase: "phase-2"},         // other phase → no
	}
	got := Dispatchable(all, "")
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("want only #2, got %+v", got)
	}
}

func TestParseIDs(t *testing.T) {
	if got := ParseIDs(" 1, #2,x, 3 0"); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Fatalf("got %v", got)
	}
	if JoinIDs([]int64{4, 5}) != "4,5" {
		t.Fatal("JoinIDs")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{"ReelMe Backend!": "reelme-backend", "  ": "x", "Add POST /orders": "add-post-orders"}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "po.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateProject(Project{Name: "Demo", RepoPath: "/tmp/demo", ActivePhase: "phase-1"}); err != nil {
		t.Fatal(err)
	}
	p, err := db.Project()
	if err != nil || p.Slug != "demo" || p.Settings.MaxParallelAgents != 3 || p.Settings.Models["techlead"] != "opus" {
		t.Fatalf("project defaults wrong: %+v %v", p, err)
	}
	qid, _ := db.AddQuestion("Auth?", "docs/a.md")
	if err := db.AnswerQuestion(qid, "JWT"); err != nil {
		t.Fatal(err)
	}
	qs, _ := db.Questions()
	if qs[0].Status != QAnswered || qs[0].Answer != "JWT" {
		t.Fatalf("answer not stored: %+v", qs[0])
	}
	fid, _ := db.AddFeature("Auth", "desc", "docs/a.md")
	t1, _ := db.AddTicket(Ticket{FeatureID: fid, Title: "Schema"})
	t2, _ := db.AddTicket(Ticket{FeatureID: fid, Title: "API", DependsOn: []int64{t1}})
	if err := db.UpdateTicketFields(t2, map[string]any{"status": TProcessing, "locked_by": "dev-1", "bogus": 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTicketStatus(t2, TWaitReview); err != nil {
		t.Fatal(err)
	}
	tk, _ := db.Ticket(t2)
	if tk.Status != TWaitReview || tk.LockedBy != "" || !reflect.DeepEqual(tk.DependsOn, []int64{t1}) {
		t.Fatalf("ticket state wrong: %+v", tk)
	}
	// Switching phase scopes questions/tickets/docs; the old phase keeps its data.
	_ = db.ReplaceDocuments([]Document{{Path: "a.md", Hash: "h1"}})
	_ = db.SetDocsCommit("c1", "")
	if err := db.SetActivePhase("phase-2"); err != nil {
		t.Fatal(err)
	}
	p2, _ := db.Project()
	if p2.ActivePhase != "phase-2" || p2.PrevPhase != "phase-1" || p2.DocsCommit != "" || !db.AnyPhaseScanned() {
		t.Fatalf("phase switch wrong: %+v", p2)
	}
	if qs, _ := db.Questions(); len(qs) != 0 {
		t.Fatal("questions of the previous phase must be hidden")
	}
	if all, _ := db.AllTickets(); len(all) != 2 {
		t.Fatal("AllTickets must span phases")
	}
	if prev, _ := db.DocumentsOf(p2.PrevPhase); len(prev) != 1 || db.PhaseCommit("phase-1") != "c1" {
		t.Fatal("previous phase docs must stay available as diff baseline")
	}
	if err := db.InsertRun(Run{ID: "r1", Role: "developer", Session: "s"}); err != nil {
		t.Fatal(err)
	}
	_ = db.MarkCallback("r1", "ok")
	running, _ := db.RunningRuns()
	if len(running) != 1 || !running[0].Callback || running[0].Summary != "ok" {
		t.Fatalf("run callback not stored: %+v", running)
	}
}

func TestRunSessionsAndMergeCheck(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "d.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	_ = d.InsertRun(Run{ID: "developer-1", Role: "developer", TicketID: 5, Session: "s", ClaudeSession: "aaa"})
	_ = d.InsertRun(Run{ID: "developer-2", Role: "developer", TicketID: 5, Session: "s", ClaudeSession: "bbb"})
	_ = d.InsertRun(Run{ID: "tester-1", Role: "tester", TicketID: 5, Session: "s", ClaudeSession: "ccc"})
	_ = d.MarkCallback("developer-2", "PR")
	_ = d.FinishRun("developer-2", RunDone)
	if got := d.LastClaudeSession("developer", 5); got != "bbb" {
		t.Fatalf("latest clean developer session: got %q", got)
	}
	_ = d.InsertRun(Run{ID: "developer-3", Role: "developer", TicketID: 5, Session: "s", ClaudeSession: "ddd"})
	_ = d.FinishRun("developer-3", RunFailed)
	if got := d.LastClaudeSession("developer", 5); got != "" {
		t.Fatalf("a failed latest run must start fresh, got %q", got)
	}
	if got := d.LastClaudeSession("developer", 6); got != "" {
		t.Fatalf("other ticket: got %q", got)
	}
	if ok, _ := d.UpdateTicketFieldsIfLocked(1, "developer-9", map[string]any{"status": TWaitReview}); ok {
		t.Fatal("no ticket is locked by developer-9")
	}
	if r, _ := d.RunByID("tester-1"); r.ClaudeSession != "ccc" {
		t.Fatalf("session not stored: %+v", r)
	}
	_ = d.CreateProject(Project{Name: "p", RepoPath: "/r"})
	id, _ := d.AddTicket(Ticket{Title: "t", PlannedFiles: "a/", Model: "opus"})
	if err := d.UpdateTicketFields(id, map[string]any{"merge_checked_at": 123, "risk_flags": "x", "gate_sha": "s"}); err != nil {
		t.Fatal(err)
	}
	_ = d.UpdateTicketFields(id, map[string]any{"locked_by": "developer-1"})
	if ok, err := d.UpdateTicketFieldsIfLocked(id, "developer-2", map[string]any{"status": TWaitReview}); ok || err != nil {
		t.Fatalf("a run that lost the lock must not write: %v %v", ok, err)
	}
	if ok, _ := d.UpdateTicketFieldsIfLocked(id, "developer-1", map[string]any{"status": TWaitReview}); !ok {
		t.Fatal("the owning run must write")
	}
	if tk, _ := d.Ticket(id); tk.MergeCheckedAt != 123 || tk.RiskFlags != "x" || tk.GateSHA != "s" || tk.PlannedFiles != "a/" || tk.Model != "opus" {
		t.Fatalf("ticket fields not stored: %+v", tk)
	}
}
