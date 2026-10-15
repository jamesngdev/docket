package store

import "time"

// Document is one Markdown file of the docs repo as of the last scan.
type Document struct {
	Path, Hash, Change, Commit string // Change: A|M|D|"" relative to the previous scan
	ScannedAt                  time.Time
}

type ThirdDoc struct {
	ID                              int64
	Type, Title, URL, Content, Note string // Type: link|cached|note
}

// Run is one agent process (TechLead, Developer or Tester) hosted in tmux.
type Run struct {
	ID, Role, Task, Session, Workdir, Status, Summary string
	ClaudeSession                                     string // Claude Code session id (resumable with --resume)
	TicketID                                          int64
	Callback                                          bool
	StartedAt, EndedAt                                time.Time
}

// Run statuses.
const (
	RunRunning = "running"
	RunDone    = "done"
	RunFailed  = "failed"
	RunStopped = "stopped"
)

func (d *DB) ThirdDocs() ([]ThirdDoc, error) {
	rows, err := d.Query(`SELECT id,type,title,url,content,note FROM third_docs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ThirdDoc
	for rows.Next() {
		var t ThirdDoc
		if err := rows.Scan(&t.ID, &t.Type, &t.Title, &t.URL, &t.Content, &t.Note); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) AddThirdDoc(t ThirdDoc) error {
	_, err := d.Exec(`INSERT INTO third_docs(type,title,url,content,note) VALUES(?,?,?,?,?)`, t.Type, t.Title, t.URL, t.Content, t.Note)
	return err
}

func (d *DB) DeleteThirdDoc(id int64) error {
	_, err := d.Exec(`DELETE FROM third_docs WHERE id=?`, id)
	return err
}

func (d *DB) InsertRun(r Run) error {
	_, err := d.Exec(`INSERT INTO runs(id,role,task,ticket_id,session,workdir,claude_session,started_at) VALUES(?,?,?,?,?,?,?,?)`,
		r.ID, r.Role, r.Task, r.TicketID, r.Session, r.Workdir, r.ClaudeSession, now())
	return err
}

const runCols = `id,role,task,ticket_id,session,workdir,status,callback,summary,started_at,ended_at,claude_session`

func scanRun(s interface{ Scan(...any) error }) (Run, error) {
	var r Run
	var st, en int64
	err := s.Scan(&r.ID, &r.Role, &r.Task, &r.TicketID, &r.Session, &r.Workdir, &r.Status, &r.Callback, &r.Summary, &st, &en, &r.ClaudeSession)
	r.StartedAt = time.Unix(st, 0)
	if en > 0 {
		r.EndedAt = time.Unix(en, 0)
	}
	return r, err
}

// Runs returns running runs first, then the most recent finished ones.
func (d *DB) Runs(limit int) ([]Run, error) {
	rows, err := d.Query(`SELECT `+runCols+` FROM runs
		ORDER BY CASE status WHEN 'running' THEN 0 ELSE 1 END, started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *DB) RunByID(id string) (Run, error) {
	return scanRun(d.QueryRow(`SELECT `+runCols+` FROM runs WHERE id=?`, id))
}

// RunningRuns returns every run still marked running.
func (d *DB) RunningRuns() ([]Run, error) {
	all, err := d.Runs(1000)
	if err != nil {
		return nil, err
	}
	var out []Run
	for _, r := range all {
		if r.Status == RunRunning {
			out = append(out, r)
		}
	}
	return out, nil
}

// MarkCallback records the agent's `docket run done` call.
func (d *DB) MarkCallback(id, summary string) error {
	_, err := d.Exec(`UPDATE runs SET callback=1, summary=? WHERE id=?`, summary, id)
	return err
}

// LastClaudeSession returns the Claude session of the latest run of role on a
// ticket if that run finished cleanly (`docket run done`), else "". Only a
// clean run is worth resuming; a crashed or stopped one starts fresh.
func (d *DB) LastClaudeSession(role string, ticketID int64) string {
	var s, status string
	var cb bool
	_ = d.QueryRow(`SELECT claude_session, status, callback FROM runs WHERE role=? AND ticket_id=?
		ORDER BY started_at DESC, rowid DESC LIMIT 1`, role, ticketID).Scan(&s, &status, &cb)
	if status != RunDone || !cb {
		return ""
	}
	return s
}

func (d *DB) FinishRun(id, status string) error {
	_, err := d.Exec(`UPDATE runs SET status=?, ended_at=? WHERE id=?`, status, now(), id)
	return err
}
