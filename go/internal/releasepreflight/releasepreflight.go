// Package releasepreflight ports legacy/scripts/release/preflight.sh.
//
// Pre-flight gate for the release pipeline. Verifies the local environment
// is ready before any mutating step runs; never modifies state. Five
// sequential checks — first failure is fatal:
//
//  1. Working tree clean (no unstaged/staged modifications).
//  2. Branch not detached (HEAD is a symbolic ref).
//  3. <target-version> parses as semver and is strictly greater than the
//     current plugin.json version.
//  4. Auditor ledger has a recent (<7 days) PASS (or WARN, fluent posture)
//     verdict for HEAD with an on-disk artifact-report.md.
//  5. Trust-boundary gate-test packages pass: ./internal/guards/... (ship,
//     role, phase gates) and ./internal/phases/ship/... (native ship matrix).
//     Replaces the deleted legacy/scripts/tests/*.sh bash suites (v12).
//
// Exit codes (cmd layer maps from sentinel errors):
//
//	0  — all checks pass
//	1  — some check failed (ErrCheckFailed wraps the cause)
//	10 — invalid arguments (handled in cmd layer)
//
// EVOLVE_RELEASE_STRICT_PASS=1 forces step 4 to reject WARN, matching the
// bash strict-PASS gate.
package releasepreflight

