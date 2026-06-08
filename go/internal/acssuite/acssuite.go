// Package acssuite is the deterministic, host-side EGPS predicate-suite
// runner. It restores the suite-execution mechanism that the deleted bash
// run-acs-suite.sh used to provide (v12 flag-day removed it without a Go
// port — see ADR-0025), and extends it with the standing red-team glob.
//
// Run executes TWO predicate lanes and merges them into one acs-verdict.json
// conforming to the schema the audit + ship gates read
// (docs/architecture/egps-v10.md):
//
//  1. Bash lane — globs three roots, executes each `.sh` with a per-predicate
//     timeout:
//     - <root>/acs/cycle-<N>/*.sh                    this cycle's predicates
//     - <root>/acs/regression-suite/cycle-*/*.sh     accumulated prior predicates
//     - <root>/acs/red-team/rt-*.sh                  standing adversarial predicates
//  2. Go lane — one post-pass `go test -json -tags acs -count=1 ./acs/cycle<N>`
//     from the Go module dir (<root>/go by default), scoped to the CURRENT
//     cycle (mirrors the bash lane, which never runs every historical cycle).
//     Each test maps to a Result via the SAME counting path as the bash lane,
//     so the gate invariant cannot diverge. A non-compiling predicate package
//     is a HARD error, never a silent PASS; a cycle with no Go package yet is a
//     no-op. The lane is on by default; Options.RunGo=false opts out.
//
// A double-count guard synthesizes a RED for any (cycle, ac) pair asserted by
// BOTH a bash and a Go predicate (a missed Phase-C lockstep-delete).
//
// red_count == 0 ⇒ verdict PASS ⇒ ship_eligible. A non-zero exit is RED,
// EXCEPT exit 77 = SKIP (the TAP/automake convention): an evidence-absent
// predicate (e.g. a runtime-only regression predicate on a fresh clone where
// the gitignored .evolve/ artifact it inspects does not exist) is counted
// neither red nor green, so it cannot block the gate yet cannot fake a pass.
// CLI: `evolve acs suite --cycle N`.
package acssuite

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
)

// killGrace is how long runBash waits, after the timeout kills the predicate's
// process group, for I/O to drain before force-closing the pipes.
const killGrace = 2 * time.Second

// DefaultTimeout bounds a single predicate's execution. EGPS predicates are
// meant to be fast assertions; a predicate that hangs is treated as RED
// (timeout). But agents legitimately author "full-suite-green" predicates that
// run `go test ./...`, which on a large repo exceeds 60s and flakes to a false
// RED (exit 124) — a passing suite must not be sunk by a too-tight timeout
// (cycle-200). EVOLVE_ACS_PREDICATE_TIMEOUT_S overrides this when a suite
// legitimately needs longer; unset/invalid keeps the 60s default.
const DefaultTimeout = 60 * time.Second

