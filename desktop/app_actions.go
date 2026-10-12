package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"docket/internal/agent"
	"docket/internal/appsvc"
	"docket/internal/daemon"
	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/store"
	"docket/internal/tmux"
)

// ---- daemon control ----

// Daemon sends a control command: techlead <task> | rescan | phase <branch> |
// stop/restart <run> | redo <ticket> | checkprs | compose | kick.
func (a *App) Daemon(repo, cmd, arg string) error {
	_, p, err := a.db(repo)
	if err != nil {
		return err
	}
	if err := daemon.EnsureRunning(p); err != nil {
		return err
	}
	return daemon.Send(p, cmd, arg)
}

func (a *App) kick(repo string) { _ = a.Daemon(repo, ipc.CmdKick, "") }

// ---- questions ----

func (a *App) AnswerQuestion(repo string, id int64, answer string) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	return db.AnswerQuestion(id, strings.TrimSpace(answer))
}

func (a *App) SkipQuestion(repo string, id int64) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	return db.SkipQuestion(id)
}

// ---- features ----

// SetFeatureStatus: approved | outdated (reject) | draft.
func (a *App) SetFeatureStatus(repo string, id int64, status string) error {
	if status != store.FApproved && status != store.FOutdated && status != store.FDraft {
		return fmt.Errorf("bad feature status %q", status)
	}
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	return db.SetFeatureStatus(id, status)
}

func (a *App) UpdateFeature(repo string, id int64, title, description string) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	return db.UpdateFeature(id, strings.TrimSpace(title), strings.TrimSpace(description))
}

// ---- tickets ----

// ApproveTicket: backlog → pending, or clears a "needs you" flag.
func (a *App) ApproveTicket(repo string, id int64) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	t, err := db.Ticket(id)
	if err != nil {
		return err
	}
	if err := appsvc.ApproveTicket(db, t); err != nil {
		return err
	}
	a.kick(repo)
	return nil
}

// ApproveAllTickets approves every backlog ticket of the active phase.
func (a *App) ApproveAllTickets(repo string) (int, error) {
	db, _, err := a.db(repo)
	if err != nil {
		return 0, err
	}
	ts, err := db.Tickets()
	n := 0
	for _, t := range ts {
		if t.Status == store.TBacklog {
			if err := appsvc.ApproveTicket(db, t); err != nil {
				return n, err
			}
			n++
		}
	}
	a.kick(repo)
	return n, err
}

// UpdateTicket edits title / requirements / acceptance / depends (comma ids).
func (a *App) UpdateTicket(repo string, id int64, field, value string) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	value = strings.TrimSpace(value)
	switch field {
	case "title", "requirements", "acceptance":
	case "depends":
		ids := store.ParseIDs(value)
		for _, d := range ids {
			if dep, err := db.Ticket(d); d == id || err != nil || dep.Status == store.TOutdated {
				return fmt.Errorf("#%d cannot be a dependency (self, missing or outdated)", d)
			}
		}
		field, value = "depends_on", store.JoinIDs(ids)
	default:
		return fmt.Errorf("field %q cannot be edited", field)
	}
	return db.UpdateTicketFields(id, map[string]any{field: value})
}

// OutdateTicket marks a not-started ticket outdated.
func (a *App) OutdateTicket(repo string, id int64) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	t, err := db.Ticket(id)
	if err != nil {
		return err
	}
	if t.Status != store.TBacklog && t.Status != store.TPending {
		return fmt.Errorf("only backlog/pending tickets can be marked outdated (stop the agent first)")
	}
	return db.SetTicketStatus(id, store.TOutdated)
}

// ---- chats & agents (opened in Terminal.app in phase 1) ----

// StartChat launches (or resumes) a chat and returns its tmux session.
// kind: question | ticket | new-ticket.
func (a *App) StartChat(repo, kind string, id int64) (string, error) {
	db, p, err := a.db(repo)
	if err != nil {
		return "", err
	}
	proj, err := db.Project()
	if err != nil {
		return "", err
	}
	var session string
	switch kind {
	case "question":
		session = agent.QuestionSession(proj.Slug, id)
	case "ticket":
		session = agent.TicketSession(proj.Slug, id)
	case "new-ticket":
		session = agent.NewTicketSession(proj.Slug)
	default:
		return "", fmt.Errorf("unknown chat %q", kind)
	}
	if tmux.Has(session) {
		return session, a.OpenInTerminal(session)
	}
	in, err := appsvc.ChatInput(db, p, kind+" chat")
	if err != nil {
		return "", err
	}
	session, err = launchChat(db, p, in, kind, id)
	if err != nil {
		return "", err
	}
	return session, a.OpenInTerminal(session)
}

func launchChat(db *store.DB, p paths.Project, in agent.ChatInput, kind string, id int64) (string, error) {
	switch kind {
	case "question":
		q, err := findQuestion(db, id)
		if err != nil {
			return "", err
		}
		return agent.LaunchQuestionChat(in, q)
	case "ticket":
		t, err := db.Ticket(id)
		if err != nil {
			return "", err
		}
		f, _ := db.Feature(t.FeatureID)
		env, _ := os.ReadFile(p.DevDBEnv())
		return agent.LaunchTicketChat(in, t, f, string(env))
	}
	return agent.LaunchNewTicketChat(in)
}

// EndSession kills a chat/agent tmux session (chats only; use Daemon stop for runs).
func (a *App) EndSession(session string) { tmux.Kill(session) }

// RunLogPath returns the full log file of an agent run.
func (a *App) RunLogPath(repo, runID string) string {
	_, p, _ := a.db(repo)
	return p.RunLog(runID)
}

// DocPath returns the absolute path of a docs file of the active phase.
func (a *App) DocPath(repo, rel string) string {
	_, p, _ := a.db(repo)
	return filepath.Join(p.Docs(), filepath.Clean("/"+rel))
}

// AddThirdDoc stores a link/note third-party document.
func (a *App) AddThirdDoc(repo string, d store.ThirdDoc) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	if strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("title is required")
	}
	return db.AddThirdDoc(d)
}

func (a *App) DeleteThirdDoc(repo string, id int64) error {
	db, _, err := a.db(repo)
	if err != nil {
		return err
	}
	return db.DeleteThirdDoc(id)
}

func findQuestion(db *store.DB, id int64) (store.Question, error) {
	qs, err := db.Questions()
	for _, q := range qs {
		if q.ID == id {
			return q, nil
		}
	}
	if err == nil {
		err = fmt.Errorf("question Q%s not found in the active phase", strconv.FormatInt(id, 10))
	}
	return store.Question{}, err
}
