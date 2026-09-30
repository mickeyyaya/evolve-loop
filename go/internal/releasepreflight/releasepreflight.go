// Package releasepreflight is the read-only gate a release runs before any
// mutating step. See docs/architecture/packages/internal-releasepreflight.md.
package releasepreflight

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"

	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

var (
	ErrCheckFailed = errors.New("releasepreflight: check failed")
)

const MaxAuditAge = 7 * 24 * time.Hour

type Options struct {
	Target     string
	RepoRoot   string
	DryRun     bool
	SkipTests  bool
	StrictPass bool
	Stderr     io.Writer

	PluginJSONPath string
	LedgerPath     string

	AllowRedCI bool

	Now              func() time.Time
	GitClean         func(repoRoot string) (bool, error)
	CurrentBranch    func(repoRoot string) (string, error)
	GateTestRunner   func(repoRoot string, suite string) error
	NameGuard        func(repoRoot string) ([]naminguard.Violation, error)
	SimulationRunner func(repoRoot string) error
	CIConclusion     func(repoRoot string) (CIRunStatus, error)
	HeadSHA          func(repoRoot string) (string, error)
}

type CIRunStatus struct {
	Conclusion string
	RunURL     string
}

const ciConclusionUnavailable = ""

type Result struct {
	StepsPassed     int
	StepsTotal      int
	CurrentVersion  string
	AuditArtifact   string
	AuditVerdict    string
	AuditAge        time.Duration
	PhantomEntries  int
	GateTestsPassed int

	SimulationAdvisoryOK *bool

	CIConclusion string
	CIOverridden bool
}

var DefaultGateTestSuites = []string{
	"./internal/guards/...",
	"./internal/phases/ship/...",
}

var semverRE = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)([+-].*)?$`)

func ParseSemver(v string) (int, int, int, bool) {
	m := semverRE.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, 0, false
	}
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	pat, _ := strconv.Atoi(m[3])
	return maj, min, pat, true
}

func SemverGT(a, b string) bool {
	a1, a2, a3, okA := ParseSemver(a)
	b1, b2, b3, okB := ParseSemver(b)
	if !okA || !okB {
		return false
	}
	if a1 != b1 {
		return a1 > b1
	}
	if a2 != b2 {
		return a2 > b2
	}
	return a3 > b3
}

var versionFieldRE = regexp.MustCompile(`"version"[[:space:]]*:[[:space:]]*"([^"]*)"`)

func ExtractJSONVersion(jsonPath string) (string, error) {
	body, err := os.ReadFile(jsonPath)
	if err != nil {
		return "", err
	}
	m := versionFieldRE.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return "", fmt.Errorf("no version field in %s", jsonPath)
	}
	return m[1], nil
}

const gitDiffQuietDirtyExitCode = 1

func defaultGitClean(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--quiet", "HEAD")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == gitDiffQuietDirtyExitCode {
			return false, nil
		}
		return false, fmt.Errorf("git diff failed: %v", err)
	}
	return false, err
}

func defaultHeadSHA(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

const detachedHEADBranch = ""

func defaultCurrentBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return detachedHEADBranch, nil
	}
	return strings.TrimSpace(string(out)), nil
}

func defaultGateTestRunner(repoRoot string, suite string) error {
	cmd := exec.Command("go", "test", "-count=1", suite)
	cmd.Dir = filepath.Join(repoRoot, "go")
	cmd.Env = stripBypassEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go test %s: %w\n%s", suite, err, out)
	}
	return nil
}

// A repo without .evolve/naming.json has nothing to guard, so a missing manifest passes.
func defaultNameGuard(repoRoot string) ([]naminguard.Violation, error) {
	manifestPath := filepath.Join(repoRoot, naminguard.DefaultManifestPath)
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, nil
	}
	m, err := naminguard.Load(manifestPath)
	if err != nil {
		return nil, err
	}
	return naminguard.Scan(repoRoot, m)
}

func stripBypassEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "EVOLVE_BYPASS_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

var defaultGoBinFn = func() string { return "go" }

func defaultSimulationRunner(repoRoot string) error {
	goBin := defaultGoBinFn()
	cmd := exec.Command(goBin, "test", "./internal/bridge/", "-run", "AutoRespond|SendKeySequence|RealizeFor")
	cmd.Dir = filepath.Join(repoRoot, "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("auto-respond regression tests failed: %w (output: %s)", err, out)
	}
	return nil
}

// A lookup failure is the unavailable verdict, never an error: absent tooling must not block a release.
func defaultCIConclusion(repoRoot string) (CIRunStatus, error) {
	head, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return CIRunStatus{Conclusion: ciConclusionUnavailable}, nil
	}
	sha := strings.TrimSpace(string(head))
	cmd := exec.Command("gh", "run", "list", "--workflow", ciparity.RequiredWorkflow,
		"--commit", sha, "--limit", "1", "--json", "status,conclusion,url")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return CIRunStatus{Conclusion: ciConclusionUnavailable}, nil
	}
	var runs []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		URL        string `json:"url"`
	}
	if json.Unmarshal(out, &runs) != nil || len(runs) == 0 {
		return CIRunStatus{Conclusion: ciConclusionUnavailable}, nil
	}
	if runs[0].Status != "completed" {
		return CIRunStatus{Conclusion: "pending", RunURL: runs[0].URL}, nil
	}
	return CIRunStatus{Conclusion: runs[0].Conclusion, RunURL: runs[0].URL}, nil
}