// resolveTimeout returns the per-predicate timeout: opts.Timeout when > 0, else
// EVOLVE_ACS_PREDICATE_TIMEOUT_S (seconds) when set to a positive integer, else
// DefaultTimeout. envGet defaults to os.Getenv (injectable for tests).
func resolveTimeout(optsTimeout time.Duration, envGet func(string) string) time.Duration {
	if optsTimeout > 0 {
		return optsTimeout
	}
	if envGet == nil {
		envGet = os.Getenv
	}
	if raw := envGet("EVOLVE_ACS_PREDICATE_TIMEOUT_S"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return DefaultTimeout
}

// evidenceMax caps the captured output excerpt per predicate.
const evidenceMax = 600

// SkipExitCode is the TAP/automake SKIP convention: a predicate exiting 77
// declares its evidence absent / not-applicable on this clone. It is counted
// neither red nor green.
const SkipExitCode = 77

// Result is one predicate's outcome (egps-v10 schema).
type Result struct {
	ACID            string `json:"ac_id"`
	Predicate       string `json:"predicate"` // repo-relative path
	ExitCode        int    `json:"exit_code"`
	ResultStr       string `json:"result"` // "green" | "red" | "skip"
	DurationMS      int64  `json:"duration_ms"`
	IsRegression    bool   `json:"is_regression"`
	IsRedTeam       bool   `json:"is_red_team,omitempty"`
	EvidenceExcerpt string `json:"evidence_excerpt,omitempty"`
}

// PredicateSuite is the count breakdown.
type PredicateSuite struct {
	ThisCycleCount       int `json:"this_cycle_count"`
	RegressionSuiteCount int `json:"regression_suite_count"`
	RedTeamCount         int `json:"red_team_count"`
	SkippedCount         int `json:"skipped_count"`
	Total                int `json:"total"`
}

// Verdict is the acs-verdict.json schema read by audit + ship gates.
type Verdict struct {
	SchemaVersion  string         `json:"schema_version"`
	Cycle          int            `json:"cycle"`
	PredicateSuite PredicateSuite `json:"predicate_suite"`
	Results        []Result       `json:"results"`
	GreenCount     int            `json:"green_count"`
	RedCount       int            `json:"red_count"`
	SkipCount      int            `json:"skip_count"`
	RedIDs         []string       `json:"red_ids"`
	SkipIDs        []string       `json:"skip_ids,omitempty"`
	Verdict        string         `json:"verdict"` // PASS | FAIL
	ShipEligible   bool           `json:"ship_eligible"`
}

// Options configures Run. Root and Cycle are required.
type Options struct {
	Root  string // repo root containing acs/ (where predicate FILES are discovered)
	Cycle int    // current cycle number
	// ProjectRoot is the MAIN project root whose `.evolve/` holds the runtime data
	// (history under .evolve/runs/, baselines, the current build-report) that
	// predicates read via ${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}. When set, it is
	// exported as EVOLVE_PROJECT_ROOT to each predicate so a suite discovered from a
	// worktree (Root=worktree, post issue-#9 audit-cwd=worktree) still resolves
	// `.evolve/` to main rather than the worktree (where `.evolve/` is absent).
	// Empty → predicates inherit the caller's env (legacy behavior). (issue #12)
	ProjectRoot string
	Timeout     time.Duration // per-predicate (bash lane); 0 → DefaultTimeout
	// GoModuleDir is the directory holding go.mod + the acs/ predicate subtree.
	// Empty → filepath.Join(Root, "go"). The Go lane runs
	// `go test -json -tags acs -count=1 ./acs/cycle<N>` from here.
	GoModuleDir string
	// RunGo gates the Go predicate lane: nil or *true → run; *false → skip
	// (the `evolve acs suite --no-go` opt-out). The Go lane is strictly stricter
	// than the bash-only suite (Go predicates were previously uncounted), so it
	// is on by default.
	RunGo *bool
	// GoTimeout bounds the WHOLE Go lane via context cancellation (not
	// per-predicate; Go compiles per package). 0 → EVOLVE_ACS_GO_TIMEOUT_S
	// (seconds) when set, else DefaultTimeout (the current-cycle scope runs a
	// single package).
	GoTimeout time.Duration
	// Seams (default to production behavior when nil).
	Now  func() time.Time
	Exec func(ctx context.Context, path string) (exitCode int, output string)
	// GoExec runs ONE Go predicate-lane package pattern and returns the raw
	// `go test -json` output plus the process exit error (nil on exit 0, an
	// *exec.ExitError on nonzero). It is called once per active scope
	// (current-cycle, each regression sub-package, redteam). Injected by tests;
	// nil → defaultGoExec.
	GoExec func(ctx context.Context, moduleDir, pkgPattern string, env []string) (rawJSON string, err error)
}

type predFile struct {
	path         string // absolute
	isRegression bool
	isRedTeam    bool
}

// Run discovers and executes the predicate suite, returning the Verdict.
func Run(opts Options) (Verdict, error) {
	if opts.Root == "" {
		return Verdict{}, fmt.Errorf("acssuite: Root required")
	}
	if opts.Cycle <= 0 {
		return Verdict{}, fmt.Errorf("acssuite: Cycle must be > 0")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	timeout := resolveTimeout(opts.Timeout, nil)
	execFn := opts.Exec
	if execFn == nil {
		projectRoot := opts.ProjectRoot
		// cycle-190: execute predicates with cwd at opts.Root — the tree being
		// shipped (the worktree for worktree cycles, main otherwise). Predicates
		// are DISCOVERED from Root; running them with cwd=Root makes a relative
		// `go test ./...` compile the SAME source the auditor reviewed, instead of
		// the caller's cwd (main), which lacks the builder's worktree changes and
		// silently RED-flagged new-code predicates → discarded PASS-audited work.
		// `.evolve/` runtime data still resolves to main via EVOLVE_PROJECT_ROOT.
		workdir := opts.Root
		// Export CHANGED_PACKAGES so a predicate can scope `go test` to the
		// cycle's touched packages (assert_go_test_pass_changed) instead of the
		// whole repo — best-effort, empty when the handoff is absent (cycle-200).
		changed := changedPackagesForCycle(opts.ProjectRoot, opts.Cycle)
		execFn = func(ctx context.Context, path string) (int, string) {
			return runBash(ctx, path, projectRoot, workdir, changed)
		}
	}

	files, err := discover(opts.Root, opts.Cycle)
	if err != nil {
		return Verdict{}, err
	}

	v := Verdict{SchemaVersion: "1.0", Cycle: opts.Cycle}

	// ── Bash lane (unchanged execution; counting unified via v.record) ──
	for _, pf := range files {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := now()
		exitCode, output := execFn(ctx, pf.path)
		dur := now().Sub(start).Milliseconds()
		cancel()

		rel := relPath(opts.Root, pf.path)
		r := Result{
			ACID:         acIDFromRel(rel),
			Predicate:    rel,
			ExitCode:     exitCode,
			DurationMS:   dur,
			IsRegression: pf.isRegression,
			IsRedTeam:    pf.isRedTeam,
		}
		switch exitCode {
		case 0:
			r.ResultStr = "green"
		case SkipExitCode:
			// TAP/automake SKIP: evidence absent / not applicable on this
			// clone. Counted neither red nor green; capture the SKIP reason.
			r.ResultStr = "skip"
			r.EvidenceExcerpt = excerpt(output)
		default:
			r.ResultStr = "red"
			r.EvidenceExcerpt = excerpt(output)
		}
		v.record(r)
	}

	// ── Go lane: one post-pass `go test -json -tags acs ./acs/...` whose
	// per-test results merge into the SAME verdict via v.record, so the gate
	// invariant (red_count==0 ⟺ PASS) is identical for both lanes. A non-
	// compiling predicate package is a HARD error, never a silent PASS. ──
	if runGoEnabled(opts) {
		goResults, gErr := runGoTest(opts)
		if gErr != nil {
			return Verdict{}, gErr
		}
		for _, r := range goResults {
			v.record(r)
		}
	}

	// ── Double-count guard: a bash predicate and a Go predicate that resolve
	// to the same (cycle, ac) pair (a missed lockstep-delete during the Phase-C
	// port) synthesize a RED, so the slip fails loudly instead of inflating
	// green. Conservative: unparseable ACIDs are skipped. ──
	for _, r := range doubleCountReds(v.Results) {
		v.record(r)
	}

	v.PredicateSuite.SkippedCount = v.SkipCount
	v.PredicateSuite.Total = len(v.Results) // skips included
	// Invariant: a skip increments neither GreenCount nor RedCount, so
	// PASS ⟺ red_count==0 is preserved exactly as before SKIP existed.
	if v.RedCount == 0 {
		v.Verdict = "PASS"
		v.ShipEligible = true
	} else {
		v.Verdict = "FAIL"
	}
	return v, nil
}

// record appends a result and updates the green/red/skip tallies + the
// PredicateSuite bucketing. Shared by the bash and Go lanes so the gate
// invariant (red_count==0 ⟺ PASS) cannot diverge between them.
func (v *Verdict) record(r Result) {
	switch r.ResultStr {
	case "green":
		v.GreenCount++
	case "skip":
		v.SkipCount++
		v.SkipIDs = append(v.SkipIDs, r.ACID)
	case "red":
		v.RedCount++
		v.RedIDs = append(v.RedIDs, r.ACID)
	}
	switch {
	case r.IsRedTeam:
		v.PredicateSuite.RedTeamCount++
	case r.IsRegression:
		v.PredicateSuite.RegressionSuiteCount++
	default:
		v.PredicateSuite.ThisCycleCount++
	}
	v.Results = append(v.Results, r)
}

// predicateEnv builds the env exported to BOTH lanes: EVOLVE_PROJECT_ROOT (so
// predicates resolve `.evolve/` runtime data to MAIN even from a worktree, issue
// #12) and CHANGED_PACKAGES (so a predicate can scope `go test` to the cycle's
// touched packages, cycle-200). With no extras it equals os.Environ() — the
// prior inherit behavior.
func predicateEnv(projectRoot string, changedPkgs []string) []string {
	env := os.Environ()
	if projectRoot != "" {
		env = append(env, "EVOLVE_PROJECT_ROOT="+projectRoot)
	}
	if len(changedPkgs) > 0 {
		// Space-joining is safe: go package patterns never contain spaces.
		env = append(env, "CHANGED_PACKAGES="+strings.Join(changedPkgs, " "))
	}
	return env
}

// runGoEnabled reports whether the Go predicate lane should run (default on).
func runGoEnabled(opts Options) bool {
	return opts.RunGo == nil || *opts.RunGo
}

// hasGoACSTree reports whether moduleDir is a Go module (go.mod present) with an
// acs/ predicate subtree. When false, the Go lane is a no-op (backward-compat
// for callers without a Go predicate tree).
func hasGoACSTree(moduleDir string) bool {
	if fi, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil || fi.IsDir() {
		return false
	}
	if fi, err := os.Stat(filepath.Join(moduleDir, "acs")); err != nil || !fi.IsDir() {
		return false
	}
	return true
}

// defaultGoExec runs the real `go test -json -tags acs -count=1 <pkgPattern>`
// from moduleDir and returns the combined output + the process exit error.
// CombinedOutput merges build errors (stderr, non-JSON) into the stream;
// parseGoTestJSON tolerates the non-JSON lines.
func defaultGoExec(ctx context.Context, moduleDir, pkgPattern string, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-tags", "acs", "-count=1", pkgPattern)
	cmd.Dir = moduleDir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// goLaneTimeout returns the whole-lane timeout: opts.GoTimeout when > 0, else
// EVOLVE_ACS_GO_TIMEOUT_S (seconds) when set, else DefaultTimeout. The Go lane
// is bounded as a whole (via context cancellation in runGoTest) because Go
// compiles per package; the current-cycle scope runs a single package, so one
// DefaultTimeout is the right ceiling.
func goLaneTimeout(optsTimeout time.Duration, envGet func(string) string) time.Duration {
	if optsTimeout > 0 {
		return optsTimeout
	}
	if envGet == nil {
		envGet = os.Getenv
	}
	if raw := envGet("EVOLVE_ACS_GO_TIMEOUT_S"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return DefaultTimeout
}

// currentCycleGoPkgDir is the Go predicate package dir for the current cycle:
// <moduleDir>/acs/cycle<N>.
func currentCycleGoPkgDir(moduleDir string, cycle int) string {
	return filepath.Join(moduleDir, "acs", fmt.Sprintf("cycle%d", cycle))
}

// currentCycleGoPkgExists reports whether the current cycle has a Go predicate
// package on disk. When absent, the Go lane is a no-op (not an error) — the
// cycle simply has no Go ACs yet.
func currentCycleGoPkgExists(moduleDir string, cycle int) bool {
	fi, err := os.Stat(currentCycleGoPkgDir(moduleDir, cycle))
	return err == nil && fi.IsDir()
}

// goLanePatterns returns the existence-gated, NON-recursive package patterns the
// Go lane runs every cycle, mirroring the bash lane's three roots:
//   - the current cycle's package (`./acs/cycle<N>`) — this cycle's predicates;
//   - each regression sub-package (`./acs/regression/<sub>`) — the curated
//     durable set, run every cycle;
//   - the red-team package (`./acs/redteam`) — standing anti-gaming predicates.
//
// Each is a single, non-recursive pattern run as a SEPARATE `go test` so a
// per-package compile error is caught by the (execErr && zero-events) hard-gate;
// a recursive `./acs/regression/...` could let one sub-package's compile failure
// hide behind another's events. Patterns whose dir is absent are skipped.
func goLanePatterns(moduleDir string, cycle int) []string {
	var pats []string
	if dirExists(currentCycleGoPkgDir(moduleDir, cycle)) {
		pats = append(pats, fmt.Sprintf("./acs/cycle%d", cycle))
	}
	if entries, err := os.ReadDir(filepath.Join(moduleDir, "acs", "regression")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				pats = append(pats, "./acs/regression/"+e.Name())
			}
		}
	}
	if dirExists(filepath.Join(moduleDir, "acs", "redteam")) {
		pats = append(pats, "./acs/redteam")
	}
	return pats
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// runGoTest executes the Go predicate lane and maps its `go test -json` output
// into []Result. It runs each active scope (current cycle + regression
// sub-packages + redteam) as a SEPARATE pattern, merging results, mirroring the
// bash lane (current cycle + curated regression + red-team — never every
// historical cycle, which would drag bit-rotted predicates into the gate).
//
// It returns (nil, nil) when the lane is a no-op (no Go module / acs subtree and
// no GoExec seam, or no scope is present). It returns a HARD error — never an
// empty, gate-clearing slice — when ANY scope exited nonzero having produced
// zero test events (a compile error / infra failure): a broken predicate package
// must not silently PASS the gate.
func runGoTest(opts Options) ([]Result, error) {
	moduleDir := opts.GoModuleDir
	if moduleDir == "" {
		moduleDir = filepath.Join(opts.Root, "go")
	}

	goExec := opts.GoExec
	var patterns []string
	if goExec == nil {
		// No seam: require a real Go module + acs subtree, then existence-gate
		// each scope against the real filesystem.
		if !hasGoACSTree(moduleDir) {
			return nil, nil
		}
		goExec = defaultGoExec
		patterns = goLanePatterns(moduleDir, opts.Cycle)
		if len(patterns) == 0 {
			return nil, nil
		}
	} else {
		// Seam mode: the seam decides each scope's output (returns "" for
		// inactive scopes), so the scope list is fixed and filesystem-independent.
		patterns = []string{
			fmt.Sprintf("./acs/cycle%d", opts.Cycle),
			"./acs/regression/...",
			"./acs/redteam",
		}
	}

	changed := changedPackagesForCycle(opts.ProjectRoot, opts.Cycle)
	env := predicateEnv(opts.ProjectRoot, changed)

	ctx, cancel := context.WithTimeout(context.Background(), goLaneTimeout(opts.GoTimeout, nil))
	defer cancel()

	var all []Result
	for _, pat := range patterns {
		raw, execErr := goExec(ctx, moduleDir, pat, env)
		results := parseGoTestJSON(strings.NewReader(raw), opts.Cycle)
		if execErr != nil && len(results) == 0 {
			// Nonzero exit with zero test events ⇒ that package did not compile
			// (or `go test` could not run). FAIL loudly, never silent-PASS.
			return nil, fmt.Errorf("acssuite: go predicate scope %q produced no test events but exited "+
				"nonzero (compile error / infra failure): %w\noutput:\n%s", pat, execErr, excerpt(raw))
		}
		all = append(all, results...)
	}
	return all, nil
}

// goEvent is the subset of the `go test -json` event schema we consume.
type goEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// parseGoTestJSON walks `go test -json` NDJSON and maps each test into a Result,
// keyed by Package+"/"+Test so the same test name in two packages stays two
// results (acsrunner keys by bare Test and would collide them — reuse boundary).
// PASS→green/0, FAIL→red/1, SKIP→skip/77. Evidence is captured for red/skip only
// (green carries none — existing invariant). Classification is by package suffix:
// cycle<N> with N==cycle → this-cycle; any other cycle → regression; a redteam
// package → IsRedTeam.
func parseGoTestJSON(r io.Reader, cycle int) []Result {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	type acc struct {
		pkg, test string
		result    string // green|red|skip
		dur       int64
		output    strings.Builder
	}
	byKey := map[string]*acc{}
	order := []string{}
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var ev goEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue // tolerate non-JSON build-output lines
		}
		if ev.Test == "" {
			continue // package-level event
		}
		key := ev.Package + "/" + ev.Test
		a, ok := byKey[key]
		if !ok {
			a = &acc{pkg: ev.Package, test: ev.Test}
			byKey[key] = a
			order = append(order, key)
		}
		switch ev.Action {
		case "output":
			a.output.WriteString(ev.Output)
		case "pass":
			a.result = "green"
			a.dur = int64(ev.Elapsed * 1000)
		case "fail":
			a.result = "red"
			a.dur = int64(ev.Elapsed * 1000)
		case "skip":
			a.result = "skip"
			a.dur = int64(ev.Elapsed * 1000)
		}
	}
	var out []Result
	if err := scanner.Err(); err != nil {
		// A scan error (e.g. a single output line exceeding the buffer) would
		// silently truncate the stream and could drop a later FAIL — a
		// gate-weakening path. Fail LOUD: emit a synthetic RED so the verdict
		// blocks rather than silent-passing on a partial parse.
		out = append(out, Result{
			ACID:            "egps/go-lane-parse-error",
			Predicate:       "egps/go-lane-parse-error",
			ExitCode:        1,
			ResultStr:       "red",
			EvidenceExcerpt: excerpt("go test -json stream parse error (results may be truncated): " + err.Error()),
		})
	}
	for _, key := range order {
		a := byKey[key]
		if a.result == "" {
			continue // saw the test run but never a terminal action; ignore
		}
		dir := path.Base(a.pkg)
		isReg, isRT := classifyGoPkg(dir, cycle)
		r := Result{
			ACID:         dir + "/" + a.test,
			Predicate:    "go/acs/" + dir + "/...:" + a.test,
			DurationMS:   a.dur,
			IsRegression: isReg,
			IsRedTeam:    isRT,
			ResultStr:    a.result,
		}
		switch a.result {
		case "green":
			r.ExitCode = 0
		case "skip":
			r.ExitCode = SkipExitCode
			r.EvidenceExcerpt = excerpt(a.output.String())
		case "red":
			r.ExitCode = 1
			r.EvidenceExcerpt = excerpt(a.output.String())
		}
		out = append(out, r)
	}
	return out
}

// classifyGoPkg maps a predicate package dir to (isRegression, isRedTeam):
// a redteam dir → red-team; cycle<N> with N==cycle → this-cycle (false,false);
// any other dir (other cycle, or non-numeric like cycledefense1) → regression.
func classifyGoPkg(dir string, cycle int) (isRegression, isRedTeam bool) {
	if strings.Contains(dir, "redteam") || strings.Contains(dir, "red-team") {
		return false, true
	}
	if n, ok := cycleNumFromDir(dir); ok && n == cycle {
		return false, false
	}
	return true, false
}

// cycleNumFromDir parses the integer N from a "cycle<N>" package dir. Returns
// (0,false) for non-numeric suffixes (e.g. "cycledefense1").
func cycleNumFromDir(dir string) (int, bool) {
	if !strings.HasPrefix(dir, "cycle") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(dir, "cycle"))
	if err != nil {
		return 0, false
	}
	return n, true
}

