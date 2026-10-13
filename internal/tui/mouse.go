package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Mouse support: clickable things are wrapped in bubblezone markers while
// rendering (mark), app.View scans them into screen rectangles, and a click
// is resolved back to the zone id (hit). Ids look like "<kind>:<arg>".

// zoneIDs are the ids marked during the last render, innermost last.
var zoneIDs []string

// mark makes s clickable under id.
func mark(id, s string) string {
	zoneIDs = append(zoneIDs, id)
	return zone.Mark(id, s)
}

// hit returns the kind and argument of the zone under the mouse, if any.
func hit(m tea.MouseMsg) (kind, arg string) {
	for i := len(zoneIDs) - 1; i >= 0; i-- {
		if zone.Get(zoneIDs[i]).InBounds(m) {
			kind, arg, _ = strings.Cut(zoneIDs[i], ":")
			return kind, arg
		}
	}
	return "", ""
}

// zoneIndex parses a numeric zone argument.
func zoneIndex(arg string) int {
	n, err := strconv.Atoi(arg)
	if err != nil {
		return -1
	}
	return n
}

// mouseKey turns wheel scrolls and clicks on key hints into key presses, so
// they behave exactly like the keyboard on every screen.
func mouseKey(m tea.MouseMsg) (tea.KeyMsg, bool) {
	if m.Action != tea.MouseActionPress {
		return tea.KeyMsg{}, false
	}
	switch m.Button {
	case tea.MouseButtonWheelUp:
		return tea.KeyMsg{Type: tea.KeyUp}, true
	case tea.MouseButtonWheelDown:
		return tea.KeyMsg{Type: tea.KeyDown}, true
	case tea.MouseButtonLeft:
		if kind, arg := hit(m); kind == "key" {
			return keyFor(arg)
		}
	}
	return tea.KeyMsg{}, false
}

// keyFor maps a hint token ("Enter/e", "Ctrl-S", "a") to the key it names.
func keyFor(tok string) (tea.KeyMsg, bool) {
	tok, _, _ = strings.Cut(tok, "/")
	switch strings.ToLower(tok) {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}, true
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}, true
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}, true
	case "ctrl-s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}, true
	}
	if r := []rune(tok); len(r) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}, true
	}
	return tea.KeyMsg{}, false
}

// leftClick reports whether m is a left-button press.
func leftClick(m tea.MouseMsg) bool {
	return m.Action == tea.MouseActionPress && m.Button == tea.MouseButtonLeft
}

// listCursor is the selection state shared by list views; mouse clicks set it.
type listCursor struct{ cursor int }

func (l *listCursor) cur() int     { return l.cursor }
func (l *listCursor) setCur(i int) { l.cursor = i }

type cursorView interface {
	cur() int
	setCur(int)
}

// click handles a left click inside the workspace.
func (ws *workspace) click(m tea.MouseMsg) tea.Cmd {
	kind, arg := hit(m)
	if ws.help {
		ws.help = false
		return nil
	}
	if ws.editor != nil || ws.cmdMode {
		return nil
	}
	i := zoneIndex(arg)
	switch kind {
	case "view":
		if i >= 0 && i < len(ws.views) {
			ws.active = i
			return ws.load()
		}
	case "cta":
		_, run := nextStep(ws)
		return run()
	case "row":
		v := ws.views[ws.active]
		cv, ok := v.(cursorView)
		if !ok || i < 0 {
			return nil
		}
		if cv.cur() == i { // clicking the selected row opens it, like Enter
			return v.keys(ws, tea.KeyMsg{Type: tea.KeyEnter})
		}
		cv.setCur(i)
		if v == ws.agents {
			return ws.load() // refresh the pane preview for the new selection
		}
	case "tab":
		return ws.tickets().setFilter(ws, i)
	case "board":
		ws.active = vTickets
		for fi, f := range ticketFilters {
			if f == arg {
				return tea.Batch(ws.load(), ws.tickets().setFilter(ws, fi))
			}
		}
		return ws.load()
	}
	return nil
}

func (ws *workspace) tickets() *ticketsView { return ws.views[vTickets].(*ticketsView) }
