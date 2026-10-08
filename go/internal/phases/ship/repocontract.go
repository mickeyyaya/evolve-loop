package ship

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var repoContractPackages = repocontract.Packages()

var repoContractSelections = repocontract.TreeReadingTests()

const packWaitDelay = 2 * time.Second

// scanLogName is the run-dir artifact every scanner-pack run is teed to —
// green runs included, since a green baseline is what disproves a false RED.
// Kept unexported: nothing outside this package consumes the name today.
const scanLogName = "ship-repocontract-scan.log"

// packOutcome is one classified scanner-pack run. failures is non-empty
// ONLY when the pack itself said "your code is broken" — a `go test -json`
// fail event carrying a test name, or a build/setup failure. An err with an
// EMPTY failures is the ambiguous case: the toolchain exited nonzero
// without any guard suite reporting a violation.
type packOutcome struct {
	failures   []packFailure
	failureLog string
	err        error
}

// packFailure is one classified failure: a named test of a package, or a build failure (Test empty).
type packFailure struct{ Package, Test string }

func (f packFailure) String() string {
	if f.Test == "" {
		return f.Package
	}
	return f.Package + "." + f.Test
}

func (o packOutcome) green() bool   { return o.err == nil }
func (o packOutcome) realRed() bool { return o.err != nil && len(o.failures) > 0 }

func (o packOutcome) failedNames() []string {
	names := make([]string, 0, len(o.failures))
	for _, f := range o.failures {
		names = append(names, f.String())
	}
	return names
}

// allNamedTests reports whether every failure is a named test, so each can be re-run by name.
func (o packOutcome) allNamedTests() bool {
	if len(o.failures) == 0 {
		return false
	}
	for _, f := range o.failures {
		if f.Test == "" {
			return false
		}
	}
	return true
}

// repoContractTestFn is the seam for the pack execution (package var, mirrors
// the runner seams elsewhere in this package's tests). Production runs
// `go test -json` in the lane worktree's module dir.
var repoContractTestFn = defaultRepoContractTest

func defaultRepoContractTest(ctx context.Context, moduleDir string, out packLog) packOutcome {
	whole := runRepoContractPackages(ctx, moduleDir, out, repoContractPackages)
	return whole.merged(runGoTestJSON(ctx, moduleDir, out, repoContractSelectionArgs(repoContractSelections)))
}

func (o packOutcome) merged(next packOutcome) packOutcome {
	return packOutcome{
		failures:   append(append([]packFailure{}, o.failures...), next.failures...),
		failureLog: o.failureLog + next.failureLog,
		err:        errors.Join(o.err, next.err),
	}
}

func repoContractSelectionArgs(selections []repocontract.TestSelection) []string {
	var packages, tests []string
	for _, selection := range selections {
		packages = append(packages, selection.Package)
		tests = append(tests, selection.Tests...)
	}
	slices.Sort(tests)
	args := append(repoContractTestArgs(nil, nil), "-run", "^("+strings.Join(slices.Compact(tests), "|")+")$")
	return append(args, packages...)
}

func RunRepoContractPack(ctx context.Context, root string) (reds []string, diagnostic string, err error) {
	var out strings.Builder
	o := repoContractTestFn(ctx, repocontract.ModuleDir(root), packLog{notes: &out, raw: &out})
	switch {
	case o.realRed():
		return o.failedNames(), o.failureLog, o.err
	case o.green():
		return nil, "", nil
	}
	return nil, out.String(), o.err
}

func repoContractSuiteNames() []string {
	names := make([]string, 0, len(repoContractPackages))
	for _, pattern := range repoContractPackages {
		names = append(names, path.Base(strings.TrimSuffix(pattern, "/...")))
	}
	return names
}

func runRepoContractPackages(ctx context.Context, moduleDir string, out packLog, packages []string) packOutcome {
	return runRepoContractPackagesWithTags(ctx, moduleDir, out, packages, nil)
}

// repoContractTestTimeout is the per-binary deadline the gate hands `go test`,
// well above Go's 10m default: a tight deadline under fleet load can make
// `go test -json` emit a fail event for the running test and every paused
// t.Parallel() test, which the gate would then class as a real contract RED
// on green code. Raising the deadline can only turn a timeout into a real
// verdict; it can never turn a failing test green.
const repoContractTestTimeout = addedtests.PackageTimeout

