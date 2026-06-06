package evalqualitycheck

// Suite-level adversarial-diversity check. Where Check() asks "is THIS eval a
// tautology?", CheckDiversity asks "does this SET of evals cover the negative
// and edge cases, or is it all happy-path?" — applying Google's adversarial-
// testing diversity requirement (skills/adversarial-testing/SKILL.md §6).
//
// Heuristics are deterministic and keyword-based: a complement to mutation
// testing (mutate-eval.sh kill-rate ≥0.8), NOT a replacement. They favor
// precision over recall — a false HALT (blocking honest work) is worse than a
// missed WARN, so negative-case detection keys on shell-level negation
// constructs, not English words like "fail" (which appear in doc-grep evals).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DiversityLevel classifies a suite of evals for adversarial diversity.
type DiversityLevel int

const (
	DiversityPass DiversityLevel = 0 // diverse enough (or nothing to assess)
	DiversityWarn DiversityLevel = 1 // weak diversity; advisory
	DiversityHalt DiversityLevel = 2 // a cohesive suite with zero negative cases
)

// maxCohesiveSuiteSize bounds the eval count for which a zero-negative-case
// suite is a hard HALT. A single cycle authors a handful of evals; beyond this
// the directory is an accumulated archive (e.g. .evolve/evals/ holds every
// historical task), not one authoring unit — so the floor downgrades to an
// advisory WARN rather than blocking on legacy corpus.
const maxCohesiveSuiteSize = 12

// DiversityOptions configures CheckDiversity. EvalDir is required.
type DiversityOptions struct {
	EvalDir string // directory of <slug>.md eval files
	Slug    string // optional: only consider files whose name contains this substring
}

// EvalDiversity is the per-file diversity fingerprint.
type EvalDiversity struct {
	Path        string
	HasNegative bool // ≥1 command asserting a rejection / non-zero exit
	HasEdge     bool // ≥1 command exercising empty/boundary/malformed input
}

// DiversityResult is the suite-level verdict + per-file breakdown.
type DiversityResult struct {
	EvalDir           string
	EvalCount         int // files with ≥1 parsed command
	NegativeCaseCount int // files with ≥1 negative case
	EdgeCaseCount     int // files with ≥1 edge/OOD case
	PositiveOnlyCount int // files with neither negative nor edge cases
	Level             DiversityLevel
	Reasons           []string
	Files             []EvalDiversity
}

var (
	// negativeCaseRE matches shell-level negation / expected-failure
	// constructs — NOT the English word "fail" (which appears in doc-grep
	// evals like `grep -q "failure pattern"`). Precision over recall: `!=`
	// is matched ONLY inside a test bracket `[ ... != ... ]`, not as a bare
	// token (else `grep -q "a != b"` would be a false negative-case match).
	negativeCaseRE = regexp.MustCompile(`(^|\s)!\s|\bexit\s+1\b|-ne\s+0|\[[^]]*!=[^]]*\]|\b(assert|expect|should|must)[_-]?(fail|error|reject|not)\b`)
	// edgeCaseRE matches boundary / out-of-distribution input indicators.
	edgeCaseRE = regexp.MustCompile(`\binvalid\b|\bmissing\b|\bcorrupt(ed)?\b|\bmalformed\b|\boverflow\b|\bboundary\b|\bempty\b|""|''`)
)

// CheckDiversity reads every <slug>.md eval in opts.EvalDir (skipping
// underscore-prefixed meta/canary files and files with no commands) and scores
// the suite for adversarial diversity. The directory-not-found case is an
// error; an empty/command-free directory is DiversityPass (nothing to assess).
func CheckDiversity(opts DiversityOptions) (DiversityResult, error) {
	if opts.EvalDir == "" {
		return DiversityResult{}, fmt.Errorf("evalqualitycheck: EvalDir required")
	}
	entries, err := os.ReadDir(opts.EvalDir)
	if err != nil {
		return DiversityResult{}, fmt.Errorf("evalqualitycheck: read dir %s: %w", opts.EvalDir, err)
	}

	res := DiversityResult{EvalDir: opts.EvalDir}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if strings.HasPrefix(name, "_") {
			continue // meta / canary / example evals are not authoring units
		}
		if opts.Slug != "" && !strings.Contains(name, opts.Slug) {
			continue
		}
		path := filepath.Join(opts.EvalDir, name)
		f, err := os.Open(path)
		if err != nil {
			return DiversityResult{}, fmt.Errorf("evalqualitycheck: open %s: %w", path, err)
		}
		cmds, scanErr := scanBashCommands(f)
		_ = f.Close()
		if scanErr != nil {
			return DiversityResult{}, fmt.Errorf("evalqualitycheck: scan %s: %w", path, scanErr)
		}
		if len(cmds) == 0 {
			continue
		}
		ed := EvalDiversity{Path: path}
		for _, c := range cmds {
			if negativeCaseRE.MatchString(c) {
				ed.HasNegative = true
			}
			if edgeCaseRE.MatchString(c) {
				ed.HasEdge = true
			}
		}
		res.EvalCount++
		if ed.HasNegative {
			res.NegativeCaseCount++
		}
		if ed.HasEdge {
			res.EdgeCaseCount++
		}
		if !ed.HasNegative && !ed.HasEdge {
			res.PositiveOnlyCount++
		}
		res.Files = append(res.Files, ed)
	}

	res.Level, res.Reasons = scoreDiversity(res)
	return res, nil
}

// scoreDiversity maps the suite metrics to a level + human reasons. The gate
// keys on the presence of a NEGATIVE case — the highest-precision adversarial
// signal (a shell-level rejection construct, unlikely to appear by accident).
// A cohesive-size suite (3..maxCohesiveSuiteSize) with zero negatives is a hard
// HALT; a smaller or archive-scale zero-negative suite is an advisory WARN;
// any suite with ≥1 negative case PASSes. Edge-case counts are reported but do
// not gate (keyword-based, so noisier).
func scoreDiversity(d DiversityResult) (DiversityLevel, []string) {
	switch {
	case d.EvalCount == 0:
		return DiversityPass, []string{"no evals with commands to assess"}
	case d.NegativeCaseCount > 0:
		return DiversityPass, nil
	case d.EvalCount >= 3 && d.EvalCount <= maxCohesiveSuiteSize:
		return DiversityHalt, []string{
			fmt.Sprintf("cohesive suite of %d evals has zero negative cases — add a rejection/failure test", d.EvalCount),
		}
	default:
		return DiversityWarn, []string{
			fmt.Sprintf("no negative cases across %d evals (suite too small or archive-scale for hard HALT; advisory)", d.EvalCount),
		}
	}
}
