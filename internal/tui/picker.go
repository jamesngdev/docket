package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"docket/internal/config"
	"docket/internal/paths"
	"docket/internal/project"
	"docket/internal/store"
)

type newProjectMsg struct{}

type projectRow struct {
	repo, name, summary string
	broken              bool
}

type pickerLoadedMsg struct {
	rows []projectRow
	err  error
}

// picker is the launch screen listing registered projects.
type picker struct {
	rows   []projectRow
	cursor int
	err    string
}

func newPicker() *picker { return &picker{} }

func (p *picker) load() tea.Cmd {
	return func() tea.Msg {
		c, err := config.Load()
		if err != nil {
			return pickerLoadedMsg{err: err}
		}
		var rows []projectRow
		for _, repo := range c.Projects {
			rows = append(rows, summarize(repo))
		}
		return pickerLoadedMsg{rows: rows}
	}
}

func summarize(repo string) projectRow {
	row := projectRow{repo: repo, name: repo}
	if err := project.MigrateLegacy(repo); err != nil {
		row.broken, row.summary = true, err.Error()
		return row
	}
	dbPath := paths.New(repo).DB()
	if _, err := os.Stat(dbPath); err != nil {
		row.broken, row.summary = true, "missing .docket database"
		return row
	}
	db, err := store.Open(dbPath)
	if err != nil {
		row.broken, row.summary = true, err.Error()
		return row
	}
	defer db.Close()
	if p, err := db.Project(); err == nil {
		row.name = p.Name
	}
	ts, _ := db.Tickets()
	qs, _ := db.Questions()
	runs, _ := db.RunningRuns()
	done, total, open := 0, 0, 0
	for _, t := range ts {
		if t.Status != store.TOutdated {
			total++
		}
		if t.Status == store.TDone {
			done++
		}
	}
	for _, q := range qs {
		if q.Status == store.QOpen {
			open++
		}
	}
	row.summary = fmt.Sprintf("agents %d   tickets %d/%d done", len(runs), done, total)
	if open > 0 {
		row.summary += sWarn.Render(fmt.Sprintf("   ⚠ %d questions open", open))
	}
	return row
}

func (p *picker) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case pickerLoadedMsg:
		p.rows = msg.rows
		p.cursor = clampCursor(p.cursor, len(p.rows))
		if msg.err != nil {
			p.err = msg.err.Error()
		}
	case tea.KeyMsg:
		key := msg.String()
		if c, ok := moveCursor(key, p.cursor, len(p.rows)); ok {
			p.cursor = c
			return nil
		}
		switch key {
		case "q":
			return tea.Quit
		case "n":
			return func() tea.Msg { return newProjectMsg{} }
		case "enter":
			if len(p.rows) > 0 && !p.rows[p.cursor].broken {
				repo := p.rows[p.cursor].repo
				return func() tea.Msg { return openProjectMsg{repo: repo} }
			}
		case "d":
			if len(p.rows) > 0 {
				if err := config.RemoveProject(p.rows[p.cursor].repo); err != nil {
					p.err = err.Error()
				}
				return p.load()
			}
		}
	}
	return nil
}

// click selects a project row; clicking the selected row opens it.
func (p *picker) click(m tea.MouseMsg) tea.Cmd {
	kind, arg := hit(m)
	i := zoneIndex(arg)
	if kind != "pick" || i < 0 || i >= len(p.rows) {
		return nil
	}
	if i != p.cursor {
		p.cursor = i
		return nil
	}
	return p.update(tea.KeyMsg{Type: tea.KeyEnter})
}

// logo is the launch-screen wordmark; it gets the shimmer sweep.
var logo = []string{
	"█▀▄ █▀█ █▀▀ █▄▀ █▀▀ ▀█▀",
	"█▄▀ █▄█ █▄▄ █ █ ██▄  █ ",
}

func (p *picker) view(w, h int) string {
	inner := w - 8
	var b strings.Builder
	b.WriteString("\n")
	for _, l := range logo {
		b.WriteString(shimmer(l, sTitle) + "\n")
	}
	b.WriteString(sMuted.Render("docs → questions → features → tickets → PRs, by isolated Claude Code agents") + "\n\n")
	b.WriteString(section("PROJECTS", fmt.Sprintf("%d registered", len(p.rows)), inner) + "\n\n")
	if len(p.rows) == 0 {
		b.WriteString(sMuted.Render("  ◌ No projects yet. Press ") + sAccent.Bold(true).Render("n") + sMuted.Render(" to create one.") + "\n")
	}
	for i, r := range p.rows {
		icon := sOK.Render("●")
		if r.broken {
			icon = sErr.Render("✗")
		}
		line := fmt.Sprintf("%s %s %s  %s", icon, sBold.Render(padRight(trunc(r.name, 22), 22)), sMuted.Render(padRight(trunc(r.repo, 40), 40)), r.summary)
		b.WriteString(mark(fmt.Sprintf("pick:%d", i), cursorLine(i == p.cursor, line, inner)) + "\n")
	}
	if p.err != "" {
		st, icon := sErr, "✗ "
		if strings.HasPrefix(p.err, "opening ") {
			st, icon = sAccent, spin(frame)+" "
		}
		b.WriteString("\n" + st.Render(trunc(icon+p.err, inner)) + "\n")
	}
	b.WriteString("\n" + keyHints("n new project · Enter open · d remove from list · q quit", inner))
	return sFrame.Width(w - 2).Height(h - 2).Render(lipgloss.NewStyle().PaddingLeft(3).Render(b.String()))
}
