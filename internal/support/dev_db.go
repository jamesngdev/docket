package support

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"docket/internal/devdb"
	"docket/internal/paths"
)

// executable is the docket binary serving support-db-mcp (tests point it at a build).
var executable = os.Executable

// dbArgs returns the claude flags that expose the project's dev database to
// Backend Support through `docket support-db-mcp` (nil when none is set). The
// MCP config holds no secret: the server reads the URL from project settings.
func dbArgs(p paths.Project, home, url string) ([]string, error) {
	if url == "" {
		return nil, nil
	}
	exe, err := executable()
	if err != nil {
		return nil, err
	}
	cfg := map[string]any{"mcpServers": map[string]any{devdb.ServerName: map[string]any{
		"type": "stdio", "command": exe, "args": []string{"support-db-mcp", "--project", p.Repo},
	}}}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, "devdb-mcp.json")
	prepMu.Lock() // threads answer concurrently; never let one read a half-written file
	defer prepMu.Unlock()
	if old, _ := os.ReadFile(path); !bytes.Equal(old, b) {
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return nil, err
		}
	}
	return []string{"--mcp-config", path, "--allowedTools", "mcp__" + devdb.ServerName}, nil
}

// dbLine tells the agent a dev database is available (and which kind).
func dbLine(url string) string {
	if url == "" {
		return ""
	}
	kind, _ := devdb.Kind(url)
	return "Dev database (" + kind + "): query it read-only with the mcp__devdb__* tools when the answer depends on data.\n"
}
