// Package support runs the read-only Backend Support agent: one headless
// `claude -p` call per Discord message, resuming the thread's session.
//
// Safety model: the agent gets NO shell — only Read, Grep, Glob and Skill —
// so it cannot write, run programs or reach the network. Its working
// directory is a detached worktree of the base branch (tracked files only, so
// .env files and .docket/ data are not there); reads outside the working
// directory and the --add-dir folders need approval, which nobody can give in
// headless mode, so they are denied; secret-looking files are denied even
// inside. Everything it would have asked `docket`/`git` for is pre-rendered
// into the context directory.
package support

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"docket/internal/agent"
	"docket/internal/config"
	"docket/internal/gitops"
	"docket/internal/paths"
	"docket/internal/store"
)

const (
	answerTimeout = 10 * time.Minute
	syncEvery     = 5 * time.Minute
	worktreeName  = "support"
	contextID     = "support"
)

// tools is everything Backend Support can use. No Bash: shell commands (even
// "read-only" git ones like `git diff --no-index`) could leave the sandbox.
const tools = "Read,Grep,Glob,Skill"

// passEnv are the daemon environment variables the agent inherits; everything
// else (cloud keys, GITHUB_TOKEN, …) stays out of its reach.
var passEnv = []string{"HOME", "USER", "LOGNAME", "SHELL", "TERM", "TMPDIR", "LANG", "LC_ALL", "LC_CTYPE",
	"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy"}

// errSessionMissing means --resume pointed at a session Claude no longer has.
var errSessionMissing = errors.New("claude session not found")

var (
	prepMu    sync.Mutex // worktree sync, context files and the home are shared by all threads
	lastSync  time.Time
	homeModel string // model the isolated home was last built for ("" = not built yet)
)

// Request is one Discord message to answer.
type Request struct {
	Session  string // Claude session of the thread; "" starts a new conversation
	Author   string
	Question string
}

// Answer is the agent's reply plus the session to resume next time.
type Answer struct {
	Text    string
	Session string
}

// Ask answers one message. It is safe to call concurrently for different threads.
func Ask(ctx context.Context, p paths.Project, db *store.DB, r Request) (Answer, error) {
	proj, err := db.Project()
	if err != nil {
		return Answer{}, err
	}
	wt, ctxDir, err := prepare(p, db, proj)
	if err != nil {
		return Answer{}, err
	}
	model := proj.Settings.Models[agent.RoleSupport]
	home, err := ensureHome(p, model, wt)
	if err != nil {
		return Answer{}, fmt.Errorf("build claude home: %w", err)
	}
	prompt := followUp(r)
	if r.Session == "" {
		prompt = firstPrompt(r, proj, p, ctxDir)
	}
	a, err := run(ctx, p, home, wt, ctxDir, model, r.Session, prompt)
	if errors.Is(err, errSessionMissing) {
		// The session is gone (e.g. claude home wiped): start over with full context.
		a, err = run(ctx, p, home, wt, ctxDir, model, "", firstPrompt(r, proj, p, ctxDir))
	}
	return a, err
}

// ensureHome builds the isolated Claude home once per daemon (and again only
// when the model changes), so running agents never see it half-rewritten.
func ensureHome(p paths.Project, model, wt string) (string, error) {
	prepMu.Lock()
	defer prepMu.Unlock()
	home := p.ClaudeHome(agent.RoleSupport)
	if homeModel == model {
		return home, nil
	}
	if err := agent.BuildHome(home, agent.RoleSupport, model, wt, nil); err != nil {
		return "", err
	}
	homeModel = model
	return home, nil
}

