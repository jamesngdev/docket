// Package tui is the Bubble Tea interface: project picker, new-project form
// and the per-project workspace (dashboard, questions, features, tickets,
// agents, docs). It reads/writes SQLite directly and controls the daemon
// over its unix socket; it never dispatches agents itself.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the TUI on the alternate screen.
func Run() error {
	_, err := tea.NewProgram(newApp(), tea.WithAltScreen()).Run()
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
}

func newApp() *app {
	return &app{screen: scrPicker, picker: newPicker()}
}

func (a *app) Init() tea.Cmd { return tea.Batch(a.picker.load(), tick()) }

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
		return a, msg.ws.load()
	case toPickerMsg:
		a.closeMain()
		a.screen = scrPicker
		return a, a.picker.load()
	case newProjectMsg:
		a.form, a.screen = newProjectForm(), scrForm
		return a, a.form.init()
	case cancelFormMsg:
		a.screen = scrPicker
		return a, nil
	case tickMsg:
		var cmd tea.Cmd
		switch a.screen {
		case scrMain:
			cmd = a.main.load()
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
	switch a.screen {
	case scrPicker:
		return a.picker.view(a.w, a.h)
	case scrForm:
		return a.form.view(a.w, a.h)
	default:
		return a.main.view(a.w, a.h)
	}
}

func (a *app) closeMain() {
	if a.main != nil {
		a.main.close()
		a.main = nil
	}
}
