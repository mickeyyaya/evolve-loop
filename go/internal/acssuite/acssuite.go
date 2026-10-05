// Package acssuite is the deterministic, host-side EGPS predicate-suite
// runner: it runs the Go predicate lane and writes acs-verdict.json to the
// schema the audit and ship gates read.
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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

const DefaultTimeout = 60 * time.Second

const evidenceMax = 600

var suiteLockWait = verifylock.MaxWait

// SkipExitCode is the TAP/automake SKIP convention: exit 77 means evidence
// absent / not-applicable, and is counted neither red nor green.
const SkipExitCode = 77

// Result is one predicate's outcome, part of the acs-verdict.json schema.
type Result struct {
	ACID            string `json:"ac_id"`
	Predicate       string `json:"predicate"` // repo-relative path
	ExitCode        int    `json:"exit_code"`
	ResultStr       string `json:"result"` // "green" | "red" | "skip"
	DurationMS      int64  `json:"duration_ms"`
	IsRegression    bool   `json:"is_regression"`
	IsRedTeam       bool   `json:"is_red_team,omitempty"`
	EvidenceExcerpt string `json:"evidence_excerpt,omitempty"`
	// FailingTests names the `--- FAIL:` tests inside this predicate's output —
	// for a meta-predicate that shells an inner `go test`, these are the INNER
	// failures. Deduped, bounded by maxFailingTests.
	FailingTests []string `json:"failing_tests,omitempty"`
	// EvidenceNote records WHY no failing test could be named on a red
	// (compile failure, timeout, signal) — a red must never be a
	// content-free exit code.
	EvidenceNote string `json:"evidence_note,omitempty"`
	// PhantomBindings names bound tests this red predicate demanded that NEVER
	// RAN — reported did-NOT-pass while absent from FailingTests, i.e. the
	// bound name no longer resolves in its target package (renamed away, or
	// never created). Distinct from a FAILING bound test on purpose: the cure
	// for a phantom is repointing the binding, not fixing code.
	PhantomBindings []string `json:"phantom_bindings,omitempty"`
	// Flaky marks a predicate red on the first run and green on the single
	// bounded retry: value "passed-on-retry". Retry outcomes for a red that
	// stays red live in RetryOutcome instead.
	Flaky string `json:"flaky,omitempty"`

	// RetryOutcome records what the bounded retry established about a red
	// that STAYED red: "red-on-retry" (the retry ran and confirmed) or
	// "retry-inconclusive" (the retry produced no result for this test —
	// expired ctx, crash). Absent on greens, skips, and absorbed flakes.
	RetryOutcome string `json:"retry_outcome,omitempty"`

	// fullEvidence retains the red predicate's complete captured stream: the
	// wire JSON stays capped at evidenceMax, and the full record lands in
	// acs-red-evidence/ beside the verdict instead.
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

// warningsFromFlaky projects Result.Flaky into verdict-level warnings — the
// single source of truth; the list is never maintained separately.
func warningsFromFlaky(results []Result) []string {
	var w []string
	for _, r := range results {
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
	// Warnings surfaces non-blocking anomalies: flaky predicates that passed
	// on the bounded retry. Projection of Result.Flaky.
	Warnings []string `json:"warnings,omitempty"`
	// SuiteRoot / ProjectRoot record which roots this verdict was minted
	// under. omitempty: verdicts written before these stamps stay
	// byte-compatible, and readers treat absence as "unstamped", never as a
	// mismatch.
	SuiteRoot   string `json:"suite_root,omitempty"`
	ProjectRoot string `json:"project_root,omitempty"`
}

// Options configures Run. Root and Cycle are required.
type Options struct {
	Root  string // repo root (the Go module's parent; the lane runs from <Root>/go)
	Cycle int    // current cycle number
	// ProjectRoot is the MAIN project root whose `.evolve/` holds the runtime
	// data predicates read via ${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}. When set,
	// it is exported as EVOLVE_PROJECT_ROOT to each predicate so a suite run
	// from a worktree still resolves `.evolve/` to main rather than the
	// worktree (where `.evolve/` is absent). Empty → predicates inherit the
	// caller's env.
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
func Run(opts Options) (Verdict, error) {
	opts, err := resolveOptions(opts)
	if err != nil {
		return Verdict{}, err
	}
	release := acquireSuiteLock(opts.Root)
	defer release()

	cfg, refusals := loadLaneConfig(opts.stateRoot())
	results := runGoTest(opts, cfg)
	// A red demoted to skip must leave no phantom red-evidence file — the
	// forensic surface must match the verdict.
	demoteWarnings := demoteOutOfScope(results, opts)
	writeRedEvidence(opts, results)
	v := Verdict{SchemaVersion: "1.0", Cycle: opts.Cycle, SuiteRoot: opts.Root, ProjectRoot: opts.ProjectRoot}
	for _, r := range results {
		v.record(r)
	}
	v.PredicateSuite.SkippedCount = v.SkipCount
	v.PredicateSuite.Total = len(v.Results)
	v.ShipEligible = v.RedCount == 0
	v.Verdict = "FAIL"
	if v.ShipEligible {
		v.Verdict = "PASS"
	}
	v.Warnings = slices.Concat(warningsFromFlaky(v.Results), demoteWarnings, refusals)
	return v, nil
}

func resolveOptions(opts Options) (Options, error) {
	if opts.Root == "" {
		return Options{}, fmt.Errorf("acssuite: Root required")
	}
	if opts.Cycle <= 0 {
		return Options{}, fmt.Errorf("acssuite: Cycle must be > 0")
	}
	for _, p := range []*string{&opts.Root, &opts.ProjectRoot, &opts.GoModuleDir} {
		if *p == "" {
			continue
		}
		abs, err := filepath.Abs(*p)
		if err != nil {
			return Options{}, fmt.Errorf("acssuite: resolve %q to an absolute path: %w", *p, err)
		}
		*p = abs
	}
	return opts, nil
}

func (o Options) stateRoot() string {
	if o.ProjectRoot != "" {
		return o.ProjectRoot
	}
	return o.Root
}

func acquireSuiteLock(root string) func() {
	// The suite execution is host-wide SINGLE-FLIGHT: verification MUST run,
	// serialized, never skipped. A wedged holder degrades this lane to
	// unserialized (WARN below) rather than deadlock the fleet.
	// See ADR-0080.
	release, lockErr := verifylock.AcquireWithin(context.Background(), root, suiteLockWait, os.Stderr)
	if lockErr != nil {
		fmt.Fprintf(os.Stderr, "[acs] WARN: verification single-flight unavailable (%v) — running unserialized\n", lockErr)
		return func() {}
	}
	return release
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

const (
	projectRootKey     = "EVOLVE_PROJECT_ROOT"
	changedPackagesKey = "CHANGED_PACKAGES"
)

func suiteExportKeys() []string {
	return []string{projectRootKey, ipcenv.WorktreeRootKey, changedPackagesKey}
}

func forwardableKeys(named []string) (forward, refusals []string) {
	owned := slices.Concat(ipcenv.ProtocolKeys(), suiteExportKeys())
	for _, key := range named {
		if slices.Contains(owned, key) {
			refusals = append(refusals, fmt.Sprintf("acs.predicate_env names %s, a lane protocol key or one of the suite's own exports; it is not forwarded", key))
			continue
		}
		forward = append(forward, key)
	}
	return forward, refusals
}

type predicateExports struct {
	stateRoot    string
	sourceRoot   string
	changedPkgs  []string
	operatorKeys []string
}

func predicateEnv(x predicateExports) []string {
	host := os.Environ()
	env := append(entriesNotNamed(ipcenv.Scrub(host), suiteExportKeys()), entriesNamed(host, x.operatorKeys)...)
	if x.stateRoot != "" {
		env = append(env, projectRootKey+"="+x.stateRoot)
	}
	if x.sourceRoot != "" {
		env = append(env, ipcenv.WorktreeRootKey+"="+x.sourceRoot)
	}
	if len(x.changedPkgs) > 0 {
		// Space-joining is safe: go package patterns never contain spaces.
		env = append(env, changedPackagesKey+"="+strings.Join(x.changedPkgs, " "))
	}
	return env
}

func entriesNamed(environ, keys []string) []string {
	return slices.DeleteFunc(slices.Clone(environ), func(entry string) bool { return !slices.Contains(keys, envKey(entry)) })
}

func entriesNotNamed(environ, keys []string) []string {
	return slices.DeleteFunc(slices.Clone(environ), func(entry string) bool { return slices.Contains(keys, envKey(entry)) })
}

func envKey(entry string) string {
	key, _, _ := strings.Cut(entry, "=")
	return key
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
	// CommandContext kills only the direct `go` process, not test-binary
	// grandchildren a meta-predicate's inner `go test` spawns; without a
	// delay a surviving grandchild pins CombinedOutput's pipe past ctx
	// expiry and the held single-flight lock starves every other lane.
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

// goLanePatterns returns the existence-gated, non-recursive package patterns
// the Go lane runs each cycle: the current cycle's package, each regression
// sub-package, and the red-team package. Patterns whose dir is absent are
// skipped.
// See ADR-0042.
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

// writeRedEvidence persists each red predicate's COMPLETE captured stream
// (first run + any retry, see retryFlakyReds) to
// <workspace>/acs-red-evidence/<ac_id>.txt, ProjectRoot preferred over Root
// (the same convention the verdict's own location uses). Each file opens
// with a `# cycle=… ac_id=… run=…` header so repeated Runs in one cycle
// stay attributable; a name collision (duplicate ACIDs across scope dirs
// share a path.Base) gets a numeric suffix instead of a silent overwrite.
// Best-effort and loud: a write failure WARNs and never blocks the verdict.
func writeRedEvidence(opts Options, results []Result) {
	dir := filepath.Join(opts.stateRoot(), ".evolve", "runs", fmt.Sprintf("cycle-%d", opts.Cycle), "acs-red-evidence")
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
			ACID:            acsverdict.SyntheticRedPrefix + "go-lane-parse-error",
			Predicate:       acsverdict.SyntheticRedPrefix + "go-lane-parse-error",
			ExitCode:        1,
			ResultStr:       "red",
			EvidenceExcerpt: excerpt("go test -json stream parse error (results may be truncated): " + err.Error()),
		})
	}
	for _, key := range order {
		a := byKey[key]
		if a.result == "" {
			out = append(out, Result{
				ACID:      acsverdict.SyntheticRedPrefix + "go-lane-incomplete/" + a.pkg + "/" + a.test,
				Predicate: acsverdict.SyntheticRedPrefix + "go-lane-incomplete", ExitCode: 1, ResultStr: "red",
				EvidenceExcerpt: "predicate started without a terminal result: " + a.pkg + "/" + a.test,
			})
			continue
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
// accumulates `--- FAIL:` detail.
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

func WriteVerdict(evolveDir string, v Verdict) (string, error) {
	dst, err := acsverdict.Path(evolveDir, v.Cycle)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("acssuite: mkdir %s: %w", dir, err)
	}
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