import (
	"bufio"
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

	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

// Sentinel errors. ErrCheckFailed maps to exit 1.
var (
	ErrCheckFailed = errors.New("releasepreflight: check failed")
)

// MaxAuditAge is the upper bound on how stale the most recent auditor PASS
// can be (mirrors bash: 7 days).
const MaxAuditAge = 7 * 24 * time.Hour

// Options drives a Run() invocation. The seam fields default to real
// implementations when nil.
type Options struct {
	Target     string
	RepoRoot   string
	DryRun     bool
	SkipTests  bool
	StrictPass bool // honors EVOLVE_RELEASE_STRICT_PASS
	Stderr     io.Writer

	// Filesystem paths; defaulted from RepoRoot if empty.
	PluginJSONPath string
	LedgerPath     string

	// Seams.
	Now              func() time.Time
	GitClean         func(repoRoot string) (bool, error)                   // step 1
	CurrentBranch    func(repoRoot string) (string, error)                 // step 2
	GateTestRunner   func(repoRoot string, suite string) error             // step 5
	NameGuard        func(repoRoot string) ([]naminguard.Violation, error) // step 5 sub-check
	SimulationRunner func(repoRoot string) error                           // advisory step (post-step-5)
}

// Result captures what happened; populated even on failure for diagnostics.
type Result struct {
	StepsPassed     int
	StepsTotal      int
	CurrentVersion  string
	AuditArtifact   string
	AuditVerdict    string // "PASS", "WARN", or "NONE" (no on-disk audit available — advisory; CI-green is the authoritative gate)
	AuditAge        time.Duration
	PhantomEntries  int
	GateTestsPassed int

	// SimulationAdvisoryOK records the outcome of the post-step-5 advisory
	// auto-respond simulation suite run (v12.1.5+). nil = skipped (DryRun or
	// SkipTests); &true = passed; &false = failed-but-advisory (logged as
	// WARN but does not block release). Promotes to hard requirement in v12.2.0.
	SimulationAdvisoryOK *bool
}

// DefaultGateTestSuites are the trust-boundary Go test packages run as the
// pre-publish gate (in run order). v12 deleted the legacy/scripts/tests/*.sh
// suites; these Go packages are their equivalents — internal/guards covers the
// ship/role/phase gates (the old guards-test, role-gate-test,
// phase-gate-precondition-test), and internal/phases/ship covers the native
// ship integration matrix (the old ship-integration-test). Tests override the
// GateTestRunner seam to bypass actual execution.
var DefaultGateTestSuites = []string{
	"./internal/guards/...",
	"./internal/phases/ship/...",
}

// IsSemver matches X.Y.Z[+pre] (numeric components; optional +pre/-pre suffix
// is parsed but stripped — matches bash parse_semver).
var semverRE = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)([+-].*)?$`)

// ParseSemver returns (major, minor, patch, true) on a valid X.Y.Z string.
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

// SemverGT returns true iff a > b.
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

// ExtractJSONVersion mirrors the bash extract_json_version sed pipeline.
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

// defaultGitClean runs `git -C <repo> diff --quiet HEAD`, returning (clean, err).
func defaultGitClean(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--quiet", "HEAD")
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		// Exit 1 means tree is dirty; exit 128 means real error.
		if exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git diff failed: %v", err)
	}
	return false, err
}

// defaultCurrentBranch runs `git -C <repo> symbolic-ref --short HEAD`.
// Returns "" (and nil) on detached HEAD to mirror bash semantics.
func defaultCurrentBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// symbolic-ref exits non-zero on detached HEAD.
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// defaultGateTestRunner runs `go test <suite>` from the Go module at
// <repoRoot>/go. The ship/role-gate bypass vars are stripped from the child
// env so the guard DENY-tests stay hermetic even when an operator's session
// (e.g. a dev's settings.local.json) sets them — otherwise a bypass would flip
// a DENY assertion and silently weaken the pre-publish gate.
func defaultGateTestRunner(repoRoot string, suite string) error {
	cmd := exec.Command("go", "test", "-count=1", suite)
	cmd.Dir = filepath.Join(repoRoot, "go")
	cmd.Env = stripBypassEnv(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go test %s: %w\n%s", suite, err, out)
	}
	return nil
}

// defaultNameGuard scans tracked files for dead naming tokens using the SSOT
// manifest. It no-ops (no manifest → nothing to guard) so the preflight stays
// green on repos that have not adopted .evolve/naming.json.
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

// stripBypassEnv returns env without the EVOLVE_BYPASS_* gate-bypass vars, so a
// child `go test` of the guard suites evaluates the real DENY behavior.
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

// defaultGoBinFn resolves the Go binary name used by defaultSimulationRunner.
// Tests replace this var (with t.Cleanup restore) to inject a fake binary.
var defaultGoBinFn = func() string { return "go" }

// defaultSimulationRunner runs the auto-respond regression coverage that
// guards against manifest/policy regressions. The bash bats simulation suite
// (tools/agent-bridge/tests/simulation) was removed in the v12 Go-bridge
// cutover; the equivalent coverage now lives in the Go bridge package's
// auto-respond + manifest tests, which this runs via `go test`.
//
// Returns an error if `go` is missing or any auto-respond test fails. The
// caller logs the error as WARN — advisory in v12.1.5, blocking in v12.2.0.
func defaultSimulationRunner(repoRoot string) error {
	goBin := defaultGoBinFn()
	cmd := exec.Command(goBin, "test", "./internal/bridge/", "-run", "AutoRespond|SendKeySequence|RealizeFor")
	cmd.Dir = filepath.Join(repoRoot, "go")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("auto-respond regression tests failed: %w (output: %s)", err, out)
	}
	return nil
}

// Run executes all 5 preflight steps in order. Returns ErrCheckFailed
// wrapped with a per-step message; cmd layer logs the message and exits 1.
func Run(opts Options) (Result, error) {
	res := Result{StepsTotal: 5}

	// Resolve defaults.
	if opts.RepoRoot == "" {
		return res, fmt.Errorf("%w: RepoRoot required", ErrCheckFailed)
	}
	if opts.PluginJSONPath == "" {
		opts.PluginJSONPath = filepath.Join(opts.RepoRoot, ".claude-plugin", "plugin.json")
	}
	if opts.LedgerPath == "" {
		opts.LedgerPath = filepath.Join(opts.RepoRoot, ".evolve", "ledger.jsonl")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	gitClean := opts.GitClean
	if gitClean == nil {
		gitClean = defaultGitClean
	}
	currentBranch := opts.CurrentBranch
	if currentBranch == nil {
		currentBranch = defaultCurrentBranch
	}
	gateRunner := opts.GateTestRunner
	if gateRunner == nil {
		gateRunner = defaultGateTestRunner
	}
	logw := opts.Stderr
	if logw == nil {
		logw = io.Discard
	}
	logf := func(format string, args ...any) {
		fmt.Fprintf(logw, "[preflight] "+format+"\n", args...)
	}

	// Step 1: clean tree.
	logf("step 1: working tree clean?")
	if opts.DryRun {
		logf("DRY-RUN: would check git diff --quiet HEAD")
	} else {
		clean, err := gitClean(opts.RepoRoot)
		if err != nil {
			return res, fmt.Errorf("%w: step 1 git error: %v", ErrCheckFailed, err)
		}
		if !clean {
			return res, fmt.Errorf("%w: working tree has uncommitted changes — commit or stash first", ErrCheckFailed)
		}
		logf("OK: working tree clean")
	}
	res.StepsPassed++

	// Step 2: branch attached.
	logf("step 2: branch attached?")
	if opts.DryRun {
		logf("DRY-RUN: would check git symbolic-ref --short HEAD")
	} else {
		branch, err := currentBranch(opts.RepoRoot)
		if err != nil {
			return res, fmt.Errorf("%w: step 2 git error: %v", ErrCheckFailed, err)
		}
		if branch == "" {
			return res, fmt.Errorf("%w: detached HEAD — checkout a branch first", ErrCheckFailed)
		}
		logf("OK: on branch %s", branch)
	}
	res.StepsPassed++

	// Step 3: semver bump validation.
	logf("step 3: target version %s > current?", opts.Target)
	if _, _, _, ok := ParseSemver(opts.Target); !ok {
		return res, fmt.Errorf("%w: target version not semver: %s", ErrCheckFailed, opts.Target)
	}
	if _, err := os.Stat(opts.PluginJSONPath); err != nil {
		return res, fmt.Errorf("%w: plugin.json missing at %s", ErrCheckFailed, opts.PluginJSONPath)
	}
	current, err := ExtractJSONVersion(opts.PluginJSONPath)
	if err != nil {
		return res, fmt.Errorf("%w: %v", ErrCheckFailed, err)
	}
	res.CurrentVersion = current
	if _, _, _, ok := ParseSemver(current); !ok {
		return res, fmt.Errorf("%w: current plugin.json version not semver: %s", ErrCheckFailed, current)
	}
	if opts.Target == current {
		return res, fmt.Errorf("%w: target %s equals current %s — nothing to bump", ErrCheckFailed, opts.Target, current)
	}
	if !SemverGT(opts.Target, current) {
		return res, fmt.Errorf("%w: target %s is not greater than current %s", ErrCheckFailed, opts.Target, current)
	}
	logf("OK: %s → %s (valid bump)", current, opts.Target)
	res.StepsPassed++

	// Step 4: recent audit PASS.
	logf("step 4: recent auditor PASS verdict?")
	if opts.DryRun {
		logf("DRY-RUN: would check %s for recent auditor PASS", opts.LedgerPath)
	} else {
		auditRes, err := checkRecentAudit(opts.LedgerPath, opts.StrictPass, now())
		if err != nil {
			return res, fmt.Errorf("%w: %v", ErrCheckFailed, err)
		}
		res.AuditArtifact = auditRes.artifact
		res.AuditVerdict = auditRes.verdict
		res.AuditAge = auditRes.age
		res.PhantomEntries = auditRes.phantomCount
		if auditRes.verdict == auditVerdictNone {
			// Determinism: no on-disk audit available in this worktree (clean
			// checkout / CI / GC'd artifacts). The authoritative release gate is
			// CI-green on the release commit (enforced by /publish) — advisory only.
			logf("advisory: no on-disk audit in this worktree — CI-green on the release commit is the authoritative gate (/publish). Skipping the audit-PASS check.")
		} else {
			if auditRes.phantomCount > 0 {
				logf("WARN: skipped %d phantom auditor entry/entries (artifact missing on disk). Using most-recent VALID entry.", auditRes.phantomCount)
			}
			if auditRes.verdict == "WARN" {
				logf("INFO: most recent audit is WARN (fluent posture; ships by default). Set EVOLVE_RELEASE_STRICT_PASS=1 for strict-PASS gate.")
			}
			logf("OK: latest audit %s, artifact=%s", auditRes.verdict, auditRes.artifact)
		}
	}
	res.StepsPassed++

	// Step 5: gate-test suites.
	logf("step 5: gate-test suites green?")
	if opts.SkipTests {
		logf("WARN: --skip-tests set; skipping gate-test execution")
		res.StepsPassed++
	} else if opts.DryRun {
		logf("DRY-RUN: would run %d gate-test suites", len(DefaultGateTestSuites))
		res.StepsPassed++
	} else {
		for _, suite := range DefaultGateTestSuites {
			logf("  running %s...", suite)
			if err := gateRunner(opts.RepoRoot, suite); err != nil {
				return res, fmt.Errorf("%w: gate-test suite failed: %s — re-run interactively to inspect (%v)",
					ErrCheckFailed, suite, err)
			}
			res.GateTestsPassed++
		}
		logf("OK: all %d gate-test suites green", len(DefaultGateTestSuites))

		// Step 5 sub-check: no dead naming tokens survive in tracked files.
		// Shares the legacynames acs gate's scanner + SSOT (.evolve/naming.json),
		// so a release can't ship a rename that left a 404 slug / dead command
		// behind. No-ops when the repo has no manifest.
		nameGuard := opts.NameGuard
		if nameGuard == nil {
			nameGuard = defaultNameGuard
		}
		logf("  scanning for dead naming tokens (.evolve/naming.json)...")
		vs, err := nameGuard(opts.RepoRoot)
		if err != nil {
			return res, fmt.Errorf("%w: naming guard error: %v", ErrCheckFailed, err)
		}
		if len(vs) > 0 {
			return res, fmt.Errorf("%w: %d dead naming token(s) in tracked files — run `evolve names fix`: %s",
				ErrCheckFailed, len(vs), vs[0])
		}
		logf("OK: no dead naming tokens")
		// Step 5 counts as passed only here — after BOTH the gate-test suites
		// (above) and this naming scan come back clean.
		res.StepsPassed++
	}

	// Advisory step (v12.1.5+): auto-respond simulation suite. Does NOT count
	// toward StepsPassed/StepsTotal and never returns ErrCheckFailed; logged
	// as WARN on failure. Promotes to a required step in v12.2.0.
	if opts.SkipTests {
		logf("advisory: auto-respond simulation suite — skipped (--skip-tests)")
	} else if opts.DryRun {
		logf("advisory: auto-respond simulation suite — skipped (dry-run)")
	} else {
		simRunner := opts.SimulationRunner
		if simRunner == nil {
			simRunner = defaultSimulationRunner
		}
		logf("advisory: running auto-respond simulation suite...")
		if err := simRunner(opts.RepoRoot); err != nil {
			f := false
			res.SimulationAdvisoryOK = &f
			logf("WARN: auto-respond simulation suite failed (advisory in v12.1.5; required in v12.2.0): %v", err)
		} else {
			t := true
			res.SimulationAdvisoryOK = &t
			logf("OK: auto-respond simulation suite passed")
		}
	}

	dryRunSuffix := ""
	if opts.DryRun {
		dryRunSuffix = " (dry-run)"
	}
	logf("DONE: preflight passed for %s%s", opts.Target, dryRunSuffix)
	return res, nil
}

// --- Audit-ledger walker ---------------------------------------------------

type auditResult struct {
	artifact     string
	verdict      string // "PASS" or "WARN"
	age          time.Duration
	phantomCount int
}

// roleAuditorRE matches the bash grep '"role":"auditor"' substring search.
var roleAuditorRE = regexp.MustCompile(`"role":"auditor"`)

// artifactPathRE extracts the artifact_path field (matches jq -r .artifact_path).
var artifactPathRE = regexp.MustCompile(`"artifact_path":"([^"]*)"`)

