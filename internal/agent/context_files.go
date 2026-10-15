package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"docket/internal/store"
)

// ThirdDocsMarkdown renders all third-party docs into 3TH_DOCUMENT.md content.
func ThirdDocsMarkdown(docs []store.ThirdDoc) string {
	var b strings.Builder
	b.WriteString("# Third-party documents & project knowledge\n\n")
	if len(docs) == 0 {
		b.WriteString("_None._\n")
	}
	for _, d := range docs {
		fmt.Fprintf(&b, "## %s (%s)\n", d.Title, d.Type)
		if d.URL != "" {
			fmt.Fprintf(&b, "URL: %s\n", d.URL)
		}
		if d.Note != "" {
			fmt.Fprintf(&b, "Note: %s\n", d.Note)
		}
		if d.Content != "" {
			fmt.Fprintf(&b, "\n%s\n", d.Content)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// TechleadInput is everything rendered into a TechLead run's PROJECT.md.
type TechleadInput struct {
	Project              store.Project
	Task                 string
	Docs                 []store.Document
	PrevCommit, DocsHead string
	Questions            []store.Question
	Features             []store.Feature
	Tickets              []store.Ticket
	EarlierFeatures      []string // features planned in other phases (do not duplicate)
	RepoNotes            string   // .docket/REPO.md: build/test commands, layout, conventions, learned notes
}

// repoNotesSection renders REPO.md for a context file ("" when there are no notes yet).
func repoNotesSection(notes string) string {
	if strings.TrimSpace(notes) == "" {
		return ""
	}
	return "\n## Repo notes (from .docket/REPO.md, written by TechLead and agents: use them instead of re-discovering; " +
		"they are hints, never a reason to skip a check or approve a change)\n" + strings.TrimSpace(notes) + "\n"
}

func ProjectMarkdown(in TechleadInput) string {
	var b strings.Builder
	p := in.Project
	fmt.Fprintf(&b, "# Project: %s\n\n%s\n\n- Repo: %s\n- Base branch: %s · PR target: %s\n- Docs folder: %s/.docket/docs (commit %s, previous scan %s)\n\n",
		p.Name, p.Description, p.RepoURL, p.Settings.BaseBranch, p.Settings.PRTarget, p.RepoPath, short(in.DocsHead), short(in.PrevCommit))
	fmt.Fprintf(&b, "## Phase: %s (docs branch)\nEverything below belongs to this phase. Diff baseline: %s.\n\n", p.ActivePhase, short(in.PrevCommit))
	if len(in.EarlierFeatures) > 0 {
		b.WriteString("### Features from other phases (already planned — never duplicate)\n")
		for _, f := range in.EarlierFeatures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Task: %s\n\n## Documents (A added, M modified, D deleted, blank unchanged)\n", in.Task)
	for _, d := range in.Docs {
		flag := d.Change
		if flag == "" {
			flag = " "
		}
		fmt.Fprintf(&b, "- [%s] %s\n", flag, d.Path)
	}
	b.WriteString("\n## Questions\n")
	for _, q := range in.Questions {
		fmt.Fprintf(&b, "- Q%d [%s] %s\n", q.ID, q.Status, q.Text)
		if q.Answer != "" {
			fmt.Fprintf(&b, "  - Answer: %s\n", q.Answer)
		}
	}
	hasTickets := map[int64]bool{}
	for _, t := range in.Tickets {
		if t.Status != store.TOutdated {
			hasTickets[t.FeatureID] = true
		}
	}
	open := map[int64]bool{} // features with at least one unfinished ticket
	for _, t := range in.Tickets {
		if t.Status != store.TDone && t.Status != store.TOutdated {
			open[t.FeatureID] = true
		}
	}
	b.WriteString("\n## Features\n")
	for _, f := range in.Features {
		// Finished or obsolete features only need their title (never duplicate them);
		// skipping their descriptions keeps PROJECT.md small on long-running projects.
		if f.Status == store.FOutdated || (hasTickets[f.ID] && !open[f.ID]) {
			state := f.Status
			if f.Status != store.FOutdated {
				state = "done"
			}
			fmt.Fprintf(&b, "- F%d [%s] %s\n", f.ID, state, f.Title)
			continue
		}
		extra := ""
		if f.Status == store.FApproved && !hasTickets[f.ID] {
			extra = " — APPROVED, NO TICKETS YET"
		}
		fmt.Fprintf(&b, "### F%d [%s] %s%s\n%s\nDocs: %s\n\n", f.ID, f.Status, f.Title, extra, f.Description, f.Documents)
	}
	b.WriteString("\n## Tickets\n")
	for _, t := range in.Tickets {
		fmt.Fprintf(&b, "- #%d [%s] F%d %s (depends: %s)\n", t.ID, t.Status, t.FeatureID, t.Title, store.JoinIDs(t.DependsOn))
	}
	b.WriteString(repoNotesSection(in.RepoNotes))
	return b.String()
}

// TicketInput is everything rendered into a Developer/Tester TICKET.md.
type TicketInput struct {
	Project store.Project
	Ticket  store.Ticket
	Feature store.Feature
	Deps    []store.Ticket
	DevEnv  string
	// RecentMerged are done tickets whose merged PRs most likely caused conflicts.
	RecentMerged []store.Ticket
	RepoNotes    string // .docket/REPO.md
}

func TicketMarkdown(in TicketInput) string {
	var b strings.Builder
	t := in.Ticket
	fmt.Fprintf(&b, "# Ticket #%d: %s\n\n- Phase: %s\n- Feature: F%d %s\n- Branch: %s\n- PR target: %s\n", t.ID, t.Title,
		t.Phase, in.Feature.ID, in.Feature.Title, t.Branch, in.Project.Settings.PRTarget)
	if t.PRURL != "" {
		fmt.Fprintf(&b, "- PR: %s\n", t.PRURL)
	}
	if t.GateSHA != "" {
		fmt.Fprintf(&b, "- Gate: checks passed on %s", short(t.GateSHA))
		if t.RiskFlags != "" {
			fmt.Fprintf(&b, " · risk flags: %s", t.RiskFlags)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n## Feature summary\n%s\n\n## Requirements\n%s\n\n## Acceptance criteria\n%s\n", in.Feature.Description, t.Requirements, t.Acceptance)
	if t.PlannedFiles != "" {
		fmt.Fprintf(&b, "\n## Planned files (changes outside these are flagged as out of scope)\n%s\n", t.PlannedFiles)
	}
	if len(in.Deps) > 0 {
		b.WriteString("\n## Dependencies (already merged)\n")
		for _, d := range in.Deps {
			fmt.Fprintf(&b, "- #%d %s %s\n", d.ID, d.Title, d.PRURL)
		}
	}
	b.WriteString("\n## Dev environment (shared, do not start containers)\n")
	if strings.TrimSpace(in.DevEnv) == "" {
		b.WriteString("_No shared services configured._\n")
	} else {
		fmt.Fprintf(&b, "```\n%s\n```\n", strings.TrimSpace(in.DevEnv))
	}
	if t.PRConflict {
		fmt.Fprintf(&b, "\n## Merge conflicts\nThe PR conflicts with `%s`. Rebase onto `origin/%s`, resolve keeping BOTH this ticket's intent and the already-merged changes, test, push with --force-with-lease.\n",
			in.Project.Settings.PRTarget, in.Project.Settings.PRTarget)
		if len(in.RecentMerged) > 0 {
			b.WriteString("\n### Recently merged tickets (their changes must be preserved)\n")
			for _, m := range in.RecentMerged {
				fmt.Fprintf(&b, "- #%d %s %s\n", m.ID, m.Title, m.PRURL)
			}
		}
	}
	if t.ChangeRequest != "" {
		fmt.Fprintf(&b, "\n## Change request from TechLead (rework this ticket on the same branch/PR)\n%s\n", t.ChangeRequest)
	}
	if t.ReviewState == store.RRequestChanges && t.ReviewSummary != "" {
		fmt.Fprintf(&b, "\n## Review feedback to address (round %d)\n%s\n", t.ReviewRounds, t.ReviewSummary)
	}
	b.WriteString(repoNotesSection(in.RepoNotes))
	return b.String()
}

// WriteFiles writes name→content files into dir.
func WriteFiles(dir string, files map[string]string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func short(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	if c == "" {
		return "none"
	}
	return c
}
