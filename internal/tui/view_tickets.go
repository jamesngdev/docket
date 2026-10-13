package tui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/agent"
	"docket/internal/ipc"
	"docket/internal/store"
)

var ticketFilters = []string{"all", store.TBacklog, store.TPending, store.TProcessing, store.TWaitReview, store.TDone, store.TOutdated}

type ticketsView struct {
	listCursor
	filter int
}

func (ticketsView) name() string { return "Tickets" }
func (ticketsView) badge(s *snapshot) int {
	n := 0
	for _, t := range s.tickets {
		if t.Status == store.TBacklog || t.NeedsUser {
			n++
		}
	}
	return n
}
func (ticketsView) hints() string {
	return "N new ticket (TechLead) · c chat with TechLead · X end chat · h/l filter · a approve · A all · e req · C acceptance · t title · d deps · x stop/outdated · Enter attach · o PR"
}

func (v *ticketsView) visible(s *snapshot) []store.Ticket {
	var out []store.Ticket
	for _, t := range s.tickets {
		if v.filter == 0 && t.Status != store.TOutdated || ticketFilters[v.filter] == t.Status {
			out = append(out, t)
		}
	}
	return out
}

func (v *ticketsView) keys(ws *workspace, k tea.KeyMsg) tea.Cmd {
	key := k.String()
	switch key {
	case "h", "left":
		return v.setFilter(ws, (v.filter+len(ticketFilters)-1)%len(ticketFilters))
	case "l", "right":
		return v.setFilter(ws, (v.filter+1)%len(ticketFilters))
	}
	ts := v.visible(&ws.snap)
	if c, ok := moveCursor(key, v.cursor, len(ts)); ok {
		v.cursor = c
		return nil
	}
	if key == "N" {
		return ws.newTicketChat()
	}
	if key == "A" {
		return ws.dbDo("all backlog tickets approved", func() error {
			for _, t := range ws.snap.tickets {
				if t.Status == store.TBacklog {
					if err := ws.db.SetTicketStatus(t.ID, store.TPending); err != nil {
						return err
					}
				}
			}
			return nil
		})
	}
	if len(ts) == 0 {
		return nil
	}
	t := ts[clampCursor(v.cursor, len(ts))]
	id := t.ID
	edit := func(label, col, cur string) tea.Cmd {
		return ws.openEditor(fmt.Sprintf("#%d %s", id, label), cur, func(s string) error {
			val := strings.TrimSpace(s)
			if col == "depends_on" {
				ids := store.ParseIDs(val)
				for _, d := range ids {
					dep, ok := ws.ticketByID(d)
					if d == id || !ok || dep.Status == store.TOutdated {
						return fmt.Errorf("#%d cannot be a dependency (self, missing or outdated)", d)
					}
				}
				val = store.JoinIDs(ids)
			}
			return ws.db.UpdateTicketFields(id, map[string]any{col: val})
		})
	}
	switch key {
	case "a":
		return ws.dbDo(fmt.Sprintf("#%d approved", id), func() error { return approveTicket(ws.db, t) })
	case "e":
		return edit("requirements", "requirements", t.Requirements)
	case "C":
		return edit("acceptance criteria", "acceptance", t.Acceptance)
	case "t":
		return edit("title", "title", t.Title)
	case "d":
		return edit("depends on (comma-separated ticket ids)", "depends_on", store.JoinIDs(t.DependsOn))
	case "x":
		if t.LockedBy != "" {
			return ws.send(ipc.CmdStop, t.LockedBy, fmt.Sprintf("stopping agent on #%d", id))
		}
		if t.Status == store.TBacklog || t.Status == store.TPending {
			return ws.dbDo(fmt.Sprintf("#%d outdated", id), func() error { return ws.db.SetTicketStatus(id, store.TOutdated) })
		}
	case "enter":
		if r, ok := ws.runByID(t.LockedBy); ok {
			return ws.attach(r.Session)
		}
		return flash("attach", errNoSession)
	case "c":
		return ws.ticketChat(t)
	case "X":
		return ws.endTicketChat(t)
	case "o":
		if t.PRURL != "" {
			return tea.ExecProcess(exec.Command("open", t.PRURL), func(error) tea.Msg { return nil })
		}
	}
	return nil
}

// setFilter switches the status filter (keys h/l or a click on a tab).
func (v *ticketsView) setFilter(ws *workspace, i int) tea.Cmd {
	if i < 0 || i >= len(ticketFilters) {
		return nil
	}
	v.filter = i
	if ticketFilters[i] == store.TWaitReview { // looking at PRs: refresh merge/conflict state now
		return ws.send(ipc.CmdCheckPRs, "", "checking PRs for merges and conflicts…")
	}
	return nil
}

