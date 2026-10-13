package tui

import (
	"fmt"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// animMsg drives frame-based effects (spinners, easing bars) independently of
// the 2s data tick.
type animMsg time.Time

const animEvery = 100 * time.Millisecond

func animate() tea.Cmd {
	return tea.Tick(animEvery, func(t time.Time) tea.Msg { return animMsg(t) })
}

// frame is the global animation counter, advanced on every animMsg. Bubble Tea
// runs Update and View on one goroutine, so no locking is needed.
var frame int

var spinFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// spin returns the spinner glyph for a frame counter.
func spin(frame int) string { return string(spinFrames[frame%len(spinFrames)]) }

// ease moves cur a fraction (rate) of the way to target, snapping when close.
func ease(cur, target, rate float64) float64 {
	if math.Abs(target-cur) < 0.05 {
		return target
	}
	return cur + (target-cur)*rate
}

// heat colours a 0..1 position on a green → amber → red ramp, per theme.
func heat(pos float64) lipgloss.TerminalColor {
	pos = min(max(pos, 0), 1)
	pick := func(g, a, r [3]float64) string {
		from, to, t := g, a, pos*2
		if pos > 0.5 {
			from, to, t = a, r, (pos-0.5)*2
		}
		return fmt.Sprintf("#%02X%02X%02X", lerp(from[0], to[0], t), lerp(from[1], to[1], t), lerp(from[2], to[2], t))
	}
	return lipgloss.AdaptiveColor{
		Light: pick([3]float64{0x15, 0x80, 0x3D}, [3]float64{0xB4, 0x53, 0x09}, [3]float64{0xB9, 0x1C, 0x1C}),
		Dark:  pick([3]float64{0x4A, 0xDE, 0x80}, [3]float64{0xFB, 0xBF, 0x24}, [3]float64{0xF8, 0x71, 0x71}),
	}
}

func lerp(a, b, t float64) int { return int(a + (b-a)*t + 0.5) }

var eighths = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// heatBar draws pct (0..100) as a smooth bar whose cells are coloured by their
// position on the heat ramp, so a fuller bar visibly runs hotter.
func heatBar(pct float64, width int) string {
	units := int(min(max(pct, 0), 100) / 100 * float64(width*8))
	var s string
	for i := 0; i < width; i++ {
		c := lipgloss.NewStyle().Foreground(heat(float64(i) / float64(max(width-1, 1))))
		switch left := units - i*8; {
		case left >= 8:
			s += c.Render("█")
		case left > 0:
			s += c.Render(eighths[left])
		default:
			s += sMuted.Render("·")
		}
	}
	return s
}

// miniBar is the compact header gauge: ▰▰▱▱▱.
func miniBar(pct float64, width int) string {
	n := int(min(max(pct, 0), 100)/100*float64(width) + 0.5)
	var s string
	for i := 0; i < width; i++ {
		if i < n {
			s += lipgloss.NewStyle().Foreground(heat(float64(i) / float64(max(width-1, 1)))).Render("▰")
		} else {
			s += sMuted.Render("▱")
		}
	}
	return s
}
