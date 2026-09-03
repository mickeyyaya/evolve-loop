// Package acssuite is the deterministic, host-side EGPS predicate-suite runner.
// It executes the Go predicate lane and writes acs-verdict.json conforming to
// the schema the audit + ship gates read (EGPS v11 — see ADR-0042; supersedes
// the bash run-acs-suite.sh of ADR-0025).
//
// The Go lane runs three scopes, each as a SEPARATE `go test -json -tags acs`
// (so a per-package compile error is a HARD error, never a silent PASS):
//   - ./acs/cycle<N>          this cycle's predicates (authored fresh)
//   - ./acs/regression/<sub>  curated durable predicates, every cycle
//   - ./acs/redteam           standing anti-gaming predicates, every cycle
//
// Each test maps to a Result via v.record. red_count == 0 ⇒ verdict PASS ⇒
// ship_eligible. A test that FAILs is RED; a t.Skip is SKIP (the TAP/automake
// convention, exit 77 in the Result) — an evidence-absent predicate (e.g. a
// runtime-only regression predicate on a fresh clone) is counted neither red nor
// green, so it cannot block the gate yet cannot fake a pass. CLI:
// `evolve acs suite --cycle N`.
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
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

// DefaultTimeout bounds the whole Go lane (per scope) via context cancellation.
const DefaultTimeout = 60 * time.Second

// evidenceMax caps the captured output excerpt per predicate.
const evidenceMax = 600

// maxLockWait bounds the single-flight queue wait: generous enough for a
// full sibling suite run, finite so a wedged holder degrades this lane to
// unserialized instead of deadlocking the fleet.
const maxLockWait = 15 * time.Minute

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
	// FailingTests names the `--- FAIL:` tests found in this predicate's
	// output — for a meta-predicate that shells an inner `go test`, these are
	// the INNER failures, the exact identity the excerpt cap used to destroy
	// (cycles 1107/1116/1123: head-truncated evidence left no failing test
	// name anywhere on disk, making their false reds permanently
	// unconfirmable). Deduped, bounded by maxFailingTests.
	FailingTests []string `json:"failing_tests,omitempty"`
	// EvidenceNote records WHY no failing test could be named on a red
	// (compile failure, timeout, signal) — a red must never be a
	// content-free exit code.
	EvidenceNote string `json:"evidence_note,omitempty"`
	// PhantomBindings names bound tests this red predicate demanded that NEVER
	// RAN — reported did-NOT-pass while absent from FailingTests, i.e. the
	// bound name no longer resolves in its target package (renamed away, or
	// never created). Distinct from a FAILING bound test on purpose: the cure
	// for a phantom is repointing the binding, not fixing code, and a red that
	// cannot say which of the two it is costs a forensic session
	// (phantom_binding.go; the 1539-1546 absorbing streak).
	PhantomBindings []string `json:"phantom_bindings,omitempty"`
	// Flaky marks a predicate that was RED on the first run and GREEN on the
	// single bounded retry (cycle-468): value "passed-on-retry". Visible in
	// the wire JSON so a flake is never silently absorbed; the first-run
	// failure evidence is retained deliberately (the flake's signature).
	// The meaning is PINNED by acs/cycle468 (deterministic reds carry no
	// flaky key) — retry outcomes for reds live in RetryOutcome instead.
	Flaky string `json:"flaky,omitempty"`

	// RetryOutcome records what the bounded retry established about a red
	// that STAYED red: "red-on-retry" (the retry ran and confirmed) or
	// "retry-inconclusive" (the retry produced no result for this test —
	// expired ctx, crash). Absent on greens, skips, and absorbed flakes.
	// Without it a confirmed red was indistinguishable from a starved
	// retry (batch-18 forensics gap).
	RetryOutcome string `json:"retry_outcome,omitempty"`

	// fullEvidence retains the red predicate's COMPLETE captured stream
	// (unexported: the wire JSON stays capped at evidenceMax — the full
	// record lands in acs-red-evidence/ beside the verdict instead; cycles
	// 1173/1175/1178 were undiagnosable from the elided excerpt alone).
	fullEvidence string
}