var (
	// bashACIDRe matches a bash predicate ACID's (cycle, ac): "cycle-<N>/<NNN>".
	// Anchored on the cycle-N/NNN segment, so it also matches the regression
	// form "regression-suite/cycle-<N>/<NNN>-slug".
	bashACIDRe = regexp.MustCompile(`cycle-(\d+)/0*(\d+)`)
	// goACIDRe matches a Go predicate ACID's (cycle, ac): "cycle<N>/TestC<M>_<NNN>".
	goACIDRe = regexp.MustCompile(`cycle(\d+)/TestC\d+_0*(\d+)`)
)

// doubleCountReds returns synthetic RED results for any (cycle, ac) pair that
// appears in BOTH a bash ACID and a Go ACID — a missed lockstep-delete during
// the Phase-C port. Conservative: ACIDs that don't parse are skipped (they
// cannot collide), so the guard never invents a RED from an unrecognized id.
func doubleCountReds(results []Result) []Result {
	bash := map[[2]int]bool{}
	go_ := map[[2]int]bool{}
	for _, r := range results {
		if m := goACIDRe.FindStringSubmatch(r.ACID); m != nil {
			go_[acKey(m)] = true
			continue
		}
		if m := bashACIDRe.FindStringSubmatch(r.ACID); m != nil {
			bash[acKey(m)] = true
		}
	}
	// Deterministic order: sort the colliding keys.
	var dupes [][2]int
	for k := range bash {
		if go_[k] {
			dupes = append(dupes, k)
		}
	}
	sort.Slice(dupes, func(i, j int) bool {
		if dupes[i][0] != dupes[j][0] {
			return dupes[i][0] < dupes[j][0]
		}
		return dupes[i][1] < dupes[j][1]
	})
	var out []Result
	for _, k := range dupes {
		id := fmt.Sprintf("egps/double-count-cycle%d-ac%d", k[0], k[1])
		out = append(out, Result{
			ACID:      id,
			Predicate: id,
			ExitCode:  1,
			ResultStr: "red",
			EvidenceExcerpt: fmt.Sprintf(
				"cycle %d ac %d is asserted by BOTH a bash predicate and a Go predicate; "+
					"delete the bash .sh in the same change that adds the Go test (Phase-C lockstep)", k[0], k[1]),
		})
	}
	return out
}

