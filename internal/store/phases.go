package store

import (
	"fmt"
	"time"
)

// A phase is one branch of the docs repo. The project works on one active
// phase at a time; questions, features, tickets and the docs scan are scoped
// to the phase they were created in.

const phaseSchema = `
CREATE TABLE IF NOT EXISTS phases (
  name TEXT PRIMARY KEY, docs_commit TEXT NOT NULL DEFAULT '', prev_docs_commit TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS phase_documents (
  phase TEXT NOT NULL, path TEXT NOT NULL, content_hash TEXT NOT NULL, change TEXT NOT NULL DEFAULT '',
  scanned_commit TEXT NOT NULL DEFAULT '', scanned_at INTEGER NOT NULL,
  PRIMARY KEY (phase, path)
);`

// phaseColumns are added to existing tables by migrate (ALTER TABLE is idempotent here).
var phaseColumns = map[string][]string{
	"project":   {"active_phase TEXT NOT NULL DEFAULT ''", "prev_phase TEXT NOT NULL DEFAULT ''"},
	"questions": {"phase TEXT NOT NULL DEFAULT ''"},
	"features":  {"phase TEXT NOT NULL DEFAULT ''"},
	"tickets": {"phase TEXT NOT NULL DEFAULT ''", "change_request TEXT NOT NULL DEFAULT ''",
		"pr_conflict INTEGER NOT NULL DEFAULT 0", "conflict_attempts INTEGER NOT NULL DEFAULT 0",
		"merge_checked_at INTEGER NOT NULL DEFAULT 0",
		"planned_files TEXT NOT NULL DEFAULT ''", "model TEXT NOT NULL DEFAULT ''",
		"risk_flags TEXT NOT NULL DEFAULT ''", "gate_sha TEXT NOT NULL DEFAULT ''"},
	"runs": {"claude_session TEXT NOT NULL DEFAULT ''"},
}

// activePhaseSQL is used as a sub-select to scope queries/inserts to the active phase.
const activePhaseSQL = `(SELECT active_phase FROM project WHERE id=1)`

func (d *DB) migrate() error {
	if _, err := d.Exec(phaseSchema); err != nil {
		return err
	}
	for table, cols := range phaseColumns {
		existing := map[string]bool{}
		rows, err := d.Query(`SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			return err
		}
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			existing[n] = true
		}
		rows.Close()
		for _, col := range cols {
			name := col[:indexByte(col, ' ')]
			if existing[name] {
				continue
			}
			if _, err := d.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s`, table, col)); err != nil {
				return fmt.Errorf("add %s.%s: %w", table, name, err)
			}
		}
	}
	return nil
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return len(s)
}

// SetActivePhase switches the working phase, remembering the previous one
// so the first scan of the new phase can be diffed against it.
func (d *DB) SetActivePhase(name string) error {
	_, err := d.Exec(`UPDATE project SET prev_phase=active_phase, active_phase=? WHERE id=1 AND active_phase<>?`, name, name)
	return err
}

// PhaseCommit returns the last scanned docs commit of a phase ("" if never scanned).
func (d *DB) PhaseCommit(name string) string {
	var c string
	_ = d.QueryRow(`SELECT docs_commit FROM phases WHERE name=?`, name).Scan(&c)
	return c
}

// AnyPhaseScanned reports whether any phase was ever scanned (false ⇒ first run, bootstrap).
func (d *DB) AnyPhaseScanned() bool {
	var n int
	_ = d.QueryRow(`SELECT COUNT(*) FROM phases WHERE docs_commit<>''`).Scan(&n)
	return n > 0
}

// SetDocsCommit records a scan of the active phase. baseline is the commit the
// scan was diffed against (previous scan of this phase, or of the previous phase).
func (d *DB) SetDocsCommit(commit, baseline string) error {
	_, err := d.Exec(`INSERT INTO phases(name, docs_commit, prev_docs_commit, created_at)
		VALUES(`+activePhaseSQL+`, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET docs_commit=excluded.docs_commit, prev_docs_commit=excluded.prev_docs_commit`,
		commit, baseline, now())
	return err
}

const docCols = `SELECT path,content_hash,change,scanned_commit,scanned_at FROM phase_documents WHERE phase=`
const docOrder = ` ORDER BY CASE change WHEN '' THEN 1 ELSE 0 END, path`

// Documents returns the active phase's docs scan.
func (d *DB) Documents() ([]Document, error) {
	return d.queryDocs(docCols + activePhaseSQL + docOrder)
}

// DocumentsOf returns the docs scan of a named phase (diff baseline for a new phase).
func (d *DB) DocumentsOf(phase string) ([]Document, error) {
	return d.queryDocs(docCols+`?`+docOrder, phase)
}

func (d *DB) queryDocs(q string, args ...any) ([]Document, error) {
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var doc Document
		var at int64
		if err := rows.Scan(&doc.Path, &doc.Hash, &doc.Change, &doc.Commit, &at); err != nil {
			return nil, err
		}
		doc.ScannedAt = time.Unix(at, 0)
		out = append(out, doc)
	}
	return out, rows.Err()
}

// ReplaceDocuments overwrites the active phase's docs scan.
func (d *DB) ReplaceDocuments(docs []Document) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM phase_documents WHERE phase=` + activePhaseSQL); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := tx.Exec(`INSERT INTO phase_documents(phase,path,content_hash,change,scanned_commit,scanned_at)
			VALUES(`+activePhaseSQL+`,?,?,?,?,?)`, doc.Path, doc.Hash, doc.Change, doc.Commit, now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FeatureTitlesOutsidePhase lists features planned in other phases (context for TechLead).
func (d *DB) FeatureTitlesOutsidePhase() ([]string, error) {
	rows, err := d.Query(`SELECT phase, id, title, status FROM features WHERE phase<>` + activePhaseSQL + ` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var phase, title, status string
		var id int64
		if err := rows.Scan(&phase, &id, &title, &status); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("[%s] F%d %s (%s)", phase, id, title, status))
	}
	return out, rows.Err()
}