// PredicateSuite is the count breakdown.
type PredicateSuite struct {
	ThisCycleCount       int `json:"this_cycle_count"`
	RegressionSuiteCount int `json:"regression_suite_count"`
	RedTeamCount         int `json:"red_team_count"`
	SkippedCount         int `json:"skipped_count"`
	Total                int `json:"total"`
}

// warningsFromFlaky derives the verdict-level warnings from flaky-annotated
// results — single source: the per-result Flaky field is authoritative and the
// warning list is its projection (never maintained separately).
func warningsFromFlaky(results []Result) []string {
	var w []string
	for _, r := range results {
		// Only absorbed flakes surface as warnings — acs/cycle468 pins that a
		// deterministic red adds NO warnings entry; RetryOutcome + the
		// acs-red-evidence/ file are the confirmed-red forensic surface.
		if r.Flaky != "" {
			w = append(w, fmt.Sprintf("flaky: %s passed-on-retry (bounded single retry absorbed a non-deterministic red; investigate under host contention)", r.ACID))
		}
	}
	return w
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
	// Warnings surfaces non-blocking anomalies (cycle-468: flaky predicates
	// that passed on the bounded retry). Projection of Result.Flaky.
	Warnings []string `json:"warnings,omitempty"`
	// SuiteRoot / ProjectRoot record which roots this verdict was minted
	// under (cycle-1434: a verdict minted with the WRONG state root red'd 3
	// predicates the correct-root run showed green, and nothing in the
	// artifact said so). omitempty: verdicts written before these stamps
	// stay byte-compatible, and readers treat absence as "unstamped", never
	// as a mismatch.
	SuiteRoot   string `json:"suite_root,omitempty"`
	ProjectRoot string `json:"project_root,omitempty"`
}

// Options configures Run. Root and Cycle are required.
type Options struct {
	Root  string // repo root (the Go module's parent; the lane runs from <Root>/go)
	Cycle int    // current cycle number
	// ProjectRoot is the MAIN project root whose `.evolve/` holds the runtime data
	// (history under .evolve/runs/, baselines, the current build-report) that
	// predicates read via ${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}. When set, it is
	// exported as EVOLVE_PROJECT_ROOT to each predicate so a suite run from a
	// worktree (Root=worktree, post issue-#9 audit-cwd=worktree) still resolves
	// `.evolve/` to main rather than the worktree (where `.evolve/` is absent).
	// Empty → predicates inherit the caller's env. (issue #12)
	ProjectRoot string
	// GoModuleDir is the directory holding go.mod + the acs/ predicate subtree.
	// Empty → filepath.Join(Root, "go"). The Go lane runs
	// `go test -json -tags acs -count=1 <scope>` from here.
	GoModuleDir string
	// GoTimeout bounds the WHOLE Go lane via context cancellation (not
	// per-predicate; Go compiles per package). 0 → EVOLVE_ACS_GO_TIMEOUT_S
	// (seconds) when set, else DefaultTimeout.
	GoTimeout time.Duration
	// GoExec runs ONE Go predicate-lane package pattern and returns the raw
	// `go test -json` output plus the process exit error (nil on exit 0, an
	// *exec.ExitError on nonzero). It is called once per active scope
	// (current-cycle, each regression sub-package, redteam). Injected by tests;
	// nil → defaultGoExec.
	GoExec func(ctx context.Context, moduleDir, pkgPattern string, env []string) (rawJSON string, err error)
}