// repoContractTestArgs builds the gate's `go test` argv. Split out from the
// runner so the flags the gate depends on are assertable without exec'ing go.
func repoContractTestArgs(packages, tags []string) []string {
	args := []string{"test", "-json", "-count=1", "-timeout", repoContractTestTimeout}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	return append(args, packages...)
}

// packagesOf returns each failure's package once, sorted, so a package with
// multiple named failures is run (or re-run) only once.
func packagesOf(failures []packFailure) []string {
	seen := map[string]bool{}
	var pkgs []string
	for _, f := range failures {
		if !seen[f.Package] {
			seen[f.Package] = true
			pkgs = append(pkgs, f.Package)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

func runRepoContractPackagesWithTags(ctx context.Context, moduleDir string, out packLog, packages, tags []string) packOutcome {
	return runGoTestJSON(ctx, moduleDir, out, repoContractTestArgs(packages, tags))
}

// goTestJSONFn is the seam for one `go test -json` run (package var, mirrors repoContractTestFn).
var goTestJSONFn = runGoTestJSON

// runRepoContractPackagesAlone re-runs each package with a named red by itself — the whole package, so same-package
// state stays in play and only the pack's concurrent load is removed. A package green by itself contributes
// nothing; one still red contributes the tests still red; one whose re-run named nothing keeps its first-run reds,
// since nothing was proven about them. The result is green only when every package is.
func runRepoContractPackagesAlone(ctx context.Context, moduleDir string, out packLog, failures []packFailure) packOutcome {
	var merged packOutcome
	var errs []string
	for _, pkg := range packagesOf(failures) {
		args := repoContractTestArgs([]string{pkg}, nil)
		fmt.Fprintf(out, "[ship] repo-contract alone re-run: go %s\n", strings.Join(args, " "))
		o := goTestJSONFn(ctx, moduleDir, out, args)
		switch {
		case o.green():
			fmt.Fprintf(out, "[ship] repo-contract alone re-run: %s green by itself\n", pkg)
		case o.realRed():
			fmt.Fprintf(out, "[ship] repo-contract alone re-run: %s still red by itself: %s\n", pkg, strings.Join(o.failedNames(), ", "))
			merged.failures = append(merged.failures, o.failures...)
		default:
			fmt.Fprintf(out, "[ship] repo-contract alone re-run: %s named nothing (%v); its first-run reds stand\n", pkg, o.err)
			merged.failures = append(merged.failures, failuresOf(pkg, failures)...)
		}
		if o.err != nil {
			errs = append(errs, pkg+": "+o.err.Error())
		}
	}
	if len(errs) != 0 {
		merged.err = errors.New(strings.Join(errs, "; "))
	}
	return merged
}

func failuresOf(pkg string, failures []packFailure) []packFailure {
	var out []packFailure
	for _, f := range failures {
		if f.Package == pkg {
			out = append(out, f)
		}
	}
	return out
}

func runGoTestJSON(ctx context.Context, moduleDir string, log packLog, args []string) packOutcome {
	out := &lockedWriter{w: log.raw}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ()) // the lane's IPC state must not reach env-sensitive tests
	cancelKillsTheProcessGroup(cmd)
	cmd.WaitDelay = packWaitDelay
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return packOutcome{err: fmt.Errorf("go test stdout pipe: %w", err)}
	}
	cmd.Stderr = out // toolchain-level chatter is not a JSON event stream
	if err := cmd.Start(); err != nil {
		return packOutcome{err: fmt.Errorf("go test start: %w", err)}
	}
	// Drain to completion BEFORE Wait: an undrained pipe deadlocks the child.
	failed, failureLog := classifyPackEvents(stdout, out)
	return packOutcome{failures: failed, failureLog: failureLog, err: cmd.Wait()}
}

// classifyPackEvents streams a `go test -json` event feed, teeing the
// human-readable Output text to tee (that is what lands in the scan log) and
// collecting the names of genuinely failing tests and their own output.
//
// The classification is deliberately conservative in ONE direction: a
// package-level fail with no test name is recorded only when the output marks
// a build/setup failure. A compile break is a real contract RED; an
// unexplained nonzero exit is NOT, and must fall through to the retry path
// rather than block an audit-green ship.
//
// Lines that are not JSON objects are teed verbatim and skipped rather than
// aborting the scan — a stray non-event line must not blind the classifier to
// the real failures after it.
func classifyPackEvents(r io.Reader, tee io.Writer) ([]packFailure, string) {
	var failed []packFailure
	failureLog := newPackFailureLog()
	// A single compile break surfaces TWICE — once as a `build-fail` action
	// (which carries no package on go1.26) and once as the `FAIL pkg [build
	// failed]` output line — so build failures are collected per package and
	// appended after the loop rather than inline. Reporting one broken package
	// as two failures would make the ship error read as a wider RED than it is.
	buildFailPkgs := map[string]bool{}
	unattributedBuildFail := false
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // guard suites emit long lines
	for sc.Scan() {
		line := sc.Bytes()
		var ev packEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			writeTee(tee, string(line)+"\n")
			continue
		}
		writeTee(tee, ev.Output)
		failureLog.note(ev)
		switch {
		case ev.Action == "fail" && ev.Test != "":
			failed = append(failed, packFailure{Package: ev.Package, Test: ev.Test})
		case ev.Action == "build-fail",
			strings.Contains(ev.Output, "[build failed]"),
			strings.Contains(ev.Output, "[setup failed]"):
			if ev.Package == "" {
				unattributedBuildFail = true
				continue
			}
			buildFailPkgs[ev.Package] = true
		}
	}
	if err := sc.Err(); err != nil {
		writeTee(tee, fmt.Sprintf("[ship] repo-contract gate: event stream read error: %v\n", err))
	}
	pkgs := make([]string, 0, len(buildFailPkgs))
	for pkg := range buildFailPkgs {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs) // deterministic ship-error message across runs
	for _, pkg := range pkgs {
		failed = append(failed, packFailure{Package: pkg + " [build failed]"})
	}
	// A build failure the toolchain never attributed to a package is still a
	// real RED — it must never fall through to the infra/retry path — so it is
	// recorded when no attributed one covers it.
	if unattributedBuildFail && len(pkgs) == 0 {
		failed = append(failed, packFailure{Package: "[build failed] (package unattributed)"})
	}
	return failed, failureLog.text.String()
}