func Run(opts Options) (Result, error) {
	o, err := resolve(opts)
	if err != nil {
		return Result{StepsTotal: len(preflightSteps)}, err
	}
	p := newPreflightRun(o, opts.Stderr)

	for _, step := range preflightSteps {
		if err := step(p); err != nil {
			return p.res, err
		}
		p.res.StepsPassed++
	}
	if err := p.gateReleaseCommitCI(); err != nil {
		return p.res, err
	}
	p.adviseSimulation()
	p.logDone()
	return p.res, nil
}

type auditResult struct {
	auditedHead  string
	artifact     string
	verdict      string
	age          time.Duration
	phantomCount int
}

func auditedUncommittedWork(e auditledger.Entry) bool {
	return e.WorktreeTreeSHA != ""
}

func shortSHA(sha string) string {
	if sha == "" {
		return "unknown"
	}
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func makeInlineVerdictRE(strict bool) *regexp.Regexp {
	accept := "(PASS|WARN)"
	if strict {
		accept = "(PASS)"
	}
	pattern := `(?i)Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*` + accept + `([[:space:]]|$|\*)`
	return regexp.MustCompile(pattern)
}

var verdictHeadingRE = regexp.MustCompile(`(?i)^#+\s+(?:[0-9]+\.\s+)?Verdict\s*$`)

func markerVerdict(body string) (string, bool) {
	s, ok := phasecontract.ParseVerdictSentinelFull(body)
	if !ok {
		return "", false
	}
	switch v := strings.ToUpper(strings.TrimSpace(s.Verdict)); v {
	case "PASS", "WARN", "FAIL":
		return v, true
	}
	return "", false
}

var (
	boldPassRE = regexp.MustCompile(`\*\*PASS[.!]?\*\*`)
	boldWarnRE = regexp.MustCompile(`\*\*WARN[.!]?\*\*`)
)

const auditVerdictNone = "NONE"

const auditVerdictScopedOut = "SCOPED_OUT"

func checkRecentAudit(ledgerPath, releaseHead string, strict bool, now time.Time) (auditResult, error) {
	var res auditResult
	rows, err := auditledger.AuditorRows(ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			res.verdict = auditVerdictNone
			return res, nil
		}
		return res, err
	}
	if len(rows) == 0 {
		res.verdict = auditVerdictNone
		return res, nil
	}

	candidate, artifact, phantom := selectAuditCandidate(rows)
	res.artifact = artifact
	res.phantomCount = phantom
	if candidate == nil {
		res.verdict = auditVerdictNone
		return res, nil
	}

	artifactBody, err := os.ReadFile(res.artifact)
	if err != nil {
		return res, fmt.Errorf("read audit-report.md: %v", err)
	}
	verdict, ok := extractVerdict(string(artifactBody), strict)
	if !ok {
		return scopedOutOrFail(res, *candidate, releaseHead, strict)
	}
	res.verdict = verdict

	age, err := auditAge(*candidate, now)
	res.age = age
	if err != nil {
		return res, err
	}
	return res, nil
}

func selectAuditCandidate(rows []auditledger.Entry) (candidate *auditledger.Entry, artifact string, phantom int) {
	for i := range rows {
		if rows[i].ArtifactPath == "" {
			phantom++
			continue
		}
		if _, err := os.Stat(rows[i].ArtifactPath); err == nil {
			return &rows[i], rows[i].ArtifactPath, phantom
		}
		phantom++
	}
	return nil, "", phantom
}

func scopedOutOrFail(res auditResult, candidate auditledger.Entry, releaseHead string, strict bool) (auditResult, error) {
	auditedHead := candidate.GitHEAD
	releaseHeadKnown := releaseHead != ""
	auditedAnotherCommit := auditedHead != "" && auditedHead != releaseHead
	if releaseHeadKnown && (auditedAnotherCommit || auditedUncommittedWork(candidate)) {
		res.verdict = auditVerdictScopedOut
		res.auditedHead = auditedHead
		return res, nil
	}
	if strict {
		return res, fmt.Errorf("EVOLVE_RELEASE_STRICT_PASS=1 and most recent audit-report.md does not declare 'Verdict: PASS' (%s)",
			res.artifact)
	}
	return res, fmt.Errorf("most recent audit-report.md does not declare 'Verdict: PASS' or 'Verdict: WARN' (%s)",
		res.artifact)
}

func auditAge(candidate auditledger.Entry, now time.Time) (time.Duration, error) {
	if candidate.TS == "" {
		return 0, errors.New("ledger entry missing ts")
	}
	ts, err := time.Parse(time.RFC3339, candidate.TS)
	if err != nil {
		return 0, nil
	}
	age := now.Sub(ts)
	if age >= MaxAuditAge {
		return age, fmt.Errorf("audit is %ds old (>%ds); re-run Auditor",
			int(age.Seconds()), int(MaxAuditAge.Seconds()))
	}
	return age, nil
}

func extractVerdict(body string, strict bool) (string, bool) {
	if v, present := markerVerdict(body); present {
		if v == "PASS" || (!strict && v == "WARN") {
			return v, true
		}
		return "", false
	}

	inline := makeInlineVerdictRE(strict)
	if m := inline.FindStringSubmatch(body); m != nil {
		v := strings.ToUpper(m[1])
		return v, true
	}
	return headingFormVerdict(body, strict)
}

const verdictHeadingLookaheadLines = 5

func headingFormVerdict(body string, strict bool) (string, bool) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !verdictHeadingRE.MatchString(line) {
			continue
		}
		for j := i + 1; j <= i+verdictHeadingLookaheadLines && j < len(lines); j++ {
			line := lines[j]
			if boldPassRE.MatchString(line) || strings.TrimSpace(line) == "PASS" {
				return "PASS", true
			}
			if !strict && (boldWarnRE.MatchString(line) || strings.TrimSpace(line) == "WARN") {
				return "WARN", true
			}
		}
	}
	return "", false
}