// Run executes the Go predicate lane (current cycle + regression + redteam
// scopes, each a separate `go test -json -tags acs`) and returns the Verdict.
// A non-compiling predicate package is a HARD error, never a silent PASS.
func Run(opts Options) (Verdict, error) {
	if opts.Root == "" {
		return Verdict{}, fmt.Errorf("acssuite: Root required")
	}
	if opts.Cycle <= 0 {
		return Verdict{}, fmt.Errorf("acssuite: Cycle must be > 0")
	}

	v := Verdict{SchemaVersion: "1.0", Cycle: opts.Cycle, SuiteRoot: opts.Root, ProjectRoot: opts.ProjectRoot}

	// ADR-0080 P1: the suite execution is host-wide SINGLE-FLIGHT. Fleet
	// lanes are separate processes each shelling full package suites; run
	// concurrently they oversubscribe the host and turn long suites into
	// false reds (batch-16: TouchedPackagesStayGreen red on 1166/1167/1169,
	// green in the preserved worktree — an identical-fingerprint halt over
	// infrastructure). Blocking is bounded by the caller's lifetime only:
	// verification MUST run, serialized, never skipped.
	// Bounded wait (review HIGH): a wedged HOLDER must degrade this lane to
	// unserialized (WARN below), never deadlock the fleet — the whole degrade
	// path exists for exactly that, and an unbounded Background ctx made it
	// unreachable.
	lockCtx, lockCancel := context.WithTimeout(context.Background(), maxLockWait)
	defer lockCancel()
	release, lockErr := verifylock.Acquire(lockCtx, opts.Root, os.Stderr)
	if lockErr != nil {
		fmt.Fprintf(os.Stderr, "[acs] WARN: verification single-flight unavailable (%v) — running unserialized\n", lockErr)
	} else {
		defer release()
	}

	goResults, gErr := runGoTest(opts)
	if gErr != nil {
		return Verdict{}, gErr
	}
	// After demoteOutOfScope: a red demoted to skip must leave no phantom
	// red-evidence file (review M2 — the forensic surface must match the
	// verdict).
	demoteWarnings := demoteOutOfScope(goResults, opts)
	writeRedEvidence(opts, goResults)
	for _, r := range goResults {
		v.record(r)
	}

	v.PredicateSuite.SkippedCount = v.SkipCount
	v.PredicateSuite.Total = len(v.Results) // skips included
	// Invariant: a skip increments neither GreenCount nor RedCount, so
	// PASS ⟺ red_count==0 holds.
	if v.RedCount == 0 {
		v.Verdict = "PASS"
		v.ShipEligible = true
	} else {
		v.Verdict = "FAIL"
	}
	v.Warnings = append(warningsFromFlaky(v.Results), demoteWarnings...)
	return v, nil
}

// record appends a result and updates the green/red/skip tallies + the
// PredicateSuite bucketing — the single place RedCount is incremented, so the
// gate invariant (red_count==0 ⟺ PASS) has one source of truth.
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