type packEvent struct {
	Action      string `json:"Action"`
	Package     string `json:"Package"`
	Test        string `json:"Test"`
	Output      string `json:"Output"`
	ImportPath  string `json:"ImportPath"`
	FailedBuild string `json:"FailedBuild"`
}

type packFailureLog struct {
	pending map[string]*strings.Builder
	named   map[string]bool
	text    strings.Builder
}

func newPackFailureLog() *packFailureLog {
	return &packFailureLog{pending: map[string]*strings.Builder{}, named: map[string]bool{}}
}

func (l *packFailureLog) note(ev packEvent) {
	key := ev.Package + "\x00" + ev.Test
	switch {
	case ev.Action == "build-output":
		l.hold(ev.ImportPath, ev.Output)
	case ev.Action == "output":
		l.hold(key, ev.Output)
	case ev.Action == "fail" && ev.Test != "":
		l.named[ev.Package] = true
		l.flush(key)
	case ev.Action == "fail":
		l.flush(ev.FailedBuild)
		if l.named[ev.Package] {
			delete(l.pending, key)
		}
		l.flush(key)
	case ev.Action == "pass" || ev.Action == "skip":
		delete(l.pending, key)
	}
}

func (l *packFailureLog) hold(key, output string) {
	b := l.pending[key]
	if b == nil {
		b = &strings.Builder{}
		l.pending[key] = b
	}
	b.WriteString(output)
}

func (l *packFailureLog) flush(key string) {
	if b := l.pending[key]; b != nil {
		l.text.WriteString(b.String())
		delete(l.pending, key)
	}
}

