package store

import (
	"strconv"
	"strings"
)

// Ticket statuses (see REQUIREMENT.md §2).
const (
	TBacklog    = "backlog"
	TPending    = "pending"
	TProcessing = "processing"
	TWaitReview = "wait_review"
	TDone       = "done"
	TOutdated   = "outdated"
)

// Review states written by the Tester.
const (
	RApprove        = "approve"
	RRequestChanges = "request_changes"
	RError          = "error"
)

type Ticket struct {
	ID, FeatureID                   int64
	Title, Requirements, Acceptance string
	DependsOn                       []int64
	Status, Branch, PRURL, LockedBy string
	ReviewState, ReviewSummary      string
	ReviewRounds                    int
	NeedsUser                       bool
	Phase                           string // docs branch the ticket was planned in
	ChangeRequest                   string // why TechLead asked to redo the ticket (ticket chat)
	PRConflict                      bool   // GitHub reports the PR as CONFLICTING with its base
	ConflictAttempts                int    // auto-resolve runs since the PR was last mergeable
}

const ticketCols = `id,feature_id,title,requirements,acceptance,depends_on,status,branch,pr_url,locked_by,
	review_state,review_summary,review_rounds,needs_user,phase,change_request,pr_conflict,conflict_attempts`

func scanTicket(s interface{ Scan(...any) error }) (Ticket, error) {
	var t Ticket
	var deps string
	err := s.Scan(&t.ID, &t.FeatureID, &t.Title, &t.Requirements, &t.Acceptance, &deps, &t.Status, &t.Branch,
		&t.PRURL, &t.LockedBy, &t.ReviewState, &t.ReviewSummary, &t.ReviewRounds, &t.NeedsUser, &t.Phase, &t.ChangeRequest, &t.PRConflict, &t.ConflictAttempts)
	t.DependsOn = ParseIDs(deps)
	return t, err
}

// ParseIDs parses "1, #2,3" into ids, ignoring junk.
func ParseIDs(s string) []int64 {
	var out []int64
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if n, err := strconv.ParseInt(strings.TrimPrefix(p, "#"), 10, 64); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func JoinIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func (d *DB) AddTicket(t Ticket) (int64, error) {
	r, err := d.Exec(`INSERT INTO tickets(feature_id,title,requirements,acceptance,depends_on,created_at,updated_at,phase)
		VALUES(?,?,?,?,?,?,?,`+activePhaseSQL+`)`, t.FeatureID, t.Title, t.Requirements, t.Acceptance, JoinIDs(t.DependsOn), now(), now())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// Tickets returns the active phase's tickets.
func (d *DB) Tickets() ([]Ticket, error) {
	return d.queryTickets(`SELECT ` + ticketCols + ` FROM tickets WHERE phase=` + activePhaseSQL + ` ORDER BY id`)
}

// AllTickets returns tickets of every phase (dependency status, PR polling, in-flight reviews).
func (d *DB) AllTickets() ([]Ticket, error) {
	return d.queryTickets(`SELECT ` + ticketCols + ` FROM tickets ORDER BY id`)
}

func (d *DB) queryTickets(q string) ([]Ticket, error) {
	rows, err := d.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Ticket
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) Ticket(id int64) (Ticket, error) {
	return scanTicket(d.QueryRow(`SELECT `+ticketCols+` FROM tickets WHERE id=?`, id))
}

// SetTicketStatus changes status; leaving `processing` always releases the lock.
func (d *DB) SetTicketStatus(id int64, status string) error {
	q := `UPDATE tickets SET status=?, updated_at=? WHERE id=?`
	if status != TProcessing {
		q = `UPDATE tickets SET status=?, locked_by='', updated_at=? WHERE id=?`
	}
	_, err := d.Exec(q, status, now(), id)
	return err
}

// UpdateTicketFields sets arbitrary whitelisted columns on a ticket.
func (d *DB) UpdateTicketFields(id int64, fields map[string]any) error {
	allowed := map[string]bool{"title": true, "requirements": true, "acceptance": true, "depends_on": true,
		"branch": true, "pr_url": true, "locked_by": true, "review_state": true, "review_summary": true,
		"review_rounds": true, "needs_user": true, "status": true, "change_request": true,
		"pr_conflict": true, "conflict_attempts": true}
	sets, args := []string{"updated_at=?"}, []any{now()}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		sets = append(sets, k+"=?")
		args = append(args, v)
	}
	args = append(args, id)
	_, err := d.Exec(`UPDATE tickets SET `+strings.Join(sets, ",")+` WHERE id=?`, args...)
	return err
}

// Dispatchable returns pending tickets whose dependencies are all done,
// excluding tickets flagged for the user. Pure function for easy testing.
func Dispatchable(all []Ticket, phase string) []Ticket {
	status := map[int64]string{}
	for _, t := range all {
		status[t.ID] = t.Status
	}
	var out []Ticket
	for _, t := range all {
		if t.Status != TPending || t.NeedsUser || t.LockedBy != "" || t.Phase != phase {
			continue
		}
		ok := true
		for _, dep := range t.DependsOn {
			if status[dep] != TDone {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}
