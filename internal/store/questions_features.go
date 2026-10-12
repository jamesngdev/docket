package store

// Question statuses.
const (
	QOpen     = "open"
	QAnswered = "answered"
	QSkipped  = "skipped"
)

// Feature statuses.
const (
	FDraft    = "draft"
	FApproved = "approved"
	FOutdated = "outdated"
)

type Question struct {
	ID                           int64
	Text, Answer, Status, Source string
}

type Feature struct {
	ID                                    int64
	Title, Description, Documents, Status string
}

func (d *DB) AddQuestion(text, source string) (int64, error) {
	r, err := d.Exec(`INSERT INTO questions(text, source, created_at, phase) VALUES(?,?,?,`+activePhaseSQL+`)`, text, source, now())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// Questions returns open questions first, then answered/skipped.
func (d *DB) Questions() ([]Question, error) {
	rows, err := d.Query(`SELECT id,text,answer,status,source FROM questions WHERE phase=` + activePhaseSQL + `
		ORDER BY CASE status WHEN 'open' THEN 0 ELSE 1 END, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Question
	for rows.Next() {
		var q Question
		if err := rows.Scan(&q.ID, &q.Text, &q.Answer, &q.Status, &q.Source); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// AnswerQuestion stores an answer; an empty answer re-opens the question.
func (d *DB) AnswerQuestion(id int64, answer string) error {
	status := QAnswered
	if answer == "" {
		status = QOpen
	}
	_, err := d.Exec(`UPDATE questions SET answer=?, status=? WHERE id=?`, answer, status, id)
	return err
}

func (d *DB) SkipQuestion(id int64) error {
	_, err := d.Exec(`UPDATE questions SET status=? WHERE id=?`, QSkipped, id)
	return err
}

func (d *DB) AddFeature(title, desc, docs string) (int64, error) {
	r, err := d.Exec(`INSERT INTO features(title,description,documents,created_at,phase) VALUES(?,?,?,?,`+activePhaseSQL+`)`, title, desc, docs, now())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

func (d *DB) Features() ([]Feature, error) {
	rows, err := d.Query(`SELECT id,title,description,documents,status FROM features WHERE phase=` + activePhaseSQL + `
		ORDER BY CASE status WHEN 'draft' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Feature
	for rows.Next() {
		var f Feature
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.Documents, &f.Status); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Feature returns one feature regardless of phase.
func (d *DB) Feature(id int64) (Feature, error) {
	var f Feature
	err := d.QueryRow(`SELECT id,title,description,documents,status FROM features WHERE id=?`, id).
		Scan(&f.ID, &f.Title, &f.Description, &f.Documents, &f.Status)
	return f, err
}

func (d *DB) SetFeatureStatus(id int64, status string) error {
	_, err := d.Exec(`UPDATE features SET status=? WHERE id=?`, status, id)
	return err
}

// UpdateFeature edits a single text column (title or description).
func (d *DB) UpdateFeature(id int64, title, desc string) error {
	_, err := d.Exec(`UPDATE features SET title=?, description=? WHERE id=?`, title, desc, id)
	return err
}
