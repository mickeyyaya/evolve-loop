package evalqualitycheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DiversityLevel classifies a suite of evals for adversarial diversity.
type DiversityLevel int

// Diversity levels in rising severity.
const (
	DiversityPass DiversityLevel = 0 // diverse enough (or nothing to assess)
	DiversityWarn DiversityLevel = 1 // weak diversity; advisory
	DiversityHalt DiversityLevel = 2 // a cohesive suite with zero negative cases
)

// maxCohesiveSuiteSize is the largest suite treated as one authoring unit; a larger
// directory is an accumulated archive, so a zero-negative result there only WARNs.
const maxCohesiveSuiteSize = 12

// DiversityOptions configures CheckDiversity. EvalDir is required.
type DiversityOptions struct {
	EvalDir string
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
	// Precision over recall: only shell negation constructs count, never English words like "fail",
	// and `!=` counts only inside a test bracket.
	negativeCaseRE = regexp.MustCompile(`(^|\s)!\s|\bexit\s+1\b|-ne\s+0|\[[^]]*!=[^]]*\]|\b(assert|expect|should|must)[_-]?(fail|error|reject|not)\b`)
	edgeCaseRE     = regexp.MustCompile(`\binvalid\b|\bmissing\b|\bcorrupt(ed)?\b|\bmalformed\b|\boverflow\b|\bboundary\b|\bempty\b|""|''`)
)

// CheckDiversity scores the .md evals in opts.EvalDir for adversarial diversity; a suite with no commands passes.
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

// scoreDiversity gates on negative cases only; edge-case detection is keyword-based and too noisy to gate.
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
