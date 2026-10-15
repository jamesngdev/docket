// Package gate is the mechanical check a Developer's change must pass before
// its ticket can enter review: committed and pushed, no forbidden files or
// secrets, the project's verify commands green. It also derives risk flags
// that decide whether an approved PR may be auto-merged without TechLead.
package gate

import (
	"errors"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultMaxDiffLines flags larger changes as risky when checks.yaml sets no limit.
const DefaultMaxDiffLines = 600

// Budget is the time all checks of one gate run may take together: it stays
// below the 10-minute cap of an agent's shell tool, so a slow suite fails with
// a clear message instead of the agent's command being killed and retried.
const Budget = 9 * time.Minute

// Check is one verify command run from the worktree root with `sh -c`.
type Check struct {
	Name    string `yaml:"name"`
	Run     string `yaml:"run"`
	Timeout string `yaml:"timeout,omitempty"` // Go duration, e.g. "5m"; capped by what is left of Budget
}

// Config is .docket/checks.yaml, written by TechLead (skill write-repo-notes).
type Config struct {
	Checks       []Check `yaml:"checks"`
	MaxDiffLines int     `yaml:"max_diff_lines,omitempty"`
}

// LoadConfig reads checks.yaml; a missing file is an empty config.
func LoadConfig(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.MaxDiffLines <= 0 {
		c.MaxDiffLines = DefaultMaxDiffLines
	}
	return c, nil
}

// timeout is the check's own limit, never more than left of the gate budget.
func (c Check) timeout(left time.Duration) time.Duration {
	if d, err := time.ParseDuration(c.Timeout); err == nil && d > 0 && d < left {
		return d
	}
	return left
}
