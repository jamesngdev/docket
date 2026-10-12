package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/agent"
	"docket/internal/config"
	"docket/internal/store"
	"docket/internal/tmux"
)

// ticketChat opens (or resumes) the TechLead chat about ticket t.
func (ws *workspace) ticketChat(t store.Ticket) tea.Cmd {
	session := agent.TicketSession(ws.snap.project.Slug, t.ID)
	if tmux.Has(session) {
		return ws.attach(session)
	}
	db, p, s := ws.db, ws.paths, ws.snap
	return tea.Batch(flash(fmt.Sprintf("starting TechLead chat about #%d…", t.ID), nil), func() tea.Msg {
		earlier, _ := db.FeatureTitlesOutsidePhase()
		cfg, _ := config.Load()
		f, _ := db.Feature(t.FeatureID)
		env, _ := os.ReadFile(p.DevDBEnv())
		session, err := agent.LaunchTicketChat(chatInput(p, s, earlier, cfg.OAuthToken, "ticket chat"), t, f, string(env))
		if err != nil {
			return flashMsg{text: "chat: " + err.Error(), err: true}
		}
		return chatStartedMsg{session: session}
	})
}

// newTicketChat opens (or resumes) the TechLead chat that creates tickets from a request.
func (ws *workspace) newTicketChat() tea.Cmd {
	session := agent.NewTicketSession(ws.snap.project.Slug)
	if tmux.Has(session) {
		return ws.attach(session)
	}
	db, p, s := ws.db, ws.paths, ws.snap
	return tea.Batch(flash("starting TechLead to create a ticket…", nil), func() tea.Msg {
		earlier, _ := db.FeatureTitlesOutsidePhase()
		cfg, _ := config.Load()
		session, err := agent.LaunchNewTicketChat(chatInput(p, s, earlier, cfg.OAuthToken, "create ticket from user request"))
		if err != nil {
			return flashMsg{text: "chat: " + err.Error(), err: true}
		}
		return chatStartedMsg{session: session}
	})
}

// endTicketChat kills the chat session of ticket t.
func (ws *workspace) endTicketChat(t store.Ticket) tea.Cmd {
	tmux.Kill(agent.TicketSession(ws.snap.project.Slug, t.ID))
	return tea.Batch(flash(fmt.Sprintf("chat about #%d ended", t.ID), nil), ws.load())
}