// predicateEnv builds the env exported to BOTH lanes. The dual-root pattern:
//   - EVOLVE_PROJECT_ROOT (STATE root) → MAIN, so predicates resolve `.evolve/`
//     runtime data to main even from a worktree (issue #12).
//   - EVOLVE_WORKTREE_ROOT (SOURCE root) → the cycle's worktree, so predicates
//     that validate a generated-from-source doc (e.g. `evolve flags check` /
//     `evolve skills check`) read the WORKTREE artifact the cycle commits — not
//     main's stale working copy. Without this, such a predicate red-fails correct
//     work because the doc only reaches main at ship, after audit (cycle-355).
//   - CHANGED_PACKAGES → the cycle's touched packages, so a predicate can scope
//     `go test` (cycle-200).
//
// With no extras it equals os.Environ() — the prior inherit behavior.
func predicateEnv(projectRoot, worktreeRoot string, changedPkgs []string) []string {
	env := os.Environ()
	if projectRoot != "" {
		env = append(env, "EVOLVE_PROJECT_ROOT="+projectRoot)
	}
	if worktreeRoot != "" {
		env = append(env, ipcenv.WorktreeRootKey+"="+worktreeRoot)
	}
	if len(changedPkgs) > 0 {
		// Space-joining is safe: go package patterns never contain spaces.
		env = append(env, "CHANGED_PACKAGES="+strings.Join(changedPkgs, " "))
	}
	return env
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
	// WaitDelay (review HIGH): CommandContext kills the direct `go` process,
	// not test-binary grandchildren — and meta-predicates shell inner go-test
	// runs. Without a delay a surviving grandchild pins CombinedOutput's pipe
	// past ctx expiry, the lane never returns, and (single-flight) the held
	// lock starves every other lane. Same discipline as the sandbox and fleet
	// launchers.
	cmd.WaitDelay = 30 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// goLaneTimeout returns the whole-lane timeout: optsTimeout when > 0, else
// cfg.GoTimeoutS (seconds) when > 0, else DefaultTimeout. The Go lane
// is bounded as a whole (via context cancellation in runGoTest) because Go
// compiles per package; the current-cycle scope runs a single package, so one
// DefaultTimeout is the right ceiling.
func goLaneTimeout(optsTimeout time.Duration, cfg policy.ACSConfig) time.Duration {
	if optsTimeout > 0 {
		return optsTimeout
	}
	if cfg.GoTimeoutS > 0 {
		return time.Duration(cfg.GoTimeoutS) * time.Second
	}
	return DefaultTimeout
}

// currentCycleGoPkgDir is the Go predicate package dir for the current cycle:
// <moduleDir>/acs/cycle<N>.
func currentCycleGoPkgDir(moduleDir string, cycle int) string {
	return filepath.Join(moduleDir, filepath.FromSlash(CyclePackage(cycle)))
}

// CyclePackage is the ONE spelling of a cycle's ACS predicate package as a Go
// package pattern relative to the module (`./acs/cycle<N>`). The suite lane,
// the scope lint and the Task Contract's predicate inventory (core) all derive
// from it, so the convention cannot drift between the writer and its readers.
func CyclePackage(cycle int) string { return fmt.Sprintf("./acs/cycle%d", cycle) }

// currentCycleGoPkgExists reports whether the current cycle has a Go predicate
// package on disk. When absent, the Go lane is a no-op (not an error) — the
// cycle simply has no Go ACs yet.
func currentCycleGoPkgExists(moduleDir string, cycle int) bool {
	fi, err := os.Stat(currentCycleGoPkgDir(moduleDir, cycle))
	return err == nil && fi.IsDir()
}

// goLanePatterns returns the existence-gated, NON-recursive package patterns the
// Go lane runs every cycle — the three predicate scopes:
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
		pats = append(pats, CyclePackage(cycle))
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
// sub-packages + redteam) as a SEPARATE pattern, merging results. The current
// cycle + curated regression + red-team are run — never every historical cycle,
// which would drag bit-rotted predicates into the gate.
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
	// opts.Root is the cycle's worktree (resolveACSSuiteRoot → active_worktree);
	// export it as EVOLVE_WORKTREE_ROOT so source/doc predicates validate the
	// committed worktree artifact, not main's stale copy (cycle-355 fix).
	env := predicateEnv(opts.ProjectRoot, opts.Root, changed)

	pol, _ := policy.Load(filepath.Join(opts.ProjectRoot, ".evolve", "policy.json"))
	ctx, cancel := context.WithTimeout(context.Background(), goLaneTimeout(opts.GoTimeout, pol.ACSTimeoutConfig()))
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
		all = append(all, retryFlakyReds(ctx, goExec, moduleDir, pat, env, results, opts.Cycle)...)
	}
	return all, nil
}

