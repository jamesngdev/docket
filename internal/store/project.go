package store

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Settings are per-project knobs stored as JSON on the project row.
type Settings struct {
	MaxParallelAgents int               `json:"max_parallel_agents"`
	BaseBranch        string            `json:"base_branch"`
	PRTarget          string            `json:"pr_target"`
	MaxReviewRounds   int               `json:"max_review_rounds"`
	Models            map[string]string `json:"models"`
	DiscordChannel    string            `json:"discord_channel,omitempty"` // Backend Support answers @mentions here
	AutoMerge         bool              `json:"auto_merge,omitempty"`      // TechLead reviews and squash-merges approved PRs
	// MergeReview is who decides on approved PRs when auto-merge is on:
	// "risky" (default, "") merges low-risk PRs with green CI directly and sends
	// only flagged ones to TechLead; "all" sends every PR to TechLead.
	MergeReview string `json:"merge_review,omitempty"`
}

// Merge review modes.
const (
	MergeReviewRisky = "risky"
	MergeReviewAll   = "all"
)

func DefaultSettings() Settings {
	return Settings{
		MaxParallelAgents: 3, BaseBranch: "develop", PRTarget: "develop", MaxReviewRounds: 2,
		Models: map[string]string{"techlead": "opus", "developer": "sonnet", "tester": "sonnet", "support": "sonnet"},
	}
}

// withDefaults fills zero values so old rows keep working after new settings are added.
func (s Settings) withDefaults() Settings {
	d := DefaultSettings()
	if s.MaxParallelAgents <= 0 {
		s.MaxParallelAgents = d.MaxParallelAgents
	}
	if s.BaseBranch == "" {
		s.BaseBranch = d.BaseBranch
	}
	if s.PRTarget == "" {
		s.PRTarget = d.PRTarget
	}
	if s.MaxReviewRounds <= 0 {
		s.MaxReviewRounds = d.MaxReviewRounds
	}
	if s.Models == nil {
		s.Models = map[string]string{}
	}
	for k, v := range d.Models {
		if s.Models[k] == "" {
			s.Models[k] = v
		}
	}
	return s
}

type Project struct {
	Name, Slug, Description        string
	RepoURL, RepoPath, DocsRepoURL string
	Settings                       Settings
	ActivePhase, PrevPhase         string // docs branches; see phases.go
	DocsCommit, PrevDocsCommit     string // last scan of the active phase and its diff baseline
	QueuedTask                     string // TechLead task waiting for the running TechLead
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases and replaces non-alphanumerics with dashes (max 40 chars).
func Slugify(s string) string {
	out := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "x"
	}
	return out
}

func (d *DB) CreateProject(p Project) error {
	if p.Slug == "" {
		p.Slug = Slugify(p.Name)
	}
	b, _ := json.Marshal(p.Settings.withDefaults())
	_, err := d.Exec(`INSERT INTO project(id,name,slug,description,repo_url,repo_path,docs_repo_url,settings,active_phase,created_at)
		VALUES(1,?,?,?,?,?,?,?,?,?)`, p.Name, p.Slug, p.Description, p.RepoURL, p.RepoPath, p.DocsRepoURL, string(b), p.ActivePhase, now())
	return err
}

func (d *DB) Project() (Project, error) {
	var p Project
	var settings string
	err := d.QueryRow(`SELECT name,slug,description,repo_url,repo_path,docs_repo_url,settings,queued_task,active_phase,prev_phase,
		COALESCE((SELECT docs_commit FROM phases WHERE name=project.active_phase),''),
		COALESCE((SELECT prev_docs_commit FROM phases WHERE name=project.active_phase),'')
		FROM project WHERE id=1`).
		Scan(&p.Name, &p.Slug, &p.Description, &p.RepoURL, &p.RepoPath, &p.DocsRepoURL, &settings, &p.QueuedTask,
			&p.ActivePhase, &p.PrevPhase, &p.DocsCommit, &p.PrevDocsCommit)
	if err != nil {
		return p, err
	}
	_ = json.Unmarshal([]byte(settings), &p.Settings)
	p.Settings = p.Settings.withDefaults()
	return p, nil
}

func (d *DB) SetQueuedTask(task string) error {
	_, err := d.Exec(`UPDATE project SET queued_task=? WHERE id=1`, task)
	return err
}
