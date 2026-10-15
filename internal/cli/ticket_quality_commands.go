package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"docket/internal/gate"
	"docket/internal/gh"
	"docket/internal/paths"
	"docket/internal/store"
)

var checklistItem = regexp.MustCompile(`(?m)^\s*- \[[ xX]\]\s+(.*)$`)

// validateAcceptance enforces a checklist where every item says how it is verified,
// so Developers, the gate and the Tester all check the same thing.
func validateAcceptance(acc string) error {
	items := checklistItem.FindAllStringSubmatch(acc, -1)
	if len(items) == 0 {
		return fmt.Errorf("acceptance must be a Markdown checklist (\"- [ ] ... — verify: <test or command>\")")
	}
	var missing []string
	for _, it := range items {
		if !strings.Contains(strings.ToLower(it[1]), "verify:") {
			missing = append(missing, strings.TrimSpace(it[1]))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("every acceptance item needs \"— verify: <test name, command or request>\"; missing on:\n- %s",
			strings.Join(missing, "\n- "))
	}
	return nil
}

var modelRe = regexp.MustCompile(`^(haiku|sonnet|opus|fable|claude-[a-z0-9.-]+)$`)

// ticketExtras reads --files and --model shared by ticket add/update.
func ticketExtras(a args, fields map[string]any) error {
	if v, ok := a.flags["files"]; ok {
		fields["planned_files"] = normalizeList(v)
	}
	if v, ok := a.flags["model"]; ok {
		v = strings.TrimSpace(v)
		if v != "" && v != "default" && !modelRe.MatchString(v) {
			return fmt.Errorf("--model must be haiku, sonnet, opus, fable, a claude-* id, or default")
		}
		if v == "default" {
			v = ""
		}
		fields["model"] = v
	}
	return nil
}

func normalizeList(s string) string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ",")
}

// waitReviewGate runs the mechanical gate for a Developer run moving its ticket
// to wait_review, and records the passed commit and its risk flags.
func waitReviewGate(db *store.DB, p paths.Project, t store.Ticket, runID, prURL string) (map[string]any, error) {
	r, err := db.RunByID(runID)
	if err != nil {
		return nil, err
	}
	if r.Workdir == "" {
		return nil, fmt.Errorf("run %s has no worktree; the wait_review gate cannot run", runID)
	}
	proj, err := db.Project()
	if err != nil {
		return nil, err
	}
	cfg, err := gate.LoadConfig(p.Checks())
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", p.Checks(), err)
	}
	env, _ := os.ReadFile(p.DevDBEnv())
	start := time.Now()
	fmt.Fprintf(os.Stderr, "gate: checking ticket #%d (%d verify checks)…\n", t.ID, len(cfg.Checks))
	res, err := gate.Run(gate.Input{Worktree: r.Workdir, Branch: t.Branch, Target: proj.Settings.PRTarget,
		Planned: t.PlannedFiles, PRBody: gh.PRBody(r.Workdir, prURL), Env: gate.EnvLines(string(env)), Config: cfg})
	if err != nil {
		db.Log("gate", "ticket #%d: gate failed", t.ID)
		return nil, err
	}
	risks := strings.Join(res.Risks, "; ")
	fmt.Printf("gate passed on %s in %s", res.SHA[:min(8, len(res.SHA))], time.Since(start).Round(time.Second))
	if risks != "" {
		fmt.Printf(" (risk flags: %s)", risks)
	}
	fmt.Println()
	return map[string]any{"gate_sha": res.SHA, "risk_flags": risks}, nil
}

// noteAdd appends one learned fact to .docket/REPO.md for every later agent.
func noteAdd(p paths.Project, a args, actor string) error {
	text, err := a.need("text")
	if err != nil {
		return err
	}
	text = oneLineText(text)
	f, err := os.OpenFile(p.RepoNotes(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "\n- %s (%s, %s)", text, actor, time.Now().Format("2006-01-02"))
	return err
}