// retryFlakyReds is the cycle-468 bounded flake absorber: when a scope's first
// run produced >=1 RED and NO red is the synthetic egps/ parse-error red
// (any such red marks the whole stream untrustworthy and suppresses the
// retry entirely — a truncated stream is not retryable evidence), the
// scope is re-run EXACTLY ONCE. A red that passes on the retry
// flips to green with the visible Flaky="passed-on-retry" annotation (first-run
// evidence retained — the flake's signature); a red that stays red keeps its
// first-run result. Greens/skips and the result set's size are untouched (the
// retry can only flip existing reds, never add or duplicate results). Rationale:
// parallel_evaluate=enforce runs the -race predicate suites under concurrent
// evaluate-phase host load; contention flakes burned 4 verified-good cycles
// (444/447/466/467). The gate is not weakened: the retry is bounded to one,
// annotated on the wire, and surfaced as a verdict warning.
func retryFlakyReds(ctx context.Context, goExec func(context.Context, string, string, []string) (string, error), moduleDir, pat string, env []string, results []Result, cycle int) []Result {
	hasTestRed := false
	for _, r := range results {
		if r.ResultStr != "red" {
			continue
		}
		if strings.HasPrefix(r.ACID, "egps/") {
			// Synthetic infra red: the stream itself is untrustworthy — never retry.
			return results
		}
		hasTestRed = true
	}
	if !hasTestRed {
		return results
	}
	raw, _ := goExec(ctx, moduleDir, pat, env) // bounded: exactly one retry
	retry := parseGoTestJSON(strings.NewReader(raw), cycle)
	greenOnRetry := make(map[string]bool, len(retry))
	retryRan := make(map[string]bool, len(retry))
	retryEvidence := make(map[string]string, len(retry))
	for _, r := range retry {
		retryRan[r.ACID] = true
		if r.ResultStr == "green" {
			greenOnRetry[r.ACID] = true
		}
		if r.fullEvidence != "" {
			retryEvidence[r.ACID] = r.fullEvidence
		}
	}
	for i := range results {
		if results[i].ResultStr != "red" {
			continue
		}
		switch {
		case greenOnRetry[results[i].ACID]:
			results[i].ResultStr = "green"
			results[i].ExitCode = 0
			results[i].Flaky = "passed-on-retry"
		case retryRan[results[i].ACID]:
			// Red stayed red: record it in RetryOutcome (NOT Flaky — its
			// passed-on-retry meaning is pinned by acs/cycle468) and keep
			// the retry's stream too.
			results[i].RetryOutcome = "red-on-retry"
			if re := retryEvidence[results[i].ACID]; re != "" {
				results[i].fullEvidence += "\n--- RETRY RUN (still red) ---\n" + re
			}
		default:
			// The retry produced NO result for this test (expired ctx, crash
			// before it ran): inconclusive, not confirmed.
			results[i].RetryOutcome = "retry-inconclusive"
		}
	}
	return results
}

// writeRedEvidence persists each red predicate's COMPLETE captured stream
// (first run + any retry, see retryFlakyReds) to
// <workspace>/acs-red-evidence/<ac_id>.txt, ProjectRoot preferred over Root
// (the same convention the verdict's own location uses). Each file opens
// with a `# cycle=… ac_id=… run=…` header so repeated Runs in one cycle
// stay attributable; a name collision (duplicate ACIDs across scope dirs
// share a path.Base) gets a numeric suffix instead of a silent overwrite.
// Best-effort and loud: a write failure WARNs and never blocks the verdict.
func writeRedEvidence(opts Options, results []Result) {
	root := opts.ProjectRoot
	if root == "" {
		root = opts.Root
	}
	dir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", opts.Cycle), "acs-red-evidence")
	for _, r := range results {
		if r.ResultStr != "red" || r.fullEvidence == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence dir: %v\n", err)
			return
		}
		body := fmt.Sprintf("# cycle=%d ac_id=%s predicate=%s run=%s\n--- FIRST RUN ---\n%s",
			opts.Cycle, r.ACID, r.Predicate, time.Now().UTC().Format(time.RFC3339), r.fullEvidence)
		base := strings.ReplaceAll(r.ACID, "/", "__")
		collisions := 0
		for i := 0; i < 10; i++ {
			name := base + ".txt"
			if i > 0 {
				name = fmt.Sprintf("%s-%d.txt", base, i+1)
			}
			f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if os.IsExist(err) {
				collisions++
				continue
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence write %s: %v\n", name, err)
				break
			}
			_, werr := f.WriteString(body)
			if cerr := f.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence write %s: %v\n", name, werr)
			}
			break
		}
		if collisions == 10 {
			fmt.Fprintf(os.Stderr, "[acs] WARN: red-evidence: 10 name collisions for %s — evidence dropped\n", base)
		}
	}
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
			full := a.output.String()
			r.fullEvidence = full
			r.EvidenceExcerpt = excerpt(full)
			// Extract inner failing-test identity from the FULL output,
			// before the excerpt cap can destroy it. The predicate's own
			// name is excluded — it is already the ACID; the inner names
			// are the diagnosis.
			for _, name := range extractFailingTests(full) {
				if name != a.test {
					r.FailingTests = append(r.FailingTests, name)
				}
			}
			if len(r.FailingTests) == 0 && !strings.Contains(full, "--- FAIL: "+a.test) {
				r.EvidenceNote = "no `--- FAIL:` marker in output — compile failure, timeout, or signal; see excerpt tail"
			}
			r.PhantomBindings = phantomBindings(full, r.FailingTests)
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

