package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// SaveSettings replaces the project settings (defaults filled in).
func (d *DB) SaveSettings(s Settings) error {
	b, err := json.Marshal(s.withDefaults())
	if err != nil {
		return err
	}
	_, err = d.Exec(`UPDATE project SET settings=? WHERE id=1`, string(b))
	return err
}

// SupportThread returns the Claude session behind a Discord thread opened by
// Backend Support; ok is false for threads it does not own.
func (d *DB) SupportThread(threadID string) (session string, ok bool, err error) {
	err = d.QueryRow(`SELECT session_id FROM discord_threads WHERE thread_id=?`, threadID).Scan(&session)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return session, err == nil, err
}

// SaveSupportThread records (or updates) the session of a Backend Support thread.
func (d *DB) SaveSupportThread(threadID, session, author string) error {
	_, err := d.Exec(`INSERT INTO discord_threads(thread_id, session_id, author, created_at) VALUES(?,?,?,?)
		ON CONFLICT(thread_id) DO UPDATE SET session_id=excluded.session_id`, threadID, session, author, now())
	return err
}
