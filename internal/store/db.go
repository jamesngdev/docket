// Package store is the per-project SQLite database. TUI, daemon and the
// agent-facing CLI all open the same file (WAL mode, busy timeout) so the
// database is the single source of truth for project state.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS project (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  name TEXT NOT NULL, slug TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
  repo_url TEXT NOT NULL DEFAULT '', repo_path TEXT NOT NULL, docs_repo_url TEXT NOT NULL DEFAULT '',
  settings TEXT NOT NULL DEFAULT '{}', docs_commit TEXT NOT NULL DEFAULT '',
  prev_docs_commit TEXT NOT NULL DEFAULT '', queued_task TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS documents (
  path TEXT PRIMARY KEY, content_hash TEXT NOT NULL, change TEXT NOT NULL DEFAULT '',
  scanned_commit TEXT NOT NULL DEFAULT '', scanned_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS questions (
  id INTEGER PRIMARY KEY AUTOINCREMENT, text TEXT NOT NULL, answer TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'open', source TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS features (
  id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
  documents TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft', created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS tickets (
  id INTEGER PRIMARY KEY AUTOINCREMENT, feature_id INTEGER NOT NULL DEFAULT 0,
  title TEXT NOT NULL, requirements TEXT NOT NULL DEFAULT '', acceptance TEXT NOT NULL DEFAULT '',
  depends_on TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'backlog',
  branch TEXT NOT NULL DEFAULT '', pr_url TEXT NOT NULL DEFAULT '', locked_by TEXT NOT NULL DEFAULT '',
  review_state TEXT NOT NULL DEFAULT '', review_summary TEXT NOT NULL DEFAULT '',
  review_rounds INTEGER NOT NULL DEFAULT 0, needs_user INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS third_docs (
  id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL, title TEXT NOT NULL,
  url TEXT NOT NULL DEFAULT '', content TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS runs (
  id TEXT PRIMARY KEY, role TEXT NOT NULL, task TEXT NOT NULL DEFAULT '', ticket_id INTEGER NOT NULL DEFAULT 0,
  session TEXT NOT NULL, workdir TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'running',
  callback INTEGER NOT NULL DEFAULT 0, summary TEXT NOT NULL DEFAULT '',
  started_at INTEGER NOT NULL, ended_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS activity (
  id INTEGER PRIMARY KEY AUTOINCREMENT, at INTEGER NOT NULL, actor TEXT NOT NULL, message TEXT NOT NULL
);`

// Open opens (and migrates) the project database at path.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // serialize writers inside one process
	d := &DB{db}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return d, nil
}

func now() int64 { return time.Now().Unix() }

// Log appends an activity entry; errors are ignored because activity is informational.
func (d *DB) Log(actor, format string, args ...any) {
	_, _ = d.Exec(`INSERT INTO activity(at, actor, message) VALUES(?,?,?)`, now(), actor, fmt.Sprintf(format, args...))
}

type Activity struct {
	At      time.Time
	Actor   string
	Message string
}

func (d *DB) RecentActivity(limit int) ([]Activity, error) {
	rows, err := d.Query(`SELECT at, actor, message FROM activity ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		var at int64
		if err := rows.Scan(&at, &a.Actor, &a.Message); err != nil {
			return nil, err
		}
		a.At = time.Unix(at, 0)
		out = append(out, a)
	}
	return out, rows.Err()
}
