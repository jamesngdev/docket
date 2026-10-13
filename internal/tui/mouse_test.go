package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"docket/internal/store"
)

// TestMouseFlow clicks real rendered zones and checks the resulting state.
func TestMouseFlow(t *testing.T) {
	zone.NewGlobal()
	ws := &workspace{cmdline: textinput.New(), agents: &agentsView{}}
	ws.views = []view{&dashboardView{}, &questionsView{}, &featuresView{}, &ticketsView{}, ws.agents, &docsView{}}
	ws.snap.project = store.Project{Name: "p", ActivePhase: "main", DocsCommit: "x"}
	ws.snap.daemonUp = true
	ws.snap.tickets = []store.Ticket{{ID: 1, Title: "a", Status: store.TPending}, {ID: 2, Title: "b", Status: store.TPending}, {ID: 3, Title: "c", Status: store.TDone}}
	a := &app{w: 120, h: 36, screen: scrMain, main: ws}
	for i, l := range strings.Split(a.View(), "\n") {
		if ansi.StringWidth(l) > a.w {
			t.Fatalf("line %d is %d wide: zone markers leaked into layout", i, ansi.StringWidth(l))
		}
	}
	clickOn := func(id string) {
		t.Helper()
		a.View()
		time.Sleep(30 * time.Millisecond)
		z := zone.Get(id)
		if z.IsZero() {
			t.Fatalf("zone %s not found", id)
		}
		a.Update(tea.MouseMsg{X: z.StartX, Y: z.StartY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	}
	clickOn("view:3")
	if ws.active != vTickets {
		t.Fatalf("sidebar click: active=%d", ws.active)
	}
	clickOn("row:2")
	if ws.tickets().cursor != 2 {
		t.Fatalf("row click: cursor=%d", ws.tickets().cursor)
	}
	clickOn("tab:5")
	if ticketFilters[ws.tickets().filter] != store.TDone {
		t.Fatalf("tab click: filter=%d", ws.tickets().filter)
	}
	a.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	clickOn("key:?")
	if !ws.help {
		t.Fatal("hint click did not press ?")
	}
	clickOn("view:0") // closes help first
	clickOn("view:0")
	if ws.active != vDashboard {
		t.Fatalf("active=%d", ws.active)
	}
	clickOn("board:pending")
	if ws.active != vTickets || ticketFilters[ws.tickets().filter] != store.TPending {
		t.Fatalf("board click: active=%d filter=%d", ws.active, ws.tickets().filter)
	}
}
