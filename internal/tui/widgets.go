package tui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// section renders "TITLE  meta ───────" filling width w.
func section(title, meta string, w int) string {
	line := sTitle.Render(title)
	if meta = strings.TrimSpace(meta); meta != "" {
		if !strings.Contains(meta, "\x1b") {
			meta = sMuted.Render(meta)
		}
		line += "  " + meta
	}
	if rest := w - ansi.StringWidth(line) - 1; rest > 0 {
		line += " " + sRule.Render(strings.Repeat("─", rest))
	}
	return trunc(line, w)
}

// pill renders a small coloured chip.
func pill(text string, fg, bg lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Padding(0, 1).Render(text)
}

// chip is a neutral pill.
func chip(text string) string { return pill(text, colMuted, colChipBg) }

// keyHints styles "a approve · e edit" so each leading key stands out. Each
// hint is a clickable button that presses its key; hints that don't fit in w
// are dropped whole so no click zone is cut in half.
func keyHints(s string, w int) string {
	var b strings.Builder
	used := 0
	for _, h := range strings.Split(s, " · ") {
		key, desc, _ := strings.Cut(strings.TrimSpace(h), " ")
		part := sAccent.Bold(true).Render(key) + " " + sMuted.Render(desc)
		sep := ""
		if used > 0 {
			sep = "  "
		}
		if used+len(sep)+ansi.StringWidth(part) > w {
			break
		}
		used += len(sep) + ansi.StringWidth(part)
		b.WriteString(sep + mark("key:"+key, part))
	}
	return b.String()
}

// shimmer paints text with a highlight that sweeps left to right over time.
func shimmer(text string, base lipgloss.Style) string {
	rs := []rune(text)
	period := len(rs) + 14
	pos := frame/2%period - 4
	var b strings.Builder
	for i, r := range rs {
		st := base
		switch d := i - pos; {
		case d == 0:
			st = st.Foreground(lipgloss.AdaptiveColor{Light: "#C4B5FD", Dark: "#F5F3FF"})
		case d == -1 || d == 1:
			st = st.Foreground(lipgloss.AdaptiveColor{Light: "#8B5CF6", Dark: "#DDD6FE"})
		}
		b.WriteString(st.Render(string(r)))
	}
	return b.String()
}

// pulse alternates between two glyphs for a gentle blink on things awaiting the user.
func pulse(a, b string) string {
	if frame/6%2 == 0 {
		return a
	}
	return b
}

var actorColors = []lipgloss.TerminalColor{colAccent, colInfo, colOK, colWarn,
	lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"}}

// actorStyle gives each activity actor a stable colour.
func actorStyle(name string) lipgloss.Style {
	h := fnv.New32a()
	h.Write([]byte(name))
	return lipgloss.NewStyle().Foreground(actorColors[h.Sum32()%uint32(len(actorColors))])
}

// toast renders the flash message; it fades to muted, then disappears.
func toast(f flashMsg, w int) string {
	age := time.Since(f.at)
	if f.text == "" || age > 8*time.Second {
		return ""
	}
	icon, fg := "✓", colOK
	if f.err {
		icon, fg = "✗", colErr
	}
	if age > 5*time.Second {
		fg = colMuted
	}
	return lipgloss.NewStyle().Foreground(fg).Render(trunc(icon+" "+f.text, w))
}

// listDetail renders a section header, a scrolling list and a detail pane below it.
func listDetail(header string, rows []string, cursor, w, h int, detail string) string {
	title, meta, _ := strings.Cut(header, "  ")
	if len(rows) > 0 {
		meta += sMuted.Render(fmt.Sprintf("  %d/%d", cursor+1, len(rows)))
	}
	var b strings.Builder
	b.WriteString(section(title, meta, w) + "\n\n")
	listH := max((h-3)*45/100, 3)
	start, end := window(cursor, len(rows), listH)
	for i := start; i < end; i++ {
		b.WriteString(mark(fmt.Sprintf("row:%d", i), cursorLine(i == cursor, rows[i], w)) + "\n")
	}
	if len(rows) == 0 {
		b.WriteString(sMuted.Render("  ◌ nothing here yet") + "\n")
	}
	more := ""
	if end < len(rows) {
		more = fmt.Sprintf(" ↓ %d more ", len(rows)-end)
	}
	b.WriteString(sRule.Render(strings.Repeat("╌", max(w-ansi.StringWidth(more), 0))) + sMuted.Render(more) + "\n")
	used := strings.Count(b.String(), "\n")
	b.WriteString(strings.Join(lines(detail, w, max(h-used, 1)), "\n"))
	return b.String()
}
