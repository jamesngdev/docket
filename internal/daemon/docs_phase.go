package daemon

import (
	"fmt"
	"os"

	"docket/internal/agent"
	"docket/internal/docscan"
	"docket/internal/gitops"
	"docket/internal/store"
)

// rescanDocs pulls the docs repo, diffs it against the last scan and starts
// the matching TechLead task (bootstrap on first scan, docdiff afterwards).
func (d *Daemon) rescanDocs() error {
	p, err := d.project()
	if err != nil {
		return err
	}
	docs := d.paths.Docs()
	if _, err := os.Stat(docs); os.IsNotExist(err) {
		if p.DocsRepoURL == "" {
			return fmt.Errorf("no docs repo configured")
		}
		if err := gitops.Clone(p.DocsRepoURL, docs); err != nil {
			return err
		}
	}
	_ = gitops.Fetch(docs)
	if p.ActivePhase != "" {
		if err := gitops.CheckoutBase(docs, p.ActivePhase); err != nil {
			return fmt.Errorf("checkout phase %s: %w", p.ActivePhase, err)
		}
	}
	if err := gitops.Pull(docs); err != nil {
		d.db.Log("docs", "pull failed, scanning local copy: %v", err)
	}
	head, _ := gitops.Head(docs)
	hashes, err := docscan.Hashes(docs)
	if err != nil {
		return err
	}
	first := !d.db.AnyPhaseScanned()
	prev, baseline := []store.Document(nil), p.DocsCommit
	if p.DocsCommit == "" && p.PrevPhase != "" {
		// First scan of a new phase: diff against the phase we came from, so
		// TechLead only works on what this phase adds or changes.
		prev, err = d.db.DocumentsOf(p.PrevPhase)
		baseline = d.db.PhaseCommit(p.PrevPhase)
	} else {
		prev, err = d.db.Documents()
	}
	if err != nil {
		return err
	}
	list, changed := docscan.Diff(prev, hashes, head)
	if err := d.db.ReplaceDocuments(list); err != nil {
		return err
	}
	if err := d.db.SetDocsCommit(head, baseline); err != nil {
		return err
	}
	d.db.Log("docs", "phase %s: scanned %d docs @ %s, %d changed", p.ActivePhase, len(list), short(head), changed)
	switch {
	case first:
		return d.startOrQueueTechlead(agent.TaskBootstrap)
	case changed > 0:
		return d.startOrQueueTechlead(agent.TaskDocDiff)
	}
	return nil
}

// switchPhase makes a docs branch the active phase and scans it.
func (d *Daemon) switchPhase(name string) error {
	if name == "" {
		return fmt.Errorf("phase name required")
	}
	if d.runningRole(agent.RoleTechLead) > 0 {
		return fmt.Errorf("TechLead is running; switch phase after it finishes")
	}
	_ = gitops.Fetch(d.paths.Docs()) // the branch may have been pushed after our last fetch
	branches, err := gitops.Branches(d.paths.Docs())
	if err != nil {
		return err
	}
	found := false
	for _, b := range branches {
		found = found || b == name
	}
	if !found {
		return fmt.Errorf("docs repo has no branch %q", name)
	}
	if err := d.db.SetActivePhase(name); err != nil {
		return err
	}
	d.db.Log("phase", "active phase → %s", name)
	return d.rescanDocs()
}