// acKey turns a regex submatch [full, cycle, ac] into a (cycle, ac) key.
func acKey(m []string) [2]int {
	c, _ := strconv.Atoi(m[1])
	a, _ := strconv.Atoi(m[2])
	return [2]int{c, a}
}

// discover globs the three predicate roots in a deterministic order.
func discover(root string, cycle int) ([]predFile, error) {
	var out []predFile

	cycleGlob := filepath.Join(root, "acs", fmt.Sprintf("cycle-%d", cycle), "*.sh")
	cyc, err := filepath.Glob(cycleGlob)
	if err != nil {
		return nil, fmt.Errorf("acssuite: glob cycle: %w", err)
	}
	sort.Strings(cyc)
	for _, p := range cyc {
		out = append(out, predFile{path: p})
	}

	regGlob := filepath.Join(root, "acs", "regression-suite", "cycle-*", "*.sh")
	reg, err := filepath.Glob(regGlob)
	if err != nil {
		return nil, fmt.Errorf("acssuite: glob regression: %w", err)
	}
	sort.Strings(reg)
	for _, p := range reg {
		out = append(out, predFile{path: p, isRegression: true})
	}

	rtGlob := filepath.Join(root, "acs", "red-team", "rt-*.sh")
	rt, err := filepath.Glob(rtGlob)
	if err != nil {
		return nil, fmt.Errorf("acssuite: glob red-team: %w", err)
	}
	sort.Strings(rt)
	for _, p := range rt {
		out = append(out, predFile{path: p, isRedTeam: true})
	}
	return out, nil
}

