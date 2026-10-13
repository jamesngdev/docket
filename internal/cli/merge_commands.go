package cli

import (
	"fmt"
	"os"
	"strings"

	"docket/internal/daemon"
	"docket/internal/gh"
	"docket/internal/ipc"
	"docket/internal/paths"
	"docket/internal/store"
)

// ticketMerge squash-merges a Tester-approved PR. Agents may only call it
// when auto-merge is on; the daemon's PR poll then marks the ticket done.
func ticketMerge(db *store.DB, p paths.Project, a args, actor string) error {
	id, err := a.id(0)
	if err != nil {
		return err
	}
	proj, err := db.Project()
	if err != nil {
		return err
	}
	if os.Getenv("DOCKET_RUN") != "" && !proj.Settings.AutoMerge {
		return fmt.Errorf("auto-merge is off for this project; leave the PR for the user")
	}
	t, err := db.Ticket(id)
	if err != nil {
		return err
	}
	switch {
	case t.Status != store.TWaitReview || t.PRURL == "":
		return fmt.Errorf("ticket #%d is %s without an open PR; nothing to merge", id, t.Status)
	case t.ReviewState != store.RApprove:
		return fmt.Errorf("ticket #%d is not approved by the Tester (review: %s)", id, orNone(t.ReviewState))
	case t.LockedBy != "":
		return fmt.Errorf("ticket #%d is being worked on by %s", id, t.LockedBy)
	case t.PRConflict:
		return fmt.Errorf("ticket #%d's PR has merge conflicts", id)
	}
	if err := gh.MergePR(t.PRURL); err != nil {
		db.Log(actor, "merge of ticket #%d failed: %v", id, err)
		return err
	}
	db.Log(actor, "squash-merged PR of ticket #%d (%s)", id, t.PRURL)
	_ = daemon.Send(p, ipc.CmdCheckPRs, "") // mark done + clean up right away
	fmt.Printf("ticket #%d merged\n", id)
	return nil
}

// autoMergeCmd toggles the project's auto-merge setting.
func autoMergeCmd(a args, sub string) error {
	db, p, err := openDB(a)
	if err != nil {
		return err
	}
	defer db.Close()
	proj, err := db.Project()
	if err != nil {
		return err
	}
	switch sub {
	case "on", "off":
		proj.Settings.AutoMerge = sub == "on"
		if err := db.SaveSettings(proj.Settings); err != nil {
			return err
		}
		db.Log("user", "auto-merge %s", sub)
		kick(p)
		return nil
	case "status", "":
		fmt.Printf("auto-merge: %s\n", map[bool]string{true: "on", false: "off"}[proj.Settings.AutoMerge])
		return nil
	}
	return fmt.Errorf("usage: docket automerge on|off|status")
}

// oneLineText collapses whitespace for activity log entries.
func oneLineText(s string) string { return strings.Join(strings.Fields(s), " ") }
