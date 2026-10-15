package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"docket/internal/extskills"
	"docket/internal/paths"
	"docket/internal/tmux"
)

// Spawn describes one agent run to launch.
type Spawn struct {
	Paths    paths.Project
	Slug     string
	Role     string
	Model    string
	RunID    string
	TicketID int64
	Workdir  string
	Prompt   string
	Token    string            // CLAUDE_CODE_OAUTH_TOKEN; empty = reuse the user's normal `claude` login
	Extra    []extskills.Skill // external team skills to install (and announce in the prompt)
	// ClaudeSession pins the Claude Code session id ("" = let claude pick one).
	// With Resume the run continues that existing conversation instead.
	ClaudeSession string
	Resume        bool
}

// NewRunID returns a short sortable id like "developer-m3x9k2".
func NewRunID(role string) string {
	return role + "-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
}

// SessionName follows docket-<slug>-<role>-<ticket|run>.
func SessionName(slug, role string, ticketID int64, runID string) string {
	suffix := runID
	if ticketID > 0 {
		suffix = strconv.FormatInt(ticketID, 10)
	}
	return fmt.Sprintf("docket-%s-%s-%s", slug, role, suffix)
}

// Launch prepares the isolated home + MCP config and starts `claude` in tmux.
// Returns the tmux session name.
func Launch(s Spawn) (string, error) {
	home := s.Paths.ClaudeHome(s.Role)
	if err := BuildHome(home, s.Role, s.Model, s.Workdir, s.Extra); err != nil {
		return "", fmt.Errorf("build claude home: %w", err)
	}
	if err := WriteMCPConfig(s.Paths.MCPConfig()); err != nil {
		return "", err
	}
	promptFile := filepath.Join(s.Paths.RunContext(s.RunID), "PROMPT.md")
	prompt := s.Prompt + extskills.PromptSection(s.Extra, s.Role)
	if err := WriteFiles(filepath.Dir(promptFile), map[string]string{"PROMPT.md": prompt}); err != nil {
		return "", err
	}
	bin, err := os.Executable()
	if err != nil {
		return "", err
	}
	env := map[string]string{
		"CLAUDE_CONFIG_DIR":               home,
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY": "1",
		"DOCKET_PROJECT":                  s.Paths.Repo,
		"DOCKET_RUN":                      s.RunID,
		"DOCKET_DATA":                     s.Paths.Root(),
		"PATH":                            filepath.Dir(bin) + string(os.PathListSeparator) + os.Getenv("PATH"),
	}
	if s.Token != "" {
		env["CLAUDE_CODE_OAUTH_TOKEN"] = s.Token
	} else {
		// Reuse the user's own Claude Code login. Claude keys its keychain item by
		// config dir; an EMPTY CLAUDE_SECURESTORAGE_CONFIG_DIR selects the default
		// item ("Claude Code-credentials") while settings/skills still come from
		// the isolated CLAUDE_CONFIG_DIR. Verified with claude 2.1.296.
		env["CLAUDE_SECURESTORAGE_CONFIG_DIR"] = ""
	}
	sessionFlag := ""
	switch {
	case s.ClaudeSession != "" && s.Resume:
		sessionFlag = "--resume " + tmux.Quote(s.ClaudeSession) + " "
	case s.ClaudeSession != "":
		sessionFlag = "--session-id " + tmux.Quote(s.ClaudeSession) + " "
	}
	cmd := fmt.Sprintf(`exec claude --setting-sources user --strict-mcp-config --mcp-config %s --dangerously-skip-permissions --model %s %s"$(cat %s)"`,
		tmux.Quote(s.Paths.MCPConfig()), tmux.Quote(s.Model), sessionFlag, tmux.Quote(promptFile))
	session := SessionName(s.Slug, s.Role, s.TicketID, s.RunID)
	tmux.Kill(session) // stale session from a crashed run
	err = tmux.Start(tmux.Spec{Name: session, Dir: s.Workdir, Env: env, Command: cmd, LogFile: s.Paths.RunLog(s.RunID)})
	return session, err
}
