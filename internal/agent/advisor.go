package agent

import (
	"fmt"
	"strings"

	"docket/internal/extskills"
	"docket/internal/paths"
	"docket/internal/store"
)

// Interactive chats are untracked agents the user talks to in tmux: they are
// not runs, so the daemon never reaps, dispatches or counts them.

// RoleAdvisor explains one question (Questions view, key c).
const RoleAdvisor = "advisor"

// chat describes one interactive session.
type chat struct {
	paths  paths.Project
	role   string // decides the isolated home + skills
	id     string // context dir / DOCKET_RUN / session suffix
	files  map[string]string
	prompt string
	ctx    TechleadInput
	third  []store.ThirdDoc
	token  string
	extra  []extskills.Skill
}

func launchChat(c chat) (string, error) {
	ctxDir := c.paths.RunContext(c.id)
	c.files["PROJECT.md"] = ProjectMarkdown(c.ctx)
	c.files["3TH_DOCUMENT.md"] = ThirdDocsMarkdown(c.third)
	if err := WriteFiles(ctxDir, c.files); err != nil {
		return "", err
	}
	p := c.ctx.Project
	return Launch(Spawn{
		Paths: c.paths, Slug: p.Slug, Role: c.role, Model: p.Settings.Models[RoleTechLead],
		RunID: c.id, Workdir: p.RepoPath, Prompt: strings.ReplaceAll(c.prompt, "{ctx}", ctxDir), Token: c.token,
		Extra: c.extra,
	})
}

// ChatInput is the shared input of every chat.
type ChatInput struct {
	Paths   paths.Project
	Context TechleadInput // rendered into PROJECT.md
	Third   []store.ThirdDoc
	Token   string
}

// QuestionChatID names the context dir / run id of a question chat.
func QuestionChatID(qid int64) string { return fmt.Sprintf("q%d", qid) }

// QuestionSession is the tmux session of a question chat.
func QuestionSession(slug string, qid int64) string {
	return SessionName(slug, RoleAdvisor, 0, QuestionChatID(qid))
}

// LaunchQuestionChat starts the Advisor explaining question q.
func LaunchQuestionChat(in ChatInput, q store.Question) (string, error) {
	question := fmt.Sprintf("# Question Q%d (%s)\n\n## Source\n%s\n\n## Question\n%s\n\n## Current answer\n%s\n",
		q.ID, q.Status, q.Source, q.Text, orNone(q.Answer))
	return launchChat(chat{paths: in.Paths, role: RoleAdvisor, id: QuestionChatID(q.ID), ctx: in.Context,
		third: in.Third, token: in.Token, files: map[string]string{"QUESTION.md": question},
		prompt: fmt.Sprintf(`The project owner wants to understand question Q%d before answering it.
Read {ctx}/QUESTION.md first, then the relevant parts of {ctx}/PROJECT.md and the cited docs in $DOCKET_DATA/docs/.
Use skill explain-question. Start with the structured explanation, then wait for follow-up questions.`, q.ID)})
}

// TicketChatID names a ticket chat; the session becomes docket-<slug>-techlead-chat-t<id>.
func TicketChatID(tid int64) string { return fmt.Sprintf("chat-t%d", tid) }

// TicketSession is the tmux session of a ticket chat.
func TicketSession(slug string, tid int64) string {
	return SessionName(slug, RoleTechLead, 0, TicketChatID(tid))
}

// LaunchTicketChat starts TechLead in chat mode for ticket t.
func LaunchTicketChat(in ChatInput, t store.Ticket, f store.Feature, devEnv string) (string, error) {
	ticket := TicketMarkdown(TicketInput{Project: in.Context.Project, Ticket: t, Feature: f, DevEnv: devEnv})
	ticket = fmt.Sprintf("Status: %s · depends on: %s · PR: %s\n\n", t.Status, orNone(store.JoinIDs(t.DependsOn)), orNone(t.PRURL)) + ticket
	skills, _ := extskills.Load() // same team skills as TechLead runs (shared home)
	return launchChat(chat{paths: in.Paths, role: RoleTechLead, id: TicketChatID(t.ID), ctx: in.Context,
		third: in.Third, token: in.Token, extra: skills, files: map[string]string{"TICKET.md": ticket},
		prompt: fmt.Sprintf(`You are the TechLead, chatting live with the project owner about ticket #%d.
Read {ctx}/TICKET.md and {ctx}/PROJECT.md first. Use skill ticket-chat.
Greet briefly with a 3-line summary of the ticket (status, goal, related tickets) and ask what they want to change.`, t.ID)})
}

// NewTicketChatID is the single "create ticket" chat of a project.
const NewTicketChatID = "chat-new"

// NewTicketSession is the tmux session of the "create ticket" chat.
func NewTicketSession(slug string) string { return SessionName(slug, RoleTechLead, 0, NewTicketChatID) }

// LaunchNewTicketChat starts TechLead in chat mode to turn a user request into tickets.
func LaunchNewTicketChat(in ChatInput) (string, error) {
	skills, _ := extskills.Load() // same team skills as TechLead runs (shared home)
	return launchChat(chat{paths: in.Paths, role: RoleTechLead, id: NewTicketChatID, ctx: in.Context,
		third: in.Third, token: in.Token, extra: skills, files: map[string]string{},
		prompt: `You are the TechLead, chatting live with the project owner who wants NEW work done.
Read {ctx}/PROJECT.md first (phase, features, existing tickets). Use skill create-ticket.
Greet in one line and ask what they need; then clarify only what is essential and create the ticket(s).`})
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "_none_"
	}
	return s
}
