package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/agent"
	"docket/internal/config"
	"docket/internal/paths"
	"docket/internal/store"
	"docket/internal/tmux"
)

type questionsView struct{ cursor int }

func (questionsView) name() string { return "Questions" }
func (questionsView) badge(s *snapshot) int {
	n := 0
	for _, q := range s.questions {
		if q.Status == store.QOpen {
			n++
		}
	}
	return n
}
func (questionsView) hints() string {
	return "Enter/e answer · s skip · c chat with agent about it · X end chat"
}

// chatStartedMsg is sent once a question chat session exists; the TUI then attaches.
type chatStartedMsg struct{ session string }

func (v *questionsView) keys(ws *workspace, k tea.KeyMsg) tea.Cmd {
	qs := ws.snap.questions
	if c, ok := moveCursor(k.String(), v.cursor, len(qs)); ok {
		v.cursor = c
		return nil
	}
	if len(qs) == 0 {
		return nil
	}
	q := qs[clampCursor(v.cursor, len(qs))]
	session := agent.QuestionSession(ws.snap.project.Slug, q.ID)
	switch k.String() {
	case "enter", "e":
		return ws.openEditor(fmt.Sprintf("Answer Q%d · %s", q.ID, trunc(q.Text, 60)), q.Answer, func(s string) error {
			return ws.db.AnswerQuestion(q.ID, strings.TrimSpace(s))
		})
	case "s":
		return ws.dbDo(fmt.Sprintf("Q%d skipped", q.ID), func() error { return ws.db.SkipQuestion(q.ID) })
	case "c":
		if tmux.Has(session) {
			return ws.attach(session) // resume the existing conversation
		}
		return ws.startQuestionChat(q)
	case "X":
		tmux.Kill(session)
		return tea.Batch(flash(fmt.Sprintf("chat about Q%d ended", q.ID), nil), ws.load())
	}
	return nil
}

// chatInput builds the shared context of an interactive chat from a snapshot.
func chatInput(p paths.Project, s snapshot, earlier []string, token, task string) agent.ChatInput {
	return agent.ChatInput{Paths: p, Third: s.third, Token: token,
		Context: agent.TechleadInput{Project: s.project, Task: task, Docs: s.docs,
			PrevCommit: s.project.PrevDocsCommit, DocsHead: s.project.DocsCommit,
			Questions: s.questions, Features: s.features, Tickets: s.tickets, EarlierFeatures: earlier}}
}

// startQuestionChat launches the advisor agent off the UI thread, then attaches.
func (ws *workspace) startQuestionChat(q store.Question) tea.Cmd {
	db, p, s := ws.db, ws.paths, ws.snap
	return tea.Batch(flash(fmt.Sprintf("starting chat about Q%d…", q.ID), nil), func() tea.Msg {
		earlier, _ := db.FeatureTitlesOutsidePhase()
		cfg, _ := config.Load()
		session, err := agent.LaunchQuestionChat(chatInput(p, s, earlier, cfg.OAuthToken, "explain question"), q)
		if err != nil {
			return flashMsg{text: "chat: " + err.Error(), err: true}
		}
		return chatStartedMsg{session: session}
	})
}

func (v *questionsView) render(ws *workspace, w, h int) string {
	qs := ws.snap.questions
	v.cursor = clampCursor(v.cursor, len(qs))
	var rows []string
	for _, q := range qs {
		row := fmt.Sprintf("%s Q%-3d %s", statusIcon(q.Status), q.ID, oneLine(q.Text))
		if ws.snap.sessions[agent.QuestionSession(ws.snap.project.Slug, q.ID)] {
			row = fmt.Sprintf("%s Q%-3d %s %s", statusIcon(q.Status), q.ID, sAccent.Render("💬"), oneLine(q.Text))
		}
		rows = append(rows, row)
	}
	detail := ""
	if len(qs) > 0 {
		detail = questionDetail(qs[v.cursor], ws.snap.sessions[agent.QuestionSession(ws.snap.project.Slug, qs[v.cursor].ID)])
	}
	header := fmt.Sprintf("QUESTIONS   %d open · %d total", v.badge(&ws.snap), len(qs))
	return listDetail(header, rows, v.cursor, w, h, detail)
}

var (
	// optionRe finds inline options like "(A)", "A)", "(1)" that deserve their own line.
	optionRe = regexp.MustCompile(`\s*(\([A-Za-z0-9]\)|\b[A-D]\))\s+`)
	// recommendRe finds a trailing recommendation sentence.
	recommendRe = regexp.MustCompile(`[.;]?\s*(Đề xuất|Recommend(?:ation|ed)?|Suggest(?:ion|ed)?)\b\s*:?\s*`)
)

// questionDetail renders a question with source items and options on separate lines.
func questionDetail(q store.Question, chatting bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s %s", sBold.Render(fmt.Sprintf("Q%d", q.ID)), statusIcon(q.Status), q.Status)
	if chatting {
		b.WriteString(sAccent.Render("   💬 chat running — c to reopen, X to end"))
	}
	b.WriteString("\n\n" + sBold.Render("NGUỒN / SOURCE") + "\n")
	for _, src := range strings.Split(q.Source, ";") {
		if src = strings.TrimSpace(src); src != "" {
			b.WriteString(sMuted.Render("  • "+src) + "\n")
		}
	}
	b.WriteString("\n" + sBold.Render("CÂU HỎI / QUESTION") + "\n")
	text := optionRe.ReplaceAllString(q.Text, "\n  $1 ")
	text = recommendRe.ReplaceAllString(text, "\n\n"+sAccent.Render("→ $1:")+" ")
	b.WriteString(text + "\n")
	b.WriteString("\n" + sBold.Render("TRẢ LỜI / ANSWER") + "\n")
	if strings.TrimSpace(q.Answer) == "" {
		b.WriteString(sMuted.Render("(chưa trả lời — Enter để trả lời, c để hỏi agent giải thích thêm)"))
	} else {
		b.WriteString(q.Answer)
	}
	return b.String()
}

// oneLine collapses whitespace so list rows stay on a single line.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
