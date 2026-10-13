package tui

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"docket/internal/ipc"
	"docket/internal/store"
)

type docsView struct{ listCursor }

func (docsView) name() string        { return "Docs" }
func (docsView) badge(*snapshot) int { return 0 }
func (docsView) hints() string {
	return "Enter switch phase / preview · r pull & rescan · i add 3rd-party doc · d delete 3rd-party doc"
}

func (v *docsView) keys(ws *workspace, k tea.KeyMsg) tea.Cmd {
	s := &ws.snap
	nP, nD := len(s.phases), len(s.docs)
	n := nP + nD + len(s.third)
	if c, ok := moveCursor(k.String(), v.cursor, n); ok {
		v.cursor = c
		return nil
	}
	switch k.String() {
	case "r":
		return ws.send(ipc.CmdRescan, "", "docs pull & rescan requested")
	case "i":
		return ws.openEditor("New 3rd-party document (type: link | cached | note)", thirdDocTemplate, func(text string) error {
			d, err := parseThirdDoc(text)
			if err != nil {
				return err
			}
			if d.Type == "cached" && d.Content == "" {
				ws.pending = append(ws.pending, ws.fetchAndStore(d)) // fetched off the UI thread
				return nil
			}
			return ws.db.AddThirdDoc(d)
		})
	}
	if n == 0 {
		return nil
	}
	c := clampCursor(v.cursor, n)
	switch {
	case c < nP: // phase row: Enter switches the active phase
		if k.String() == "enter" {
			name := s.phases[c]
			if name == s.project.ActivePhase {
				return flash("phase "+name+" is already active", nil)
			}
			return ws.send(ipc.CmdPhase, name, "switching to phase "+name+" (docs checkout + rescan)")
		}
	case c < nP+nD:
		if k.String() == "enter" {
			return ws.pager(filepath.Join(ws.paths.Docs(), s.docs[c-nP].Path))
		}
	default:
		td := s.third[c-nP-nD]
		switch k.String() {
		case "d":
			return ws.dbDo("3rd-party doc deleted", func() error { return ws.db.DeleteThirdDoc(td.ID) })
		case "enter":
			return ws.openEditor(td.Title+" (read-only preview, Esc to close)", td.URL+"\n"+td.Note+"\n\n"+td.Content, func(string) error { return nil })
		}
	}
	return nil
}

func (v *docsView) render(ws *workspace, w, h int) string {
	s := &ws.snap
	var rows []string
	for _, ph := range s.phases {
		mark, label := sMuted.Render("○"), sMuted.Render("phase")
		if ph == s.project.ActivePhase {
			mark, label = sOK.Render("●"), sOK.Render("phase · active")
		}
		rows = append(rows, fmt.Sprintf("%s ⎇ %-46s %s", mark, trunc(ph, 46), label))
	}
	for _, d := range s.docs {
		flag := d.Change
		if flag == "" {
			flag = " "
		}
		rows = append(rows, fmt.Sprintf("%s %-48s %s", docChangeStyle(flag).Render(flag), trunc(d.Path, 48), sMuted.Render("scanned "+d.ScannedAt.Format("01-02 15:04"))))
	}
	for _, t := range s.third {
		rows = append(rows, fmt.Sprintf("%s %-30s %s", sAccent.Render("3rd/"+t.Type), trunc(t.Title, 30), sMuted.Render(trunc(t.URL, 50))))
	}
	v.cursor = clampCursor(v.cursor, len(rows))
	head := s.project.DocsCommit
	if len(head) > 8 {
		head = head[:8]
	}
	header := fmt.Sprintf("DOCUMENTS   phase %s @ %s · %d files · %d third-party", s.project.ActivePhase, orDash(head), len(s.docs), len(s.third))
	detail := sMuted.Render("Phases are branches of the docs repo. Enter on a phase switches to it.\n" +
		"Docs are checked out and diffed against the previous phase; TechLead plans only what changed.\n" +
		"Questions, features and tickets are per phase. Agents already running keep going.\n" +
		"A added · M modified · D deleted since the previous scan.")
	return listDetail(header, rows, v.cursor, w, h, detail)
}

// docChangeStyle colours the A/M/D change flag of a scanned doc.
func docChangeStyle(flag string) lipgloss.Style {
	switch flag {
	case "A":
		return sOK
	case "M":
		return sWarn
	case "D":
		return sErr
	}
	return sMuted
}

const thirdDocTemplate = `type: link
title:
url:
note:
---
(content: required for type "note"; for type "cached" leave empty and the URL is fetched)
`

// parseThirdDoc reads the editor template; "cached" fetches the URL body now.
func parseThirdDoc(text string) (store.ThirdDoc, error) {
	var d store.ThirdDoc
	head, body, _ := strings.Cut(text, "\n---\n")
	for _, l := range strings.Split(head, "\n") {
		k, val, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(k) {
		case "type":
			d.Type = val
		case "title":
			d.Title = val
		case "url":
			d.URL = val
		case "note":
			d.Note = val
		}
	}
	d.Content = strings.TrimSpace(body)
	if strings.HasPrefix(d.Content, "(content:") {
		d.Content = ""
	}
	if d.Title == "" {
		return d, fmt.Errorf("title is required")
	}
	switch d.Type {
	case "link":
	case "note":
		if d.Content == "" {
			return d, fmt.Errorf("a note needs content after ---")
		}
	case "cached":
		if d.URL == "" {
			return d, fmt.Errorf("cached docs need a url")
		}
	default:
		return d, fmt.Errorf("type must be link, cached or note")
	}
	return d, nil
}

// fetchAndStore downloads a cached doc in the background, then saves it.
func (ws *workspace) fetchAndStore(d store.ThirdDoc) tea.Cmd {
	db := ws.db
	return func() tea.Msg {
		c, err := fetch(d.URL)
		if err != nil {
			return flashMsg{text: "fetch " + d.URL + ": " + err.Error(), err: true}
		}
		d.Content = c
		if err := db.AddThirdDoc(d); err != nil {
			return flashMsg{text: "save doc: " + err.Error(), err: true}
		}
		return flashMsg{text: "cached " + d.Title}
	}
}

func fetch(url string) (string, error) {
	cl := &http.Client{Timeout: 20 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return string(b), err
}
