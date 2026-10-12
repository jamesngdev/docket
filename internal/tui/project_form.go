package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/project"
	"docket/internal/store"
)

type cancelFormMsg struct{}

type projectCreatedMsg struct {
	repo string
	err  error
}

var formFields = []struct{ label, def string }{
	{"Name", ""}, {"Description", ""}, {"GitHub repo", ""}, {"Local path", ""}, {"Docs repo", ""}, {"Docs phase", ""},
	{"Base branch", "develop"}, {"PR target", "develop"}, {"Max agents", "3"},
	{"Model techlead", "opus"}, {"Model developer", "sonnet"}, {"Model tester", "sonnet"},
}

// projectForm is the "New project" screen.
type projectForm struct {
	inputs []textinput.Model
	focus  int
	busy   bool
	err    string
}

func newProjectForm() *projectForm {
	f := &projectForm{}
	for _, fd := range formFields {
		ti := textinput.New()
		ti.SetValue(fd.def)
		ti.Prompt = ""
		ti.CharLimit = 300
		f.inputs = append(f.inputs, ti)
	}
	f.inputs[0].Focus()
	return f
}

func (f *projectForm) init() tea.Cmd { return textinput.Blink }

func (f *projectForm) val(i int) string { return strings.TrimSpace(f.inputs[i].Value()) }

func (f *projectForm) submit() tea.Cmd {
	s := store.DefaultSettings()
	s.BaseBranch, s.PRTarget = f.val(6), f.val(7)
	if n, err := strconv.Atoi(f.val(8)); err == nil && n > 0 {
		s.MaxParallelAgents = n
	}
	s.Models = map[string]string{"techlead": f.val(9), "developer": f.val(10), "tester": f.val(11)}
	in := project.Input{Name: f.val(0), Description: f.val(1), RepoURL: f.val(2), LocalPath: f.val(3),
		DocsRepoURL: f.val(4), Phase: f.val(5), Settings: s}
	f.busy, f.err = true, ""
	return func() tea.Msg {
		repo, err := project.Create(in)
		return projectCreatedMsg{repo: repo, err: err}
	}
}

func (f *projectForm) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectCreatedMsg:
		f.busy = false
		if msg.err != nil {
			f.err = msg.err.Error()
			return nil
		}
		return func() tea.Msg { return openProjectMsg{repo: msg.repo} }
	case tea.KeyMsg:
		if f.busy {
			return nil
		}
		switch msg.String() {
		case "esc":
			return func() tea.Msg { return cancelFormMsg{} }
		case "ctrl+s":
			return f.submit()
		case "tab", "down", "enter":
			return f.setFocus(f.focus + 1)
		case "shift+tab", "up":
			return f.setFocus(f.focus - 1)
		}
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return cmd
}

func (f *projectForm) setFocus(i int) tea.Cmd {
	n := len(f.inputs)
	f.inputs[f.focus].Blur()
	f.focus = (i%n + n) % n
	return f.inputs[f.focus].Focus()
}

func (f *projectForm) view(w, h int) string {
	var b strings.Builder
	b.WriteString(sTitle.Render("New project") + "\n\n")
	for i, fd := range formFields {
		label := sMuted.Render(padRight(fd.label, 16))
		if i == f.focus {
			label = sAccent.Render(padRight(fd.label, 16))
		}
		f.inputs[i].Width = w - 26
		b.WriteString(label + " [ " + f.inputs[i].View() + " ]\n")
	}
	b.WriteString("\n" + sMuted.Render("Local path is cloned from GitHub repo when it does not exist. Docs phase = docs branch (empty: default branch).") + "\n")
	switch {
	case f.busy:
		b.WriteString(sAccent.Render("Creating project: cloning repos, preparing .docket/ …") + "\n")
	case f.err != "":
		b.WriteString(sErr.Render(trunc(f.err, w-6)) + "\n")
	}
	b.WriteString("\n" + sMuted.Render("Tab next field · Ctrl-S create · Esc cancel"))
	return sFrame.Width(w-2).Height(h-2).Padding(0, 2).Render(b.String())
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
