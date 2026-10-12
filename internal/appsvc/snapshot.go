package appsvc

import (
	"strings"

	"docket/internal/agent"
	"docket/internal/config"
	"docket/internal/daemon"
	"docket/internal/gitops"
	"docket/internal/paths"
	"docket/internal/store"
	"docket/internal/tmux"
)

// Snapshot is everything a UI renders for one project (active phase).
type Snapshot struct {
	Project   store.Project
	Questions []store.Question
	Features  []store.Feature
	Tickets   []store.Ticket
	Runs      []store.Run
	Docs      []store.Document
	Third     []store.ThirdDoc
	Activity  []store.Activity
	Phases    []string // docs branches
	Chats     []string // live chat tmux sessions of this project
	DaemonUp  bool
	Counts    Counts
	Next      Next
}

// Load reads a fresh snapshot. withPhases also lists docs branches (a git call).
func Load(db *store.DB, p paths.Project, withPhases bool) (Snapshot, error) {
	var s Snapshot
	var err error
	if s.Project, err = db.Project(); err != nil {
		return s, err
	}
	s.Questions, _ = db.Questions()
	s.Features, _ = db.Features()
	s.Tickets, _ = db.Tickets()
	s.Runs, _ = db.Runs(80)
	s.Docs, _ = db.Documents()
	s.Third, _ = db.ThirdDocs()
	s.Activity, _ = db.RecentActivity(30)
	s.DaemonUp = daemon.Alive(p)
	if withPhases {
		s.Phases, _ = gitops.Branches(p.Docs())
	}
	prefix := "docket-" + s.Project.Slug + "-"
	for name := range tmux.List() {
		if strings.HasPrefix(name, prefix) && (strings.Contains(name, "-chat-") || strings.Contains(name, "-advisor-")) {
			s.Chats = append(s.Chats, name)
		}
	}
	s.Counts = Count(s.Project, len(s.Docs), s.Questions, s.Features, s.Tickets, s.Runs)
	s.Next = NextStep(s.Counts)
	return s, nil
}

// ChatInput builds the shared context handed to an interactive chat agent.
func ChatInput(db *store.DB, p paths.Project, task string) (agent.ChatInput, error) {
	s, err := Load(db, p, false)
	if err != nil {
		return agent.ChatInput{}, err
	}
	earlier, _ := db.FeatureTitlesOutsidePhase()
	cfg, _ := config.Load()
	return agent.ChatInput{Paths: p, Third: s.Third, Token: cfg.OAuthToken,
		Context: agent.TechleadInput{Project: s.Project, Task: task, Docs: s.Docs,
			PrevCommit: s.Project.PrevDocsCommit, DocsHead: s.Project.DocsCommit,
			Questions: s.Questions, Features: s.Features, Tickets: s.Tickets, EarlierFeatures: earlier}}, nil
}