// approveTicket: backlog → pending; on a flagged ticket clear the flag so the
// daemon resumes (review loop gets a fresh round budget).
func approveTicket(db *store.DB, t store.Ticket) error {
	if t.Status == store.TBacklog {
		return db.UpdateTicketFields(t.ID, map[string]any{"status": store.TPending, "needs_user": 0})
	}
	if !t.NeedsUser {
		return nil
	}
	fields := map[string]any{"needs_user": 0}
	if t.ReviewState == store.RError {
		fields["review_state"] = ""
	}
	if t.ReviewState == store.RRequestChanges {
		fields["review_rounds"] = 0
	}
	return db.UpdateTicketFields(t.ID, fields)
}

func (v *ticketsView) render(ws *workspace, w, h int) string {
	ts := v.visible(&ws.snap)
	v.cursor = clampCursor(v.cursor, len(ts))
	var tabs []string
	for i, f := range ticketFilters {
		tab := sMuted.Render(" " + f + " ")
		if i == v.filter {
			tab = pill(f, colOnAcc, colAccent)
		}
		tabs = append(tabs, mark(fmt.Sprintf("tab:%d", i), tab))
	}
	var rows []string
	for _, t := range ts {
		extra := ""
		switch {
		case t.LockedBy != "":
			extra = sAccent.Render(t.LockedBy)
		case t.PRURL != "":
			extra = sMuted.Render(prNumber(t.PRURL))
		}
		if len(t.DependsOn) > 0 && t.Status != store.TDone {
			extra += sMuted.Render(" ⇠ " + store.JoinIDs(t.DependsOn))
		}
		if t.PRConflict {
			extra += sErr.Render(" ⚔ conflict")
		}
		if t.NeedsUser {
			extra += sWarn.Render(" ⚠")
		}
		if ws.snap.sessions[agent.TicketSession(ws.snap.project.Slug, t.ID)] {
			extra += sAccent.Render(" 💬")
		}
		rows = append(rows, fmt.Sprintf("%s %s %-36s %s %s %s", statusIcon(t.Status), sBold.Render(fmt.Sprintf("#%-3d", t.ID)), trunc(t.Title, 36),
			sMuted.Render(fmt.Sprintf("F%-3d", t.FeatureID)), statusStyle(t.Status).Render(fmt.Sprintf("%-11s", t.Status)), extra))
	}
	detail := ""
	if len(ts) > 0 {
		detail = ticketDetail(ts[v.cursor])
	}
	header := "TICKETS  " + strings.Join(tabs, "")
	if ws.snap.sessions[agent.NewTicketSession(ws.snap.project.Slug)] {
		header += sAccent.Render("   💬 new-ticket chat running (N to reopen)")
	}
	return listDetail(header, rows, v.cursor, w, h, detail)
}

func ticketDetail(t store.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s  %s %s\n", sAccent.Bold(true).Render(fmt.Sprintf("#%d", t.ID)), sBold.Render(t.Title), statusIcon(t.Status), statusStyle(t.Status).Render(t.Status))
	if t.Branch != "" {
		fmt.Fprintf(&b, "branch %s  %s\n", t.Branch, t.PRURL)
	}
	fmt.Fprintf(&b, "depends on: %s · review: %s (round %d)\n", orDash(store.JoinIDs(t.DependsOn)), orDash(t.ReviewState), t.ReviewRounds)
	if t.PRConflict {
		b.WriteString(sErr.Render(fmt.Sprintf("⚔ PR has merge conflicts — auto-resolve attempt %d/2 (Developer rebases onto the PR target)", t.ConflictAttempts)) + "\n")
	}
	if t.NeedsUser {
		b.WriteString(sWarn.Render("⚠ needs you: press a to resume after checking the agent log / PR") + "\n")
	}
	fmt.Fprintf(&b, "\n%s\n%s\n\n%s\n%s\n", sTitle.Render("REQUIREMENTS"), t.Requirements, sTitle.Render("ACCEPTANCE CRITERIA"), t.Acceptance)
	if t.ChangeRequest != "" {
		fmt.Fprintf(&b, "\n%s\n%s\n", sTitle.Render("CHANGE REQUEST (TechLead)"), t.ChangeRequest)
	}
	if t.ReviewSummary != "" {
		fmt.Fprintf(&b, "\n%s\n%s\n", sTitle.Render("REVIEW"), t.ReviewSummary)
	}
	return b.String()
}

func prNumber(url string) string {
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return "PR#" + url[i+1:]
	}
	return url
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}
