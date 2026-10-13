package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"docket/internal/config"
	"docket/internal/usage"
)

// usageEvery throttles calls to the Claude usage API, which rate-limits
// aggressive polling (HTTP 429).
const usageEvery = 5 * time.Minute

// usageCache is shared by every workspace, so reopening or switching projects
// shows the last result instead of calling the API again.
var usageCache struct {
	limits usage.Limits
	at     time.Time // last fetch attempt
}

type usageMsg struct {
	limits usage.Limits
	err    error
}

// usageRow is one limit window plus its animated (eased) display value.
type usageRow struct {
	label, short string
	win          *usage.Window
	shown        *float64
}

// refreshUsage fetches subscription usage in the background, at most once per usageEvery.
func (ws *workspace) refreshUsage() tea.Cmd {
	if time.Since(usageCache.at) < usageEvery {
		if ws.usageAt.IsZero() && !usageCache.at.IsZero() { // fresh workspace: reuse the cached result
			ws.usage, ws.usageAt = usageCache.limits, usageCache.at
		}
		return nil
	}
	usageCache.at, ws.usageAt, ws.usageBusy = time.Now(), time.Now(), true
	return func() tea.Msg {
		cfg, _ := config.Load()
		l, err := usage.Fetch(context.Background(), cfg.OAuthToken)
		return usageMsg{l, err}
	}
}

func (ws *workspace) usageRows() []usageRow {
	l := ws.usage
	return []usageRow{
		{"Session · 5h", "5h", l.Session, &ws.usageShown[0]},
		{"Weekly", "7d", l.Weekly, &ws.usageShown[1]},
		{"Weekly · Opus", "opus", l.Opus, &ws.usageShown[2]},
		{"Weekly · Sonnet", "sonnet", l.Sonnet, &ws.usageShown[3]},
	}
}

// animateUsage eases each bar toward its real value; bars grow in on first load.
func (ws *workspace) animateUsage() {
	for _, r := range ws.usageRows() {
		if r.win != nil {
			*r.shown = ease(*r.shown, r.win.Percent, 0.22)
		}
	}
}

// pctStyle colours a percentage by the same heat ramp as the bars.
func pctStyle(pct float64) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(heat(pct / 100))
}

// usageBrief is the compact header form: "5h ▰▰▱▱▱ 34%  7d ▱▱▱▱▱ 2%".
func (ws *workspace) usageBrief() string {
	var parts []string
	for _, r := range ws.usageRows()[:2] {
		if r.win != nil {
			parts = append(parts, sMuted.Render(r.short+" ")+miniBar(*r.shown, 5)+" "+pctStyle(*r.shown).Render(fmt.Sprintf("%.0f%%", *r.shown)))
		}
	}
	return strings.Join(parts, "  ")
}

// usageSection is the dashboard block: a heat bar and reset countdown per window.
func (ws *workspace) usageSection(w int) string {
	var b strings.Builder
	meta := ""
	switch {
	case ws.usageBusy:
		meta = sAccent.Render(spin(frame) + " refreshing")
	case !ws.usageAt.IsZero():
		meta = "updated " + ago(ws.usageAt)
	}
	b.WriteString(section("CLAUDE USAGE", meta, w) + "\n")
	barW := min(max(w-50, 10), 28)
	shown := 0
	for _, r := range ws.usageRows() {
		if r.win == nil {
			continue
		}
		shown++
		pct := *r.shown
		line := fmt.Sprintf("%-15s %s %s  %s", r.label, heatBar(pct, barW),
			pctStyle(pct).Render(fmt.Sprintf("%3.0f%%", pct)), sMuted.Render("↻ "+resetIn(r.win.ResetsAt)))
		b.WriteString(" " + trunc(line, w-1) + "\n")
	}
	if shown == 0 {
		msg := sAccent.Render(spin(frame)) + sMuted.Render(" fetching usage…")
		if ws.usageErr != nil && !ws.usageBusy {
			msg = sWarn.Render("⚠ ") + sMuted.Render(trunc("unavailable: "+ws.usageErr.Error(), w-4))
		}
		b.WriteString(" " + msg + "\n")
	}
	return b.String()
}

// ago renders a past time as "12s ago" / "3m ago".
func ago(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm ago", int(d.Minutes()))
}

// resetIn renders a reset time as "1h 14m · Mon 17:10".
func resetIn(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Until(t).Round(time.Minute)
	if d <= 0 {
		return "now"
	}
	var rel string
	switch {
	case d >= 24*time.Hour:
		rel = fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		rel = fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		rel = fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return rel + " · " + t.Local().Format("Mon 15:04")
}