// changedPackagesForCycle returns the go test patterns for the files the
// builder touched this cycle, read from handoff-build.json under the cycle
// workspace (<projectRoot>/.evolve/runs/cycle-<N>/). Best-effort: nil when
// projectRoot is empty or no handoff is found, so predicates fall back to their
// own scope.
func changedPackagesForCycle(projectRoot string, cycle int) []string {
	if projectRoot == "" {
		return nil
	}
	dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		if pkgs := changedpkgs.ChangedPackages(filepath.Join(dir, name)); len(pkgs) > 0 {
			return pkgs
		}
	}
	return nil
}

// runBash executes `bash <path>` and returns its exit code + combined output.
// A non-exec failure (e.g. bash missing) or timeout maps to a non-zero exit so
// the predicate counts as RED — the suite never silently swallows a failure.
//
// The predicate runs in its own process group so a timeout can kill the whole
// tree (a predicate that spawns `sleep` would otherwise hold the output pipe
// open and defeat ctx cancellation). The Cancel hook signals the group on
// timeout; in the narrow window where the deadline fires before Start sets
// cmd.Process the signal is skipped, so WaitDelay is the guaranteed backstop —
// it force-closes the pipes killGrace after cancellation, ensuring Run always
// returns (a timed-out predicate counts RED) rather than blocking on a child.
func runBash(ctx context.Context, path, projectRoot, workdir string, changedPkgs []string) (int, string) {
	cmd := exec.CommandContext(ctx, "bash", path)
	if workdir != "" {
		cmd.Dir = workdir // cycle-190 / issue #9: run predicates with cwd at the shipped tree (the Run closure passes opts.Root)
	}
	// Shared with the Go lane so both lanes export an identical predicate env
	// (EVOLVE_PROJECT_ROOT, CHANGED_PACKAGES). With no extras this equals
	// os.Environ() — the prior inherit behavior.
	cmd.Env = predicateEnv(projectRoot, changedPkgs)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			// Negative pid → signal the whole process group.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = killGrace
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(out)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if code := ee.ExitCode(); code >= 0 {
			return code, string(out)
		}
		// Killed by signal (timeout) → no exit code; treat as RED timeout.
		return 124, string(out) + "\n[acssuite] predicate killed (timeout)"
	}
	// Could not run at all (bash missing, etc.) — RED with diagnostic.
	return 126, string(out) + "\n[acssuite] exec error: " + err.Error()
}

