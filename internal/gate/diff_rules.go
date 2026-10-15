package gate

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Change is one file of the diff against the PR target.
type Change struct {
	Status string // git name-status letter: A, M, D, R…
	Path   string
}

// secretPatterns are high-confidence credential formats: a hit blocks the gate.
var secretPatterns = map[string]*regexp.Regexp{
	"AWS access key":  regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	"private key":     regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	"Anthropic token": regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`),
	"GitHub token":    regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}`),
	"Slack token":     regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),
	"Google API key":  regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`),
}

// AllowSecretMarker on a line exempts it from the secret scan (test fixtures).
const AllowSecretMarker = "gate:allow-secret"

// suspiciousAssign is a likely hard-coded secret; only flagged (tests use fixtures).
var suspiciousAssign = regexp.MustCompile(`(?i)(password|passwd|secret|api_?key|access_?token)\s*[:=]\s*["'][^"'\s]{8,}["']`)

// blockingFiles returns problems with the changed file set that must be fixed.
func blockingFiles(changes []Change) []string {
	var out []string
	for _, c := range changes {
		base := path.Base(c.Path)
		switch {
		case strings.HasPrefix(c.Path, ".docket/"):
			out = append(out, c.Path+": docket data must never be committed")
		case c.Status != "D" && (base == ".env" || strings.HasPrefix(base, ".env.")) && !isEnvTemplate(base):
			out = append(out, c.Path+": env files must not be committed")
		}
	}
	return out
}

func isEnvTemplate(base string) bool {
	for _, s := range []string{".example", ".sample", ".template", ".dist"} {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	return false
}

// scanAddedLines checks the added lines of a unified diff (-U0) for secrets.
// It returns blocking hits and softer risk flags, never the secret itself.
func scanAddedLines(diff string) (block, risks []string) {
	file := ""
	seen := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			file = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
			continue
		}
		if !strings.HasPrefix(line, "+") || strings.Contains(line, AllowSecretMarker) {
			continue
		}
		for name, re := range secretPatterns {
			if key := file + name; re.MatchString(line) && !seen[key] {
				seen[key] = true
				block = append(block, fmt.Sprintf("%s: looks like a %s — remove it and use configuration "+
					"(a fake test fixture may carry a %q comment on the same line)", file, name, AllowSecretMarker))
			}
		}
		if key := file + "assign"; suspiciousAssign.MatchString(line) && !seen[key] {
			seen[key] = true
			risks = append(risks, "possible hard-coded secret in "+file)
		}
	}
	return block, risks
}

var (
	migrationRe = regexp.MustCompile(`(?i)(^|/)(migrations?|migrate)(/|$)|\.sql$|schema\.prisma$`)
	securityRe  = regexp.MustCompile(`(?i)(^|/|_|-)(auth|authn|authz|security|permissions?|rbac|acl|jwt|oauth|crypto)([/_.-]|$)`)
	testRe      = regexp.MustCompile(`(?i)(_test\.go$|\.(test|spec)\.[jt]sx?$|(^|/)(tests?|__tests__|spec)/|(^|/)test_[^/]*\.py$)`)
	depFiles    = map[string]bool{"go.mod": true, "go.sum": true, "package.json": true, "package-lock.json": true,
		"yarn.lock": true, "pnpm-lock.yaml": true, "requirements.txt": true, "pyproject.toml": true, "poetry.lock": true,
		"Pipfile": true, "Pipfile.lock": true, "Cargo.toml": true, "Cargo.lock": true, "Gemfile": true, "Gemfile.lock": true,
		"pom.xml": true, "build.gradle": true, "build.gradle.kts": true, "composer.json": true, "composer.lock": true}
)

// riskFlags derives why an approved PR still needs a TechLead look.
func riskFlags(changes []Change, diffLines, maxLines int, planned string) []string {
	var mig, sec, deps, ci, delTests, outside []string
	for _, c := range changes {
		p := c.Path
		switch {
		case migrationRe.MatchString(p):
			mig = append(mig, p)
		case securityRe.MatchString(p):
			sec = append(sec, p)
		}
		if depFiles[path.Base(p)] {
			deps = append(deps, p)
		}
		if strings.HasPrefix(p, ".github/workflows/") || strings.HasPrefix(p, ".gitlab-ci") {
			ci = append(ci, p)
		}
		if c.Status == "D" && testRe.MatchString(p) {
			delTests = append(delTests, p)
		}
		if planned != "" && !testRe.MatchString(p) && !depFiles[path.Base(p)] && !InPlanned(p, planned) {
			outside = append(outside, p)
		}
	}
	var out []string
	add := func(label string, files []string) {
		if len(files) > 0 {
			out = append(out, fmt.Sprintf("%s: %s", label, abbreviate(files)))
		}
	}
	add("migration", mig)
	add("auth/security", sec)
	add("dependencies", deps)
	add("CI config", ci)
	add("deleted tests", delTests)
	add("outside planned files", outside)
	if diffLines > maxLines {
		out = append(out, fmt.Sprintf("large diff: %d lines (limit %d)", diffLines, maxLines))
	}
	return out
}

// InPlanned reports whether p matches one of the comma-separated planned
// entries: an exact path, a directory prefix, a glob (*.go) or a prefix/** glob.
func InPlanned(p, planned string) bool {
	for _, e := range strings.Split(planned, ",") {
		e = strings.TrimPrefix(strings.TrimSpace(e), "./")
		if e == "" {
			continue
		}
		if i := strings.Index(e, "**"); i >= 0 {
			if strings.HasPrefix(p, e[:i]) {
				return true
			}
			continue
		}
		if ok, _ := path.Match(e, p); ok {
			return true
		}
		if p == e || strings.HasPrefix(p, strings.TrimSuffix(e, "/")+"/") {
			return true
		}
	}
	return false
}

func abbreviate(files []string) string {
	if len(files) <= 3 {
		return strings.Join(files, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(files[:3], ", "), len(files)-3)
}

// prBodyProblems checks the PR body follows the open-pr template.
func prBodyProblems(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil // gh could not read it; the Tester still checks the body
	}
	var out []string
	lower := strings.ToLower(body)
	for _, h := range []string{"acceptance", "test"} {
		found := false
		for _, line := range strings.Split(lower, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.Contains(line, h) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("PR body needs a \"## %s…\" section (see skill open-pr)", strings.ToUpper(h[:1])+h[1:]))
		}
	}
	return out
}
