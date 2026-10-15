package cli

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"docket/internal/daemon"
	"docket/internal/ipc"
	"docket/internal/store"
)

// write handles every state-changing agent command, then nudges the daemon.
func write(cmd, sub string, a args) error {
	db, p, err := openDB(a)
	if err != nil {
		return err
	}
	defer db.Close()
	actor := cmd
	if rid := os.Getenv("DOCKET_RUN"); rid != "" {
		actor, _, _ = strings.Cut(rid, "-") // role prefix of the run id
	}
	switch cmd + " " + sub {
	case "question add":
		text, err := a.need("text")
		if err != nil {
			return err
		}
		id, err := db.AddQuestion(text, a.flags["source"])
		if err != nil {
			return err
		}
		fmt.Println(id)
	case "question answer":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		text, err := a.need("text")
		if err != nil {
			return err
		}
		if err := db.AnswerQuestion(id, strings.TrimSpace(text)); err != nil {
			return err
		}
		db.Log(actor, "Q%d answered", id)
	case "feature add":
		title, err := a.need("title")
		if err != nil {
			return err
		}
		desc, err := a.text("description")
		if err != nil {
			return err
		}
		id, err := db.AddFeature(title, desc, a.flags["docs"])
		if err != nil {
			return err
		}
		db.Log("techlead", "feature F%d %s", id, title)
		fmt.Println(id)
	case "feature outdated":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		if err := db.SetFeatureStatus(id, store.FOutdated); err != nil {
			return err
		}
		db.Log("techlead", "feature F%d marked outdated", id)
	case "ticket add":
		return ticketAdd(db, a)
	case "ticket update":
		return ticketUpdate(db, a, actor)
	case "ticket redo":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		reason, err := a.need("reason")
		if err != nil {
			return err
		}
		t, err := db.Ticket(id)
		if err != nil {
			return err
		}
		if t.Status == store.TDone {
			return fmt.Errorf("ticket #%d is merged: create a follow-up ticket with --depends %d instead", id, id)
		}
		if err := db.UpdateTicketFields(id, map[string]any{"change_request": reason}); err != nil {
			return err
		}
		if err := daemon.EnsureRunning(p); err != nil {
			return err
		}
		if err := daemon.Send(p, ipc.CmdRedo, strconv.FormatInt(id, 10)); err != nil {
			return err
		}
		db.Log(actor, "ticket #%d redo requested", id)
		fmt.Printf("ticket #%d will be reworked on its branch %s\n", id, t.Branch)
		return nil
	case "ticket merge":
		return ticketMerge(db, p, a, actor)
	case "ticket flag":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		reason, err := a.need("reason")
		if err != nil {
			return err
		}
		if err := db.UpdateTicketFields(id, map[string]any{"needs_user": 1}); err != nil {
			return err
		}
		db.Log(actor, "ticket #%d needs you: %s", id, oneLineText(reason))
	case "ticket outdated":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		t, err := db.Ticket(id)
		if err != nil {
			return err
		}
		if t.Status == store.TDone || t.Status == store.TProcessing || t.Status == store.TWaitReview {
			db.Log("techlead", "ticket #%d is %s; NOT marked outdated (user decides)", id, t.Status)
			return fmt.Errorf("ticket #%d is %s and cannot be marked outdated automatically; the user was warned", id, t.Status)
		}
		if err := db.SetTicketStatus(id, store.TOutdated); err != nil {
			return err
		}
		db.Log("techlead", "ticket #%d marked outdated", id)
	case "ticket status":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		if len(a.pos) < 2 || !slices.Contains(validTicketStatus, a.pos[1]) {
			return fmt.Errorf("status must be one of %v", validTicketStatus)
		}
		t, err := db.Ticket(id)
		if err != nil {
			return err
		}
		if rid := os.Getenv("DOCKET_RUN"); rid != "" && t.LockedBy != rid {
			return fmt.Errorf("ticket #%d is not assigned to run %s", id, rid)
		}
		fields := map[string]any{"status": a.pos[1]}
		if pr := a.flags["pr"]; pr != "" {
			fields["pr_url"] = pr
		}
		if a.pos[1] == store.TWaitReview && a.flags["pr"] == "" && t.PRURL == "" {
			return fmt.Errorf("wait_review requires --pr <url>")
		}
		rid := os.Getenv("DOCKET_RUN")
		if a.pos[1] == store.TWaitReview && strings.HasPrefix(rid, "developer-") {
			pr := a.flags["pr"]
			if pr == "" {
				pr = t.PRURL
			}
			gated, err := waitReviewGate(db, p, t, rid, pr)
			if err != nil {
				return err
			}
			for k, v := range gated {
				fields[k] = v
			}
		}
		if rid != "" {
			// The gate can take minutes: only write if this run still owns the ticket
			// (the user may have stopped it, or a redo may have reset it meanwhile).
			ok, err := db.UpdateTicketFieldsIfLocked(id, rid, fields)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("ticket #%d is no longer assigned to run %s; nothing recorded", id, rid)
			}
		} else if err := db.UpdateTicketFields(id, fields); err != nil {
			return err
		}
		db.Log(actor, "ticket #%d → %s %s", id, a.pos[1], a.flags["pr"])
	case "review submit":
		id, err := a.id(0)
		if err != nil {
			return err
		}
		result := a.flags["result"]
		if result != store.RApprove && result != store.RRequestChanges {
			return fmt.Errorf("--result must be approve or request_changes")
		}
		summary, err := a.text("summary")
		if err != nil {
			return err
		}
		if err := db.UpdateTicketFields(id, map[string]any{"review_state": result, "review_summary": summary}); err != nil {
			return err
		}
		db.Log("tester", "ticket #%d review: %s", id, result)
	case "note add":
		if err := noteAdd(p, a, actor); err != nil {
			return err
		}
	case "run done":
		rid, err := runID()
		if err != nil {
			return err
		}
		summary, _ := a.text("summary")
		if err := db.MarkCallback(rid, summary); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command %q %q\n\n%s", cmd, sub, usage)
	}
	kick(p)
	return nil
}

