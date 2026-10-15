package agent

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
)

// NewSessionID returns a random UUID v4 for `claude --session-id`.
func NewSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SessionExists reports whether Claude still has the transcript of session id
// in the role's isolated home (<home>/projects/<encoded cwd>/<id>.jsonl), i.e.
// whether `claude --resume <id>` can continue it.
func SessionExists(home, id string) bool {
	if id == "" {
		return false
	}
	m, _ := filepath.Glob(filepath.Join(home, "projects", "*", id+".jsonl"))
	return len(m) > 0
}
