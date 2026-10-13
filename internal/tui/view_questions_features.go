package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"docket/internal/store"
)

type featuresView struct{ listCursor }

func (featuresView) name() string { return "Features" }
func (featuresView) badge(s *snapshot) int {
	n := 0
	for _, f := range s.features {
		if f.Status == store.FDraft {
			n++
		}
	}
	return n
}
func (featuresView) hints() string {
	return "a approve · A approve all · x reject · e description · t title"
}

func (v *featuresView) keys(ws *workspace, k tea.KeyMsg) tea.Cmd {
	fs := ws.snap.features
	if c, ok := moveCursor(k.String(), v.cursor, len(fs)); ok {
		v.cursor = c
		return nil
	}
	if len(fs) == 0 {
		return nil
	}
	f := fs[clampCursor(v.cursor, len(fs))]
	switch k.String() {
	case "a":
		return ws.dbDo(fmt.Sprintf("F%d approved", f.ID), func() error { return ws.db.SetFeatureStatus(f.ID, store.FApproved) })
	case "A":
		return ws.dbDo("all draft features approved", func() error {
			for _, x := range fs {
				if x.Status == store.FDraft {
					if err := ws.db.SetFeatureStatus(x.ID, store.FApproved); err != nil {
						return err
					}
				}
			}
			return nil
		})
	case "x":
		return ws.dbDo(fmt.Sprintf("F%d rejected", f.ID), func() error { return ws.db.SetFeatureStatus(f.ID, store.FOutdated) })
	case "e", "enter":
		return ws.openEditor(fmt.Sprintf("F%d description", f.ID), f.Description, func(s string) error {
			return ws.db.UpdateFeature(f.ID, f.Title, strings.TrimSpace(s))
		})
	case "t":
		return ws.openEditor(fmt.Sprintf("F%d title", f.ID), f.Title, func(s string) error {
			return ws.db.UpdateFeature(f.ID, strings.TrimSpace(s), f.Description)
		})
	}
	return nil
}

func (v *featuresView) render(ws *workspace, w, h int) string {
	fs := ws.snap.features
	v.cursor = clampCursor(v.cursor, len(fs))
	nTickets := map[int64]int{}
	for _, t := range ws.snap.tickets {
		if t.Status != store.TOutdated {
			nTickets[t.FeatureID]++
		}
	}
	var rows []string
	for _, f := range fs {
		rows = append(rows, fmt.Sprintf("%s %s %-44s %s %s", statusIcon(f.Status), sBold.Render(fmt.Sprintf("F%-3d", f.ID)), trunc(f.Title, 44),
			statusStyle(f.Status).Render(fmt.Sprintf("%-9s", f.Status)), sMuted.Render(fmt.Sprintf("%d tickets", nTickets[f.ID]))))
	}
	detail := ""
	if len(fs) > 0 {
		f := fs[v.cursor]
		detail = fmt.Sprintf("%s %s  %s %s\n%s\n\n%s", sAccent.Bold(true).Render(fmt.Sprintf("F%d", f.ID)), sBold.Render(f.Title), statusIcon(f.Status),
			statusStyle(f.Status).Render(f.Status), sMuted.Render(fmt.Sprintf("docs: %s · tickets: %d", f.Documents, nTickets[f.ID])), f.Description)
	}
	return listDetail(fmt.Sprintf("FEATURES   %d draft · %d total", v.badge(&ws.snap), len(fs)), rows, v.cursor, w, h, detail)
}