// acIDFromRel derives a stable id from a repo-relative path: "<parent-dir>/<base-without-ext>".
func acIDFromRel(rel string) string {
	rel = strings.TrimSuffix(rel, ".sh")
	return strings.TrimPrefix(rel, "acs/")
}

func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= evidenceMax {
		return s
	}
	return s[:evidenceMax] + "…"
}

// WriteVerdict marshals v to <evolveDir>/runs/cycle-<N>/acs-verdict.json
// atomically (tmp + rename) and returns the path written.
func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dir := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", v.Cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("acssuite: mkdir %s: %w", dir, err)
	}
	dst := filepath.Join(dir, "acs-verdict.json")
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("acssuite: marshal: %w", err)
	}
	// Random tmp suffix (not PID) so concurrent same-process writers to the
	// same cycle dir cannot collide — matches acsrunner.WriteVerdict.
	tmpf, err := os.CreateTemp(dir, "acs-verdict.*.tmp")
	if err != nil {
		return "", fmt.Errorf("acssuite: create tmp: %w", err)
	}
	tmp := tmpf.Name()
	if cerr := tmpf.Close(); cerr != nil {
		return "", fmt.Errorf("acssuite: close tmp: %w", cerr)
	}
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return "", fmt.Errorf("acssuite: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("acssuite: rename: %w", err)
	}
	return dst, nil
}