// lockedWriter serializes the two writers the pack runner points at ONE
// io.Writer: the child's stderr (copied by exec's own goroutine) and the tee of
// the JSON event stream (classifyPackEvents, on the caller's goroutine).
// Without it a bytes.Buffer out is a data race — and its ReadFrom, which
// io.Copy picks for the stderr goroutine, re-slices to the length it captured
// before its blocking read, dropping every tee write made meanwhile — while an
// *os.File out merely interleaves by luck. It deliberately implements Write
// only (no io.ReaderFrom), so exec's io.Copy takes the lock per chunk.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func writeTee(tee io.Writer, s string) {
	if tee == nil || s == "" {
		return
	}
	_, _ = io.WriteString(tee, s)
}

// runRepoContractGate is the test-facing projection of runRepoContractGateAt:
// the given root is the tree under test and its changes are measured against
// HEAD. No production caller today — runNative resolves both through
// repoContractGateRoot.
func runRepoContractGate(ctx context.Context, gate, root, workspace string, stderr io.Writer) error {
	return runRepoContractGateAt(ctx, gate, root, "HEAD", workspace, stderr, nil)
}

// runRepoContractGateAt executes the three gate layers per the resolved dial,
// in root — the tree the ship will land: the lane worktree for a cycle ship
// (repoContractGateRoot), the project root otherwise — against baseRef, the
// base the tree's changes are measured from. Returns nil when the dial is
// off/empty-off, every layer is green, or an unclassifiable first failure
// cleared on the single retry. A genuine RED returns CodeRepoContractGate
// naming the failing tests; a twice-ambiguous failure (a pack run, or the
// change discovery itself) returns the distinct CodeRepoContractInfra.
//
// workspace is the run dir (`req.Workspace`); the scanner output is teed to
// <workspace>/ship-repocontract-scan.log. Empty workspace degrades to
// stderr-only diagnostics — a missing run dir must never block a ship.
func runRepoContractGateAt(ctx context.Context, gate, root, baseRef, workspace string, stderr io.Writer, cleared func([]string)) error {
	if !repocontract.GateOn(gate) {
		return nil
	}
	moduleDir := repocontract.ModuleDir(root)
	out := packLog{notes: stderr, raw: stderr}
	if scan := openScanLog(workspace, stderr); scan != nil {
		// Close error deliberately dropped: the scan log is best-effort
		// forensics; a close failure must never turn a green pack red.
		defer func() { _ = scan.Close() }()
		defer fmt.Fprintf(stderr, "[ship] repo-contract gate: full output: %s\n", scan.Name())
		out = packLog{notes: io.MultiWriter(stderr, scan), raw: scan}
	}
	if err := runFixedPack(ctx, out, gate, root, baseRef, workspace); err != nil {
		return err
	}

	files, untagged, err := runAddedTestBackstop(ctx, out, root, baseRef, moduleDir, workspace)
	if err != nil {
		return err
	}
	return runImporterBackstop(ctx, out, root, moduleDir, workspace, files, untagged, cleared)
}

func runFixedPack(ctx context.Context, out packLog, gate, root, baseRef, workspace string) error {
	runs, note := repocontract.PackRuns(gate, root)
	moduleDir := repocontract.ModuleDir(root)
	if !runs {
		fmt.Fprintf(out, "[ship] repo-contract scanner pack skipped (module %s, changes vs %s): %s\n", moduleDir, baseRef, note)
		return nil
	}
	if note != "" {
		fmt.Fprintf(out, "[ship] repo-contract gate: %s\n", note)
	}
	// Header first, so the artifact is non-empty and self-identifying even on
	// a green run — the green baseline is what disproves a false RED.
	fmt.Fprintf(out, "[ship] repo-contract scanner pack: go test -json -count=1 -timeout %s %s (module %s, changes vs %s)\n",
		repoContractTestTimeout,
		strings.Join(repoContractPackages, " "), moduleDir, baseRef)
	fmt.Fprintf(out, "[ship] repo-contract tree-reading tests by name: go %s\n", strings.Join(repoContractSelectionArgs(repoContractSelections), " "))
	return runClassifiedPack(ctx, out, workspace, "scanner pack", func() packOutcome {
		return repoContractTestFn(ctx, moduleDir, out)
	})
}