// tsFieldRE extracts the ts field.
var tsFieldRE = regexp.MustCompile(`"ts":"([^"]*)"`)

// inlineVerdictRE matches `Verdict: PASS`, `**Verdict: PASS**`, or
// `Verdict: **PASS**` (case-insensitive). Matches the bash:
//
//	grep -qiE "Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*${accept_pattern}([[:space:]]|\$|\*)"
//
// Mirrors the optional bold-wrapping syntax tolerated by the bash regex.
func makeInlineVerdictRE(strict bool) *regexp.Regexp {
	accept := "(PASS|WARN)"
	if strict {
		accept = "(PASS)"
	}
	pattern := `(?i)Verdict[[:space:]]*:[[:space:]]*\*?\*?[[:space:]]*` + accept + `([[:space:]]|$|\*)`
	return regexp.MustCompile(pattern)
}

// headingVerdictRE matches `## Verdict\n**PASS**` (or **WARN**) — the
// awk-based fallback in bash. We scan up to 5 lines after the heading.
var verdictHeadingRE = regexp.MustCompile(`(?i)^#+\s+(?:[0-9]+\.\s+)?Verdict\s*$`)

// auditVerdictNone marks that no on-disk audit artifact was available to check
// (absent ledger, no auditor entry, or all artifacts GC'd) — distinct from a
// real PASS/WARN/FAIL. Determinism fix: a release MUST NOT be blocked by absent
// transient runtime state (a clean checkout / CI / fresh worktree has no
// ledger). The authoritative release gate is CI-green on the release commit,
// enforced by the /publish skill. A present-but-failed audit still hard-blocks.
const auditVerdictNone = "NONE"

