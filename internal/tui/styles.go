package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	colAccent = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#A78BFA"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8B8F98"}
	colOK     = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colErr    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}

	sAccent   = lipgloss.NewStyle().Foreground(colAccent)
	sBold     = lipgloss.NewStyle().Bold(true)
	sTitle    = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	sMuted    = lipgloss.NewStyle().Foreground(colMuted)
	sOK       = lipgloss.NewStyle().Foreground(colOK)
	sWarn     = lipgloss.NewStyle().Foreground(colWarn)
	sErr      = lipgloss.NewStyle().Foreground(colErr)
	sSelected = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	sFrame    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colMuted)
	sModal    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colAccent).Padding(0, 1)
)

// trunc shortens s (ANSI-aware) to width w.
func trunc(s string, w int) string {
	if w <= 1 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// cursorLine renders one list row with a selection marker.
func cursorLine(selected bool, s string, w int) string {
	if selected {
		return sSelected.Render(trunc("▸ "+s, w))
	}
	return trunc("  "+s, w)
}

// window returns the [start,end) slice of n rows that keeps cursor visible in h rows.
func window(cursor, n, h int) (int, int) {
	if h <= 0 || n <= h {
		return 0, n
	}
	start := cursor - h/2
	if start < 0 {
		start = 0
	}
	if start+h > n {
		start = n - h
	}
	return start, start + h
}

// clampCursor keeps a cursor inside [0, n).
func clampCursor(c, n int) int {
	if c >= n {
		c = n - 1
	}
	if c < 0 {
		c = 0
	}
	return c
}

// moveCursor applies vim-style motion keys; returns the new cursor and whether the key was handled.
func moveCursor(key string, c, n int) (int, bool) {
	switch key {
	case "j", "down":
		return clampCursor(c+1, n), true
	case "k", "up":
		return clampCursor(c-1, n), true
	case "g", "home":
		return 0, true
	case "G", "end":
		return clampCursor(n-1, n), true
	}
	return c, false
}

// lines splits text into at most max lines, each truncated to w.
func lines(text string, w, max int) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		for _, wl := range strings.Split(ansi.Wordwrap(l, w, ""), "\n") {
			if len(out) == max {
				return out
			}
			out = append(out, trunc(wl, w))
		}
	}
	return out
}

func statusIcon(status string) string {
	switch status {
	case "done", "approved", "answered":
		return sOK.Render("✓")
	case "processing", "running":
		return sAccent.Render("◐")
	case "wait_review":
		return sWarn.Render("◔")
	case "pending", "open", "draft":
		return sWarn.Render("●")
	case "outdated", "skipped", "stopped":
		return sMuted.Render("⊘")
	case "failed":
		return sErr.Render("✗")
	}
	return sMuted.Render("○")
}
