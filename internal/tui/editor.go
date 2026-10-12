package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// editor is the inline multi-line textarea modal used for every long field.
type editor struct {
	title  string
	ta     textarea.Model
	onSave func(string) error
}

// openEditor shows the modal; onSave runs on Ctrl-S and the modal closes on success.
func (ws *workspace) openEditor(title, initial string, onSave func(string) error) tea.Cmd {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.MaxHeight = 0
	ta.SetValue(initial)
	ws.editor = &editor{title: title, ta: ta, onSave: onSave}
	return ws.editor.ta.Focus()
}

func (e *editor) update(ws *workspace, msg tea.Msg) tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			ws.editor = nil
			return flash("edit cancelled", nil)
		case "ctrl+s":
			if err := e.onSave(e.ta.Value()); err != nil {
				return flash("save", err)
			}
			ws.editor = nil
			cmds := append([]tea.Cmd{flash("saved", nil), ws.load()}, ws.pending...)
			ws.pending = nil
			return tea.Batch(cmds...)
		}
	}
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return cmd
}

func (e *editor) view(w, h int) string {
	e.ta.SetWidth(w - 4)
	e.ta.SetHeight(max(h-5, 3))
	return sModal.Width(w - 2).Render(sTitle.Render(e.title) + "\n" + e.ta.View())
}