// runAddedTestBackstop is the second gate layer: it derives the gate's seed
// (the tree's changes vs baseRef) and runs every ADDED test package under the
// build tags its files declare. Returns the seed and the untagged groups'
// patterns so the importer backstop (the third layer) neither re-derives the
// seed nor re-runs those packages in the same build context.
func runAddedTestBackstop(ctx context.Context, out packLog, root, baseRef, moduleDir, workspace string) (files []changedpkgs.ChangedFile, untagged []string, err error) {
	files, err = changedFilesTwice(out, root, baseRef)
	if err != nil {
		return nil, nil, err
	}
	groups, excluded, inspectErr := addedtests.Groups(root, files)
	if inspectErr != nil {
		return nil, nil, shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
			fmt.Sprintf("repo-contract added-test backstop: could not inspect an added test's build constraints (%v) — INFRA fault, not a contract violation; safe to re-dispatch", inspectErr))
	}
	for _, path := range excluded {
		fmt.Fprintf(out, "[ship] repo-contract gate: EXCLUDED %s (requires_tmux or another build constraint unavailable on this host; backstop required)\n", path)
	}
	for _, group := range groups {
		fmt.Fprintf(out, "[ship] repo-contract added-test backstop: go test -json -count=1 -timeout %s", repoContractTestTimeout)
		if len(group.Tags) > 0 {
			fmt.Fprintf(out, " -tags %s", strings.Join(group.Tags, ","))
		}
		fmt.Fprintf(out, " %s\n", strings.Join(group.Packages, " "))
		if err := runClassifiedPack(ctx, out, workspace, "added-test backstop", func() packOutcome {
			return runRepoContractPackagesWithTags(ctx, moduleDir, out, group.Packages, group.Tags)
		}); err != nil {
			return nil, nil, err
		}
		if len(group.Tags) == 0 {
			for _, pkg := range group.Packages {
				untagged = append(untagged, pkg+"/...")
			}
		}
	}
	return files, untagged, nil
}

func runClassifiedPack(ctx context.Context, out io.Writer, workspace, name string, run func() packOutcome) error {
	return runClassifiedPackRetrying(ctx, out, workspace, name, true, run, nil)
}

// aloneRerun is the importer backstop's second look at a named red: run
// re-runs the red packages by themselves, cleared hears the names a green
// re-run cleared (nil when nobody listens).
type aloneRerun struct {
	run     func([]packFailure) packOutcome
	cleared func(names []string)
}

// runClassifiedPackRetrying is runClassifiedPack with the ambiguous-exit
// retry as a decision: a pack too large to run twice inside one ship classes
// an ambiguous exit infra straight away (re-dispatchable) instead of paying
// for the second run.
func runClassifiedPackRetrying(ctx context.Context, out io.Writer, workspace, name string, retry bool, run func() packOutcome, alone *aloneRerun) error {
	first := run()
	switch {
	case first.green():
		return nil
	case first.realRed():
		verdict, cleared := clearedAlone(out, name, first, alone)
		if cleared {
			return nil
		}
		return contractRed(name, verdict)
	}
	if !retry {
		return shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
			fmt.Sprintf("repo-contract %s exited nonzero with no test-level failure (%v) — not retried: the pack is too large to run twice inside one ship; INFRA fault, not a contract violation; safe to re-dispatch. Scanner output: %s",
				name, first.err, scanLogHint(workspace)))
	}
	fmt.Fprintf(out, "[ship] repo-contract %s exited nonzero with NO test-level failure (%v) — retrying once before classing it infra (cycle-1402/1403/1405 false-RED class)\n", name, first.err)
	second := run()
	switch {
	case second.green():
		fmt.Fprintf(out, "[ship] repo-contract gate: retry GREEN — first failure was infra noise, ship proceeds\n")
		return nil
	case second.realRed():
		return contractRed(name, second)
	}
	return shiperr.NewShipError(shiperr.CodeRepoContractInfra, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
		fmt.Sprintf("repo-contract %s exited nonzero TWICE with no test-level failure (attempt 1: %v; attempt 2: %v) — INFRA fault, not a contract violation; safe to re-dispatch. Scanner output: %s",
			name, first.err, second.err, scanLogHint(workspace)))
}

