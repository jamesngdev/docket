// Package tui is the Bubble Tea interface: project picker, new-project form
// and the per-project workspace (dashboard, questions, features, tickets,
// agents, docs). It reads/writes SQLite directly and controls the daemon
// over its unix socket; it never dispatches agents itself.
package tui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"docket/internal/config"
	"docket/internal/project"
)

// Run starts the TUI on the alternate screen with mouse support. With a
// path it skips the project picker: it opens the docket project containing
// path (registering it if needed), or the new-project form prefilled for it.
func Run(path string) error {
	zone.NewGlobal()
	defer zone.Close()
	a := newApp()
	if path != "" {
		if repo, err := project.Find(path); err == nil {
			_ = config.AddProject(repo) // so the picker lists it next time
			a.start = func() tea.Msg { return openProjectMsg{repo: repo} }
		} else {
			a.form, a.screen = newProjectForm(path), scrForm
		}
	}
	_, err := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

type screen int

const (
	scrPicker screen = iota
	scrForm
	scrMain
)

type (
	tickMsg            time.Time
	openProjectMsg     struct{ repo string }
	toPickerMsg        struct{}
	workspaceOpenedMsg struct {
		ws  *workspace
		err error
	}
	flashMsg struct {
		text string
		err  bool
		at   time.Time // set when received; drives the toast fade
	}
)

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func flash(text string, err error) tea.Cmd {
	return func() tea.Msg {
		if err != nil {
			return flashMsg{text: text + ": " + err.Error(), err: true}
		}
		return flashMsg{text: text}
	}
}

type app struct {
	w, h   int
	screen screen
	picker *picker
	form   *projectForm
	main   *workspace
	start  tea.Cmd // first action when launched as `docket <path>`
}

func newApp() *app {
	return &app{screen: scrPicker, picker: newPicker()}
}

func (a *app) Init() tea.Cmd {
	cmds := []tea.Cmd{a.picker.load(), tick(), animate(), a.start}
	if a.form != nil {
		cmds = append(cmds, a.form.init())
	}
	return tea.Batch(cmds...)
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			a.closeMain()
			return a, tea.Quit
		}
	case openProjectMsg:
		repo := msg.repo
		a.picker.err = "opening " + repo + " (starting daemon)…"
		return a, func() tea.Msg {
			ws, err := openWorkspace(repo)
			return workspaceOpenedMsg{ws: ws, err: err}
		}
	case workspaceOpenedMsg:
		if msg.err != nil {
			a.picker.err = msg.err.Error()
			a.screen = scrPicker
			return a, nil
		}
		a.picker.err = ""
		a.closeMain()
		a.main, a.screen = msg.ws, scrMain
		return a, tea.Batch(msg.ws.load(), msg.ws.refreshUsage())
	case toPickerMsg:
		a.closeMain()
		a.screen = scrPicker
		return a, a.picker.load()
	case newProjectMsg:
		cwd, _ := os.Getwd()
		a.form, a.screen = newProjectForm(cwd), scrForm
		return a, a.form.init()
	case cancelFormMsg:
		a.screen = scrPicker
		return a, nil
	case tea.MouseMsg:
		if k, ok := mouseKey(msg); ok {
			return a.Update(k)
		}
		if !leftClick(msg) {
			return a, nil
		}
		switch a.screen {
		case scrPicker:
			return a, a.picker.click(msg)
		case scrForm:
			return a, a.form.click(msg)
		default:
			return a, a.main.click(msg)
		}
	case animMsg:
		frame++
		if a.screen == scrMain {
			a.main.animate()
		}
		return a, animate()
	case tickMsg:
		var cmd tea.Cmd
		switch a.screen {
		case scrMain:
			cmd = tea.Batch(a.main.load(), a.main.refreshUsage())
		case scrPicker:
			cmd = a.picker.load()
		}
		return a, tea.Batch(cmd, tick())
	}
	switch a.screen {
	case scrPicker:
		return a, a.picker.update(msg)
	case scrForm:
		return a, a.form.update(msg)
	default:
		return a, a.main.update(msg)
	}
}

func (a *app) View() string {
	if a.w == 0 {
		return "loading…"
	}
	zoneIDs = zoneIDs[:0]
	var out string
	switch a.screen {
	case scrPicker:
		out = a.picker.view(a.w, a.h)
	case scrForm:
		out = a.form.view(a.w, a.h)
	default:
		out = a.main.view(a.w, a.h)
	}
	return zone.Scan(out)
}

func (a *app) closeMain() {
	if a.main != nil {
		a.main.close()
		a.main = nil
	}
}