// prepare syncs the read-only worktree and rewrites the project context files.
func prepare(p paths.Project, db *store.DB, proj store.Project) (wt, ctxDir string, err error) {
	prepMu.Lock()
	defer prepMu.Unlock()
	wt, ctxDir = p.Worktree(worktreeName), p.RunContext(contextID)
	_ = os.MkdirAll(p.Docs(), 0o755) // --add-dir needs it even for projects without a docs repo
	if _, statErr := os.Stat(wt); statErr != nil || time.Since(lastSync) > syncEvery {
		if err := gitops.SyncDetachedWorktree(proj.RepoPath, wt, proj.Settings.BaseBranch); err != nil {
			return "", "", fmt.Errorf("sync support worktree: %w", err)
		}
		lastSync = time.Now()
	}
	in := agent.TechleadInput{Project: proj, Task: "answer questions from teammates on Discord (read-only)",
		DocsHead: proj.DocsCommit, PrevCommit: proj.PrevDocsCommit}
	in.Docs, _ = db.Documents()
	in.Questions, _ = db.Questions()
	in.Features, _ = db.Features()
	in.Tickets, _ = db.Tickets()
	third, _ := db.ThirdDocs()
	err = agent.WriteFiles(ctxDir, map[string]string{
		"PROJECT.md":      agent.ProjectMarkdown(in),
		"TICKETS.md":      ticketsMarkdown(in.Tickets),
		"GIT_LOG.md":      gitLogMarkdown(wt, proj.Settings.BaseBranch),
		"3TH_DOCUMENT.md": agent.ThirdDocsMarkdown(third),
	})
	return wt, ctxDir, err
}

func firstPrompt(r Request, proj store.Project, p paths.Project, ctxDir string) string {
	return fmt.Sprintf(`%s asks on Discord:

%s

---
Context directory: %s (PROJECT.md: questions+answers, features, documents; TICKETS.md: full tickets; GIT_LOG.md: recent commits; 3TH_DOCUMENT.md)
Docs directory: %s (phase %s)
Source code: your working directory, base branch %s.
Answer as Backend Support (read-only). Use skill answer-discord.`,
		r.Author, r.Question, ctxDir, p.Docs(), proj.ActivePhase, proj.Settings.BaseBranch)
}

func followUp(r Request) string {
	return fmt.Sprintf("%s (follow-up in the thread):\n\n%s", r.Author, r.Question)
}

// result is the subset of `claude -p --output-format json` we use.
type result struct {
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
	IsError   bool   `json:"is_error"`
}

func run(ctx context.Context, p paths.Project, home, wt, ctxDir, model, session, prompt string) (Answer, error) {
	ctx, cancel := context.WithTimeout(ctx, answerTimeout)
	defer cancel()
	args := []string{"-p", "--setting-sources", "user", "--strict-mcp-config", "--model", model,
		"--tools", tools,
		"--add-dir", p.Docs(), "--add-dir", ctxDir, "--output-format", "json"}
	if session != "" {
		args = append(args, "--resume", session)
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = wt
	cmd.Env = env(home)
	cmd.Stdin = strings.NewReader(prompt)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	runErr := cmd.Run()
	if strings.Contains(out.String()+errb.String(), "No conversation found") {
		return Answer{}, errSessionMissing
	}
	if ctx.Err() != nil {
		return Answer{}, fmt.Errorf("claude: %w", ctx.Err())
	}
	var res result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		if runErr == nil {
			runErr = err
		}
		return Answer{}, fmt.Errorf("claude: %v: %s", runErr, strings.TrimSpace(errb.String()))
	}
	if res.IsError || strings.TrimSpace(res.Result) == "" {
		return Answer{}, errors.New("claude: " + orDefault(strings.TrimSpace(res.Result), "empty answer"))
	}
	return Answer{Text: strings.TrimSpace(res.Result), Session: res.SessionID}, nil
}

// env is a minimal environment: passEnv plus the isolated Claude home.
func env(home string) []string {
	var out []string
	for _, k := range passEnv {
		if v, ok := os.LookupEnv(k); ok {
			out = append(out, k+"="+v)
		}
	}
	out = append(out, "PATH="+os.Getenv("PATH"), "CLAUDE_CONFIG_DIR="+home, "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")
	if cfg, _ := config.Load(); cfg.OAuthToken != "" {
		out = append(out, "CLAUDE_CODE_OAUTH_TOKEN="+cfg.OAuthToken)
	} else {
		out = append(out, "CLAUDE_SECURESTORAGE_CONFIG_DIR=") // reuse the user's login (see agent.Launch)
	}
	return out
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