// aloneRerunMaxFailures caps the named reds the alone re-run is still worth: more named reds under the importer
// backstop are a broken contract, not a timing window.
const aloneRerunMaxFailures = 3

// CodeBackstopFlake is the ship.warning of a named red the importer backstop cleared by re-running its package
// by itself; its recurrence across cycles is what makes a flake a hygiene item.
const CodeBackstopFlake signalcenter.Code = "SHIP_BACKSTOP_FLAKE"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeBackstopFlake, "the importer backstop named red tests that were green when their packages re-ran by themselves, without the pack's concurrent load; the ship proceeded on that evidence — recurrence is a hygiene item, not proof of a timing window")
}

// backstopFlakeSignal is what the gate tells the Signal Center when a re-run clears named reds; a nil Center hears
// nothing.
func backstopFlakeSignal(signals *signalcenter.Center, cycle int) func(names []string) {
	return func(names []string) {
		if signals == nil {
			return
		}
		signals.Emit(signalcenter.Event{
			Cycle: cycle, Phase: "ship",
			Module: signalcenter.ModuleShip, Origin: "Phase.runNative", Kind: signalcenter.KindShipWarning,
			Severity: signalcenter.SeverityWarn, Code: CodeBackstopFlake,
			Reason: "importer backstop: green when their packages re-ran by themselves (evidence, not proof): " + strings.Join(names, ", "),
		})
	}
}

// clearedAlone re-runs the named reds by themselves when the pack is not the ship's own change (alone is set),
// every red is a named test and there are few enough to be a flake. A green re-run is flake evidence, logged
// loudly, and the pack is not the lane's RED; a red one is returned as the verdict; a re-run that names nothing
// leaves the first verdict standing.
func clearedAlone(out io.Writer, name string, first packOutcome, alone *aloneRerun) (packOutcome, bool) {
	if alone == nil || !first.allNamedTests() || len(first.failures) > aloneRerunMaxFailures {
		return first, false
	}
	names := first.failedNames()
	fmt.Fprintf(out, "[ship] repo-contract %s: %d named red(s) — re-running each red package by itself, without the pack's concurrent load, before any is the lane's RED: %s\n", name, len(first.failures), strings.Join(names, ", "))
	second := alone.run(first.failures)
	switch {
	case second.green():
		fmt.Fprintf(out, "[ship] WARN %s repo-contract %s: %s green when their packages ran by themselves — one re-run without the pack's load is flake evidence, not proof; the ship proceeds\n", CodeBackstopFlake, name, strings.Join(names, ", "))
		if alone.cleared != nil {
			alone.cleared(names)
		}
		return second, true
	case second.realRed():
		return second, false
	}
	return first, false
}

// contractRed builds the genuine-violation ship error, naming the parsed
// failing tests so ship-error.json carries them directly instead of a bare
// "exit status 1".
func contractRed(packName string, o packOutcome) error {
	detail := packName
	switch packName {
	case "scanner pack":
		detail = "fixed scanner pack (" + strings.Join(repoContractSuiteNames(), ", ") + ")"
	case "importer backstop":
		detail = "importer backstop (the packages that import what this ship changes)"
	}
	return shiperr.NewShipError(shiperr.CodeRepoContractGate, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
		fmt.Sprintf("repo-contract %s RED in the lane worktree (%v) — failing: %s — pushing would red main; land the green fix or use an explicit t.Skip for an intentionally red-first reproducer",
			detail, o.err, strings.Join(o.failedNames(), ", ")))
}

// openScanLog opens the run-dir scan log, truncating any prior attempt's file.
// Returns nil (never an error) when there is no workspace or the file cannot
// be opened: the log is diagnostics, and losing diagnostics must never fail a
// ship that would otherwise succeed.
func openScanLog(workspace string, stderr io.Writer) *os.File {
	if workspace == "" {
		return nil
	}
	path := filepath.Join(workspace, scanLogName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "[ship] repo-contract gate: scan log %s unavailable (%v) — continuing with stderr-only diagnostics\n", path, err)
		return nil
	}
	return f
}

func scanLogHint(workspace string) string {
	if workspace == "" {
		return "stderr only (no run workspace)"
	}
	return filepath.Join(workspace, scanLogName)
}