// ticketUpdate edits text/dependencies of a ticket that is not merged yet.
func ticketUpdate(db *store.DB, a args, actor string) error {
	id, err := a.id(0)
	if err != nil {
		return err
	}
	t, err := db.Ticket(id)
	if err != nil {
		return err
	}
	if t.Status == store.TDone || t.Status == store.TOutdated {
		return fmt.Errorf("ticket #%d is %s and cannot be edited: create a follow-up ticket (--depends %d)", id, t.Status, id)
	}
	fields := map[string]any{}
	for _, f := range []struct{ flag, col string }{{"title", "title"}, {"requirements", "requirements"}, {"acceptance", "acceptance"}} {
		v, err := a.text(f.flag)
		if err != nil {
			return err
		}
		if strings.TrimSpace(v) != "" {
			fields[f.col] = strings.TrimSpace(v)
		}
	}
	if acc, ok := fields["acceptance"].(string); ok {
		if err := validateAcceptance(acc); err != nil {
			return err
		}
	}
	if err := ticketExtras(a, fields); err != nil {
		return err
	}
	if raw, ok := a.flags["depends"]; ok {
		deps := store.ParseIDs(raw)
		for _, dep := range deps {
			if dep == id {
				return fmt.Errorf("ticket #%d cannot depend on itself", id)
			}
			if _, err := db.Ticket(dep); err != nil {
				return fmt.Errorf("dependency #%d does not exist", dep)
			}
		}
		fields["depends_on"] = store.JoinIDs(deps)
	}
	if len(fields) == 0 {
		return fmt.Errorf("nothing to update: pass --title, --requirements, --acceptance, --depends, --files or --model")
	}
	if err := db.UpdateTicketFields(id, fields); err != nil {
		return err
	}
	db.Log(actor, "ticket #%d updated (%d fields)", id, len(fields))
	return nil
}

func ticketAdd(db *store.DB, a args) error {
	title, err := a.need("title")
	if err != nil {
		return err
	}
	fid, err := strconv.ParseInt(a.flags["feature"], 10, 64)
	if err != nil {
		return fmt.Errorf("--feature <id> is required")
	}
	req, err := a.need("requirements")
	if err != nil {
		return err
	}
	acc, err := a.need("acceptance")
	if err != nil {
		return err
	}
	if err := validateAcceptance(acc); err != nil {
		return err
	}
	extras := map[string]any{}
	if err := ticketExtras(a, extras); err != nil {
		return err
	}
	files, _ := extras["planned_files"].(string)
	if files == "" {
		return fmt.Errorf("--files is required: comma-separated paths/globs the ticket will change (e.g. internal/orders/,migrations/*.sql)")
	}
	model, _ := extras["model"].(string)
	deps := store.ParseIDs(a.flags["depends"])
	for _, d := range deps {
		if _, err := db.Ticket(d); err != nil {
			return fmt.Errorf("dependency #%d does not exist", d)
		}
	}
	id, err := db.AddTicket(store.Ticket{FeatureID: fid, Title: title, Requirements: req, Acceptance: acc, DependsOn: deps,
		PlannedFiles: files, Model: model})
	if err != nil {
		return err
	}
	db.Log("techlead", "ticket #%d %s", id, title)
	fmt.Println(id)
	return nil
}