func checkRecentAudit(ledgerPath string, strict bool, now time.Time) (auditResult, error) {
	var res auditResult
	body, err := os.ReadFile(ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Clean checkout / CI / fresh worktree: no ledger → advisory, not fatal.
			res.verdict = auditVerdictNone
			return res, nil
		}
		return res, fmt.Errorf("ledger read %s: %w", ledgerPath, err)
	}
	// Walk auditor entries in reverse (newest first).
	var auditorEntries []string
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	scanner.Buffer(make([]byte, 1<<20), 1<<24) // allow large lines
	for scanner.Scan() {
		line := scanner.Text()
		if roleAuditorRE.MatchString(line) {
			auditorEntries = append(auditorEntries, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return res, fmt.Errorf("ledger read: %v", err)
	}
	if len(auditorEntries) == 0 {
		// Ledger exists but no audit has run → audit signal unavailable, advisory.
		res.verdict = auditVerdictNone
		return res, nil
	}

	var candidate string
	for i := len(auditorEntries) - 1; i >= 0; i-- {
		entry := auditorEntries[i]
		m := artifactPathRE.FindStringSubmatch(entry)
		if len(m) < 2 || m[1] == "" {
			res.phantomCount++
			continue
		}
		if _, err := os.Stat(m[1]); err == nil {
			candidate = entry
			res.artifact = m[1]
			break
		}
		res.phantomCount++
	}
	if candidate == "" {
		// Audit artifacts GC'd (all-phantom) or none usable → signal unavailable,
		// advisory (not a failed audit). CI-green is the authoritative gate.
		res.verdict = auditVerdictNone
		return res, nil
	}

	// Verdict check.
	artifactBody, err := os.ReadFile(res.artifact)
	if err != nil {
		return res, fmt.Errorf("read audit-report.md: %v", err)
	}
	verdict, ok := extractVerdict(string(artifactBody), strict)
	if !ok {
		if strict {
			return res, fmt.Errorf("EVOLVE_RELEASE_STRICT_PASS=1 and most recent audit-report.md does not declare 'Verdict: PASS' (%s)",
				res.artifact)
		}
		return res, fmt.Errorf("most recent audit-report.md does not declare 'Verdict: PASS' or 'Verdict: WARN' (%s)",
			res.artifact)
	}
	res.verdict = verdict

	// Age check.
	tsMatch := tsFieldRE.FindStringSubmatch(candidate)
	if len(tsMatch) < 2 || tsMatch[1] == "" {
		return res, errors.New("ledger entry missing ts")
	}
	ts, err := time.Parse(time.RFC3339, tsMatch[1])
	if err != nil {
		// Bash fallback: missing/unparseable ts → skip age check (return ok).
		return res, nil
	}
	res.age = now.Sub(ts)
	if res.age >= MaxAuditAge {
		return res, fmt.Errorf("audit is %ds old (>%ds); re-run Auditor",
			int(res.age.Seconds()), int(MaxAuditAge.Seconds()))
	}
	return res, nil
}

// extractVerdict returns ("PASS"|"WARN", true) on a match. Accepts both
// inline (`Verdict: PASS`) and heading form (`## Verdict\n**PASS**`).
// When strict=true, WARN is rejected.
func extractVerdict(body string, strict bool) (string, bool) {
	inline := makeInlineVerdictRE(strict)
	if m := inline.FindStringSubmatch(body); m != nil {
		v := strings.ToUpper(m[1])
		return v, true
	}
	// Heading form: scan for `## Verdict` line, then within 5 lines look
	// for **PASS**/**WARN** or a BARE verdict line (exactly `PASS`/`WARN`,
	// the cycle-249 shape — auditors legitimately omit the bold). A sentence
	// merely containing the word must not match.
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !verdictHeadingRE.MatchString(line) {
			continue
		}
		for j := i + 1; j <= i+5 && j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if strings.Contains(lines[j], "**PASS**") || trimmed == "PASS" {
				return "PASS", true
			}
			if !strict && (strings.Contains(lines[j], "**WARN**") || trimmed == "WARN") {
				return "WARN", true
			}
		}
	}
	return "", false
}