// excerptHead is the slice of evidenceMax kept from the FRONT of over-limit
// output: a predicate's own t.Fatalf line — the author's diagnosis with its
// file:line — prints first. The remainder comes from the TAIL, where go test
// accumulates `--- FAIL:` detail; pure head-truncation left cycles
// 1107/1116/1123 with reds whose failing test name survives nowhere on disk.
const excerptHead = 200

// excerpt caps s at ~evidenceMax as head+"…"+tail (see excerptHead). Both cut
// points are re-anchored to valid UTF-8 so a mid-rune slice cannot leak
// mojibake into the verdict JSON.
func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= evidenceMax {
		return s
	}
	head := strings.ToValidUTF8(s[:excerptHead], "")
	tail := strings.ToValidUTF8(s[len(s)-(evidenceMax-excerptHead):], "")
	return head + "…" + tail
}

// maxFailingTests bounds Result.FailingTests so a mass failure cannot bloat
// the verdict JSON; the excerpt tail still shows the overflow.
const maxFailingTests = 8

// failLineRE matches a go-test failure marker at any nesting depth — including
// inner-subprocess output a meta-predicate t.Logf'd as free-form text.
var failLineRE = regexp.MustCompile(`--- FAIL: (\S+)`)

// extractFailingTests returns the deduped, order-preserving, bounded list of
// test names behind every `--- FAIL:` in s.
func extractFailingTests(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range failLineRE.FindAllStringSubmatch(s, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) == maxFailingTests {
			break
		}
	}
	return out
}

var (
	writeVerdictCreateTemp = os.CreateTemp
	writeVerdictClose      = func(f *os.File) error { return f.Close() }
	writeVerdictWriteFile  = os.WriteFile
)

// VerdictFilename is the canonical acs-verdict artifact name. Exported so the
// readers and retirement paths in other packages (core, phases, router) can
// project from ONE spelling instead of re-typing the literal — the audit-report
// name already has this via phasecontract.ArtifactFilename, and the verdict
// name had drifted into 7+ copies before this const existed.
const VerdictFilename = "acs-verdict.json"

// WriteVerdict marshals v to <evolveDir>/runs/cycle-<N>/acs-verdict.json
// atomically (tmp + rename) and returns the path written.
func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dir := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", v.Cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("acssuite: mkdir %s: %w", dir, err)
	}
	dst := filepath.Join(dir, VerdictFilename)
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("acssuite: marshal: %w", err)
	}
	// Random tmp suffix (not PID) so concurrent same-process writers to the
	// same cycle dir cannot collide — matches acsrunner.WriteVerdict.
	tmpf, err := writeVerdictCreateTemp(dir, "acs-verdict.*.tmp")
	if err != nil {
		return "", fmt.Errorf("acssuite: create tmp: %w", err)
	}
	tmp := tmpf.Name()
	if cerr := writeVerdictClose(tmpf); cerr != nil {
		return "", fmt.Errorf("acssuite: close tmp: %w", cerr)
	}
	if err := writeVerdictWriteFile(tmp, buf, 0o644); err != nil {
		return "", fmt.Errorf("acssuite: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", fmt.Errorf("acssuite: rename: %w", err)
	}
	return dst, nil
}
