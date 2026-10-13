package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/project"
	"docket/internal/sh"
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
	{"Auto merge (y/n)", "n"},
}

// projectForm is the "New project" screen.
type projectForm struct {
	inputs []textinput.Model
	focus  int
	busy   bool
	err    string
}

// newProjectForm builds the form, prefilled from dir (see prefill).
func newProjectForm(dir string) *projectForm {
	f := &projectForm{}
	for _, fd := range formFields {
		ti := textinput.New()
		ti.SetValue(fd.def)
		ti.Prompt = ""
		ti.CharLimit = 300
		f.inputs = append(f.inputs, ti)
	}
	f.prefill(dir)
	f.inputs[0].Focus()
	return f
}

// prefill uses dir (where docket was started, or `docket <path>`): Local path
// always (the repo root when inside a git checkout), plus Name and GitHub repo from git.
func (f *projectForm) prefill(dir string) {
	dir, err := filepath.Abs(dir)
	if err != nil || dir == "" {
		return
	}
	if root, err := sh.RunTimeout(5*time.Second, dir, "git", "rev-parse", "--show-toplevel"); err == nil {
		dir = root
		f.inputs[0].SetValue(filepath.Base(root))
	}
	f.inputs[3].SetValue(dir)
	if url, err := sh.RunTimeout(5*time.Second, dir, "git", "remote", "get-url", "origin"); err == nil {
		f.inputs[2].SetValue(url)
	}
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
	s.AutoMerge = strings.HasPrefix(strings.ToLower(f.val(12)), "y")
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

// click focuses the clicked field.
func (f *projectForm) click(m tea.MouseMsg) tea.Cmd {
	kind, arg := hit(m)
	if i := zoneIndex(arg); kind == "field" && i >= 0 && !f.busy {
		return f.setFocus(i)
	}
	return nil
}

func (f *projectForm) setFocus(i int) tea.Cmd {
	n := len(f.inputs)
	f.inputs[f.focus].Blur()
	f.focus = (i%n + n) % n
	return f.inputs[f.focus].Focus()
}

func (f *projectForm) view(w, h int) string {
	var b strings.Builder
	b.WriteString(section("NEW PROJECT", fmt.Sprintf("field %d/%d", f.focus+1, len(formFields)), w-8) + "\n\n")
	for i, fd := range formFields {
		label, bar := sMuted.Render(padRight(fd.label, 16)), "  "
		if i == f.focus {
			label, bar = sAccent.Bold(true).Render(padRight(fd.label, 16)), sAccent.Render("▌ ")
		}
		f.inputs[i].Width = w - 28
		b.WriteString(mark(fmt.Sprintf("field:%d", i), bar+label+sRule.Render("│ ")+f.inputs[i].View()) + "\n")
	}
	b.WriteString("\n" + sMuted.Render("Local path is cloned from GitHub repo when it does not exist. Docs phase = docs branch (empty: default branch).") + "\n")
	switch {
	case f.busy:
		b.WriteString(sAccent.Render(spin(frame)+" Creating project: cloning repos, preparing .docket/ …") + "\n")
	case f.err != "":
		b.WriteString(sErr.Render(trunc("✗ "+f.err, w-6)) + "\n")
	}
	b.WriteString("\n" + keyHints("Tab next field · Ctrl-S create · Esc cancel", w-8))
	return sFrame.Width(w-2).Height(h-2).Padding(1, 3).Render(b.String())
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
