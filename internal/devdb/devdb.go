// Package devdb gives Backend Support read-only access to the project's dev
// database through a tiny MCP server (`docket support-db-mcp`). The connection
// URL never reaches the agent: the server reads it from project settings.
//
// Read-only is enforced by the server, not by the prompt: Postgres queries run
// as one prepared statement inside a READ ONLY transaction that is always
// rolled back; MongoDB only exposes find/aggregate/count with $out/$merge
// rejected. A read-only database user is still recommended.
package devdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	queryTimeout = 20 * time.Second
	maxRows      = 100  // rows/documents returned per call
	maxCell      = 2000 // characters per rendered row/document
	maxOutput    = 60_000
)

// Tool is one MCP tool: name, description and JSON Schema of its arguments.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Engine is one database backend exposed as MCP tools.
type Engine interface {
	Tools() []Tool
	Call(ctx context.Context, tool string, args json.RawMessage) (string, error)
	Close()
}

// Kind returns "postgres" or "mongodb" for a supported connection URL.
func Kind(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("not a valid connection URL")
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		return "postgres", nil
	case "mongodb", "mongodb+srv":
		return "mongodb", nil
	}
	return "", fmt.Errorf("unsupported database %q (use postgres://… or mongodb://…)", u.Scheme)
}

// Open connects to the database behind raw (postgres:// or mongodb://).
func Open(ctx context.Context, raw string) (Engine, error) {
	kind, err := Kind(raw)
	if err != nil {
		return nil, err
	}
	if kind == "postgres" {
		return openPostgres(ctx, raw)
	}
	return openMongo(ctx, raw)
}

// Ping opens and closes a connection, for `docket discord db` feedback.
func Ping(raw string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	e, err := Open(ctx, raw)
	if err != nil {
		return err
	}
	e.Close()
	return nil
}

// Redact hides the password of a connection URL for display.
func Redact(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "(invalid URL)"
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	}
	u.RawQuery = "" // query params may carry credentials too
	return u.String()
}

// clip shortens s to n characters, marking the cut.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}

// render joins one JSON document per line with a row-count footer.
func render(rows []string, more bool) string {
	var b strings.Builder
	for _, r := range rows {
		if b.Len() > maxOutput {
			b.WriteString("…(output truncated)\n")
			break
		}
		b.WriteString(clip(r, maxCell))
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "(%d rows", len(rows))
	if more {
		fmt.Fprintf(&b, ", limited to %d — narrow the query or aggregate", maxRows)
	}
	b.WriteString(")")
	return b.String()
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
