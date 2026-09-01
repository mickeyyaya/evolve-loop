package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// ciparity.go — the audit phase's "CI-parity" deterministic gates. Each runs a
// whole-repo CI command (the EXACT one .github/workflows/go.yml runs) against
// THIS cycle's worktree, so a cycle can never ship green-locally / red-in-CI —
// the recurring "per-cycle proof ≠ repo-wide CI gate" disease that broke main
// via import cycles (go vet ./...), unregistered/over-ceiling env flags (-tags
// acs acs-durable), and unnamed exports (apicover -enforce).
//
// These are wired ONLY in NewDefaultWithStageCompact (production); New(Config{})
// leaves them nil so the audit package's own `go test` never recursively forks
// the go toolchain. They run in the phase-runner process (not the sandboxed
// auditor LLM), so the subprocess is unrestricted.
//
// Contract (matches gofmtCheckDefault): returns ([]offenders, nil) → FAIL when
// the CI command reports failures; (nil, err) → WARN (fail-open) when the gate
// itself cannot run (no toolchain / no module); (nil, nil) → clean.

const (
	goVetTimeout      = 4 * time.Minute
	acsDurableTimeout = 8 * time.Minute

	// integrationTierParallelism bounds the local integration-tier gate's -p
	// (concurrent package test binaries) and -parallel (in-package t.Parallel
	// tests). CI runs unbounded on an isolated box; the per-cycle gate shares a
	// contended machine with concurrent fleet lanes, where an unbounded `go test
	// -race ./...` spawns enough git subprocesses to race on pipe FDs (EBADF,
	// Path:"|0" — the flake the ship pkg's captureWithEBADFRetry band-aids) and to
	// spike memory (the race detector is 5-10x) until clean isolated tests fail at
	// 0.00s on mkdir. Bounding concurrency shrinks that footprint, including on the
	// whole-suite fallback. (Raising RLIMIT_NOFILE would NOT help: the Go 1.19+
	// runtime already lifts the soft limit to the hard max at startup, and EBADF
	// here is a concurrent-spawn pipe race, not a soft-limit exhaustion.)
	integrationTierParallelism = 4
)

// integrationTierParallelismArg is the decimal -p/-parallel value for the gate.
var integrationTierParallelismArg = strconv.Itoa(integrationTierParallelism)

// apicoverTimeout bounds the WHOLE apicover gate — the forked toolchain
// pre-steps AND the in-process apicover.Run measurement (which threads this
// ctx to its per-file AST walks; apicover-inprocess-ctx-timeout). A var, not a
// const, for the same reason as runCmd below: tests shrink it to force the
// ctx-interruption path without an 8-minute wait.
var apicoverTimeout = 8 * time.Minute

// integrationTierTimeout bounds EACH integration-tier attempt (first run and
// serialized retake separately). A var, mirroring apicoverTimeout: tests
// shrink it to force the ctx-interruption path without a 15-minute wait.
var integrationTierTimeout = 15 * time.Minute

// runCmd is the subprocess runner the CI-parity gates use. It is a package var
// so tests can inject a fake runner and exercise the exit-code mapping + the
// apicover pipeline without forking the real go toolchain.
var runCmd sysexec.RunFunc = sysexec.DefaultRunner

// moduleDirForReq resolves the cycle's go/ module dir (where the builder's code
// lives), preferring the worktree. Empty → no-op signal ("").
func moduleDirForReq(req core.PhaseRequest) string {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	if root == "" {
		return ""
	}
	dir := codequality.ModuleDir(root)
	// Require a real go module (go.mod present). ModuleDir falls back to `root`
	// itself when there is no go/ subdir, so an IsDir check alone would run the
	// gate in a non-module directory — go vet then fails "go.mod not found",
	// a false offender. A synthetic/incomplete test worktree has no go.mod.
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return ""
	}
	return dir
}

// runCIGate runs one CI command in the cycle's go/ dir and maps the result to
// the hook contract via the EXIT CODE (see the Capture note in the body): an
// exec-start failure (binary not found, context cancelled) → error → fail-open
// WARN; ANY non-zero exit → offenders → FAIL (a synthesized line covers the
// rare no-output case); exit 0 → clean.
func runCIGate(req core.PhaseRequest, label string, timeout time.Duration, name string, args ...string) ([]string, error) {
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil // no go module in the worktree → nothing to check
	}
	run := runCmd // capture once at entry (consistent with apicoverEnforceChangedDefault)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Capture (NOT CombinedOutput): DefaultRunner maps a non-zero process EXIT to
	// (code, nil), reserving err for unrecoverable start failures. So only the
	// exit code distinguishes "the tool ran and found problems" (code != 0 →
	// FAIL) from "the gate could not run" (err != nil → fail-open WARN). Capture
	// returns stdout AND stderr — go vet writes its diagnostics to stderr.
	out, errOut, code, err := sysexec.Capture(ctx, run, dir, name, args...)
	if err != nil {
		return nil, fmt.Errorf("%s gate could not run: %w", label, err) // fail-open → WARN
	}
	if code == 0 {
		return nil, nil // clean
	}
	combined := strings.TrimSpace(out + "\n" + errOut)
	if combined == "" {
		combined = fmt.Sprintf("%s exited %d (no output)", name, code)
	}
	return offenderLines(combined), nil // ran + non-zero exit → FAIL
}

// goCompilerDiagRe matches a Go compiler/vet diagnostic line ("file.go:12:34: …"
// or "file.go:12: …") — the line shape that names a build/vet offender.
var goCompilerDiagRe = regexp.MustCompile(`^\S+\.go:\d+(:\d+)?:`)

// offenderLines extracts the lines that IDENTIFY a failure from a failing
// command's output, bounded so a runaway log cannot bloat the verdict. Matching
// is LINE-ANCHORED on real failure markers — the old substring heuristics
// ("error"/"FAIL" anywhere in the line) kept PASSING tests' verbose chatter
// (in-test orchestrator WARN lines, a git usage dump) while the last-12 cap
// pushed the real `--- FAIL` lines out, so cycles 930/931/932 recorded verdicts
// citing 12 lines of noise with the true offender unknowable.
// offenderMarkerLine is the ONE home of "this line is a real failure marker"
// — shared by offenderLines (which additionally falls back to the last lines
// when nothing matches) and hasOffenderMarker (which must NOT inherit that
// fallback: the deadline-kill path degrades to WARN precisely when no marker
// exists, and the fallback would make every non-empty truncation look judged).
func offenderMarkerLine(ln string) bool {
	return strings.HasPrefix(ln, "--- FAIL") || // test failure header
		strings.HasPrefix(ln, "FAIL") || // go test package summary ("FAIL\tpkg…")
		strings.HasPrefix(ln, "panic:") || // runtime panic
		strings.HasPrefix(ln, "# ") || // build-failure package header
		strings.Contains(ln, "import cycle") ||
		strings.Contains(ln, "UNCOVERED") || // apicover offender lines
		strings.Contains(ln, "measurement error") || // apicover's synthesized infra line
		goCompilerDiagRe.MatchString(ln) // compiler/vet diagnostics
}

// hasOffenderMarker reports whether any line of out carries a real failure
// marker — the fallback-free projection of offenderMarkerLine.
func hasOffenderMarker(out string) bool {
	for _, ln := range strings.Split(out, "\n") {
		if offenderMarkerLine(strings.TrimSpace(ln)) {
			return true
		}
	}
	return false
}

func offenderLines(out string) []string {
	all := strings.Split(out, "\n")
	var keep []string
	for _, ln := range all {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if offenderMarkerLine(ln) {
			keep = append(keep, ln)
		}
	}
	if len(keep) == 0 { // no recognizable marker — fall back to the last few lines
		start := len(all) - 6
		if start < 0 {
			start = 0
		}
		for _, ln := range all[start:] {
			if ln = strings.TrimSpace(ln); ln != "" {
				keep = append(keep, ln)
			}
		}
	}
	if len(keep) > 12 {
		keep = keep[len(keep)-12:]
	}
	return keep
}

// errChangeSetUnderivable is the WARN-carrying error every whole-repo gate
// returns when it cannot determine its own input. applyCIGate maps a non-nil
// error to a WARN diagnostic (verdict unchanged), which is the deliberate
// severity: the concrete trigger is a transient concurrent-fleet
// `.git/index.lock` race, and a hard FAIL there would discard a shippable cycle.
// Loud-but-soft — never the silent no-op it replaces.
var errChangeSetUnderivable = errors.New(
	"changed-package set is underivable this cycle (git diff failed — e.g. a concurrent-fleet .git/index.lock race), " +
		"so this whole-repo gate cannot tell an untouched cycle from an unreadable one: gate skipped, CI backstops it")

// changedScopeForGate is the SINGLE owner of the touched∧derivable decision the
// three whole-repo gates (go vet, acs-durable, integration-tier) consult — no
// gate re-derives the change-set independently. It returns the change-set too, so
// the integration tier scopes from THIS result instead of calling
// changedPackagesForAudit a second time.
//
// Three outcomes:
//
//	(pkgs, true,  nil) — a real cycle build touching >=1 Go package: run, scoped to pkgs.
//	(nil,  false, err) — UNDERIVABLE: git failed, so "touched nothing" is unknowable.
//	                     Fail-open LOUD (WARN), never silently.
//	(nil,  false, nil) — derivable and genuinely empty (docs-only cycle, or no Go
//	                     module at all): nothing to check, stay silent.
//
// It replaces cycleTouchedGo, which discarded FromGitChecked's derivable bool and
// so collapsed the underivable case into the silent "touched nothing" one — the
// cycle-581 D1/D2 fail-open class (apicover was hardened against it at its own
// call site; this is the sibling frame the three shared gates went through).
// The no-module guard is checked FIRST: a worktree with no go.mod has nothing to
// check whatever git says, so a synthetic fixture gains no spurious WARN.
func changedScopeForGate(req core.PhaseRequest) ([]string, bool, error) {
	if moduleDirForReq(req) == "" {
		return nil, false, nil // no go module in the worktree → nothing to check
	}
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	pkgs, derivable := changedPackagesForAudit(root, req.Cycle)
	if !derivable {
		return nil, false, errChangeSetUnderivable
	}
	return pkgs, len(pkgs) > 0, nil
}

// goVetCheckDefault runs `go vet ./...` (CI go.yml "vet + fmt" step / `make
// lint`) over the whole worktree module — catches import cycles and other
// vet-level defects a scoped build misses. No-op unless the cycle built Go; WARN
// (not a silent skip) when the change-set is underivable.
func goVetCheckDefault(req core.PhaseRequest) ([]string, error) {
	if _, run, err := changedScopeForGate(req); err != nil || !run {
		return nil, err
	}
	return runCIGate(req, "go vet ./...", goVetTimeout, "go", "vet", "./...")
}

// acsDurableCheckDefault runs the durable ACS regression suite with -tags acs
// (CI ci.yml acs-durable gate / `make test-acs-durable`) — catches flagregistry
// / flag-ceiling / skills-drift regressions invisible without the acs build tag.
// No-op unless the cycle built Go; WARN when the change-set is underivable.
func acsDurableCheckDefault(req core.PhaseRequest) ([]string, error) {
	if _, run, err := changedScopeForGate(req); err != nil || !run {
		return nil, err
	}
	return runCIGate(req, "acs-durable (-tags acs)", acsDurableTimeout,
		"go", "test", "-count=1", "-tags", "acs", "./acs/regression/...")
}

// integrationTierCheckDefault runs the `-tags integration` test tier (go.yml's
// "test … incl. integration tier" step: `go test -tags integration $(go list
// ./... | grep -v /acs/)`) against the cycle worktree. It closes the parity
// hole one tier above go vet: TestFleetSoak went CI-red under a green per-cycle
// audit because ciparity never built the integration tier. Faithful to CI on the
// tier and flags (-race IS included — a genuine data race in a touched package
// must fail the gate; only -cover is dropped, a CI-only concern per ADR-0069),
// it runs the tier over the cycle's TOUCHED packages (see integrationTierScope),
// not the whole module. No-op unless the cycle built Go; any non-zero exit →
// offenders → FAIL.
func integrationTierCheckDefault(req core.PhaseRequest) ([]string, error) {
	// The TIER no longer derives twice: changed is the change-set
	// changedScopeForGate already resolved for this gate's touched∧derivable
	// decision, so the second changedPackagesForAudit call is gone (4 derivations
	// per cycle -> 3). Each of the three whole-repo gates still resolves its own
	// scope independently, so an underivable cycle appends three identical WARN
	// diagnostics — accurate, if repetitive (review MEDIUM: the earlier comment
	// claimed ONE shared derivation, which overstated the change). Hoisting
	// resolution into Classify and passing it down is the follow-up.
	changed, shouldRun, scopeErr := changedScopeForGate(req)
	if scopeErr != nil || !shouldRun {
		return nil, scopeErr
	}
	dir := moduleDirForReq(req)
	ctx, cancel := context.WithTimeout(context.Background(), integrationTierTimeout)
	defer cancel()
	run := runCmd
	pkgs, err := integrationTierScope(ctx, run, dir, changed)
	if err != nil {
		return nil, err // fail-open → WARN
	}
	if len(pkgs) == 0 {
		return nil, nil // cycle touched only acs/ (or nothing testable) → gate skips
	}
	// Bound execution concurrency (see integrationTierParallelism) so the forked
	// `go test -race` cannot exhaust pipe FDs / memory under concurrent fleet
	// lanes. This changes -race goroutine interleavings vs CI's unbounded run, so
	// it is a fail-open aid — a local pass that misses a race does not block, and
	// CI (isolated + unbounded) still catches it — not strict outcome-parity.
	args := append([]string{"test", "-race", "-count=1",
		"-p", integrationTierParallelismArg,
		"-parallel", integrationTierParallelismArg,
		"-tags", "integration"}, pkgs...)
	// CI-parity env scrub: CI runs the tier with a CLEAN environment; inheriting
	// the lane's os.Environ() (sysexec nil-env default) leaked EVOLVE_*/session
	// vars into env-sensitive integration tests and false-REDded them
	// deterministically (cycles 950/955: identical 0.00s failures in two
	// different worktrees, all green in isolation). Every attempt runs scrubbed.
	scrubbed := scrubbedRun(run)
	out, errOut, code, cerr := sysexec.Capture(ctx, scrubbed, dir, "go", args...)
	if cerr != nil {
		return nil, fmt.Errorf("integration-tier gate could not run: %w", cerr) // fail-open → WARN
	}
	if code == 0 {
		return nil, nil // clean
	}
	// Red first attempt. Under a live fleet the -race tier also starves for
	// CPU/IO (cycle-943: one package took 469s then failed; green in isolation),
	// so a single red is not yet evidence: RETAKE ONCE under a cross-lane
	// exclusive lock (isolation on demand — the root cause is contention, and
	// serialization removes it). Both attempts persist to integration-tier.log
	// (state.json truncates; the artifact is the one-grep diagnosis).
	// DELIBERATE trade-offs: worst-case gate wall-clock doubles (attempt 1 +
	// a fresh integrationTierTimeout retake — red paths only, and a false FAIL
	// discarding a shippable cycle costs far more). Retake trouble splits by
	// what it can testify to (2026-09-01): an EXEC failure falls back to
	// attempt-1 offenders — possibly contended data, but a real red must never
	// be laundered by retake infra trouble; a DEADLINE kill first reports any
	// offenders the truncated output already flushed (go test emits each
	// completed package's verdict before the SIGKILL — evidence outranks the
	// budget), and only a marker-free truncation degrades to the fail-open
	// WARN, because a bare exit code from a killed run is not a test verdict.
	// logPath is set on the FIRST successful log write and never cleared: a
	// later append failure must not drop the pointer to a real on-disk artifact
	// that already carries attempt 1 (go-review MEDIUM).
	logPath := ""
	appendLog := func(attempt int, note, o, e string, c int) {
		if req.Workspace == "" {
			return
		}
		p := filepath.Join(req.Workspace, "integration-tier.log")
		entry := fmt.Sprintf("# attempt %d%s\n# go %s\n# exit: %d\n\n%s\n%s\n", attempt, note, strings.Join(args, " "), c, o, e)
		f, ferr := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if ferr != nil {
			return
		}
		defer func() { _ = f.Close() }()
		if _, werr := f.WriteString(entry); werr == nil {
			logPath = p
		}
	}
	appendLog(1, " (lane env, contended)", out, errOut, code)

	// offendersWithLogPointer turns a run's raw output into FAIL offenders,
	// plus a pointer line (not a failure marker) to the untruncated log —
	// slightly inflates the offender count, acceptable for discoverability.
	offendersWithLogPointer := func(o, e string) []string {
		offenders := offenderLines(strings.TrimSpace(o + "\n" + e))
		if logPath != "" {
			offenders = append(offenders, "full output: "+logPath)
		}
		return offenders
	}

	release, lockNote := acquireTierLock(req)
	retakeCtx, retakeCancel := context.WithTimeout(context.Background(), integrationTierTimeout)
	out2, errOut2, code2, cerr2 := sysexec.Capture(retakeCtx, scrubbed, dir, "go", args...)
	retakeCancel()
	release()
	if cerr2 != nil {
		// The retake itself could not run — fall back to the first attempt's
		// offenders (a real red should not be laundered by retake infra trouble).
		return offendersWithLogPointer(out, errOut), nil
	}
	appendLog(2, " (serialized retake"+lockNote+")", out2, errOut2, code2)
	if code2 == 0 {
		// Red-then-green: a contention flake, absorbed. Surface a visible WARN
		// (applyCIGate's could-not-run path) — never a false FAIL that discards
		// a shippable cycle, and never silent.
		where := "integration-tier.log unavailable"
		if logPath != "" {
			where = "both attempts: " + logPath
		}
		return nil, fmt.Errorf("integration tier was RED under fleet contention but GREEN on a serialized clean-env retake — contention flake absorbed, not a code defect (%s)", where)
	}
	if errors.Is(retakeCtx.Err(), context.DeadlineExceeded) {
		// Deadline kill. Reachable because sysexec maps a ctx kill to a
		// non-nil ExitError with err==nil (exit -1), so the cerr2 early-return
		// above does not swallow it — pinned by the orchestration deadline
		// tests. Evidence outranks the budget: report any offenders the
		// truncated output already flushed; only a marker-free truncation
		// degrades to WARN (2026-09-01 re-widened scope: quiet-host tier
		// ~127s x 3.6-7.7 contention against this budget makes the deadline a
		// live path, and it must not become a laundering channel).
		if hasOffenderMarker(out2 + "\n" + errOut2) {
			return offendersWithLogPointer(out2, errOut2), nil
		}
		where := "integration-tier.log unavailable"
		if logPath != "" {
			where = "both attempts: " + logPath
		}
		return nil, fmt.Errorf("integration tier exceeded its %s budget on the serialized retake with no test verdict in the truncated output — a deadline kill is not a judgment; degraded to WARN (%s)", integrationTierTimeout, where)
	}
	// Red-then-red inside budget: genuine. The serialized clean-env retake is
	// the truthful attempt — its offenders name the real failure.
	return offendersWithLogPointer(out2, errOut2), nil
}

// integrationTierEnvAllowlist is the minimal environment the tier subprocess
// keeps — what a clean CI shell provides. Everything else (EVOLVE_*, BRIDGE_*,
// tmux/session vars) is the lane's runtime state and must not reach
// env-sensitive integration tests.
var integrationTierEnvAllowlist = []string{
	"PATH", "HOME", "TMPDIR", "USER", "SHELL",
	"GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN", "CC",
}

func integrationTierCleanEnv() []string {
	env := make([]string, 0, len(integrationTierEnvAllowlist))
	for _, k := range integrationTierEnvAllowlist {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// scrubbedRun wraps a sysexec.RunFunc so every invocation carries the scrubbed
// allowlist env instead of whatever env the caller passes (nil would inherit
// the lane's full os.Environ()).
func scrubbedRun(run sysexec.RunFunc) sysexec.RunFunc {
	clean := integrationTierCleanEnv()
	return func(ctx context.Context, name, dir string, args, _ []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return run(ctx, name, dir, args, clean, stdin, stdout, stderr)
	}
}

// tierLockWait bounds how long a red retake waits for the cross-lane lock. An
// INDEPENDENT budget, deliberately NOT the attempt-1 ctx: under the exact
// contention the retake exists to absorb, attempt 1 may have consumed most of
// the tier deadline, and a lock wait bounded by the leftovers would degrade to
// an unserialized retake precisely when serialization matters most (go-review
// HIGH). A var so tests can shrink the wait.
var tierLockWait = 5 * time.Minute

// acquireTierLock takes a best-effort cross-lane exclusive lock so the retake
// runs serialized against other lanes' tier/test load, via the shared
// internal/adapters/flock primitive (which owns the runtime.KeepAlive raw-fd
// defense and in-process held-tracking — never re-derive raw flock here,
// go-review HIGH). Best-effort by design: a lock failure degrades to an
// unserialized retake (noted in the log), never blocks the gate. Root is
// ProjectRoot FIRST — deliberately reversed vs moduleDirForReq's Worktree-first
// order: the lock must live on the CYCLE-SHARED path so lanes contend on ONE
// file; a per-lane worktree path would defeat cross-lane serialization.
func acquireTierLock(req core.PhaseRequest) (release func(), note string) {
	root := req.ProjectRoot
	if root == "" {
		root = req.Worktree
	}
	if root == "" {
		return func() {}, ", lock unavailable: no project root"
	}
	path := filepath.Join(root, ".evolve", "locks", "integration-tier.lock")
	deadline := time.Now().Add(tierLockWait)
	for {
		rel, held, err := flock.TryLock(path)
		if err != nil {
			return func() {}, ", lock unavailable: " + err.Error()
		}
		if !held {
			return rel, ""
		}
		if time.Now().After(deadline) {
			return func() {}, ", lock wait timed out (retake unserialized)"
		}
		time.Sleep(2 * time.Second)
	}
}

// integrationTierScope returns the `go test` package patterns the integration
// tier should run for a cycle's already-derived, non-empty changed-package set
// (`changed`): the TOUCHED packages themselves (the same O(change) scoping the
// apicover-enforce gate uses), minus /acs/ (which has its own -tags acs gate).
// The one whole-module fallback is a module-root change (a `./...` pattern from
// go.mod/go.sum/root main.go) — rare, and no narrower scope exists.
//
// Scoping is load-bearing for RELIABILITY, not just speed: the tier stays
// O(change) so the -race run's contention exposure is bounded, and the
// serialized retake-on-red (below) absorbs what contention remains. The one
// env-exclusive package (see integrationTierEnvExclusive — the record is the
// authority) is skipped with its recorded backstop named in the WARN. The rare
// module-root fallback is derivable (not the contention-correlated git-failure
// case). Whole-repo integration coverage remains CI's job — the identical
// backstop apicover-enforce relies on.
func integrationTierScope(ctx context.Context, run sysexec.RunFunc, dir string, changed []string) ([]string, error) {
	scoped := make([]string, 0, len(changed))
	var envExclusive []string
	for _, p := range changed {
		if p == "./..." {
			return integrationTierWholeSuite(ctx, run, dir) // module-root change → whole module
		}
		if strings.Contains(p, "/acs/") {
			continue // acs has its own -tags acs gate (acsDurableCheckDefault)
		}
		if envExclusivePkg(p) {
			envExclusive = append(envExclusive, p)
			continue
		}
		scoped = append(scoped, p)
	}
	if len(scoped) == 0 && len(envExclusive) > 0 {
		// Everything in scope is env-exclusive: surface a visible WARN (applyCIGate's
		// could-not-run path) instead of a false FAIL; each record names its
		// own evidence and backstop.
		return nil, fmt.Errorf("touched package(s) %s are env-exclusive under a live loop. Backstop: %s (ADR-0069)", strings.Join(envExclusive, ", "), envExclusiveBackstopNote(envExclusive))
	}
	if len(envExclusive) > 0 {
		// Mixed scope: run the runnable remainder; name the skips in the lane log.
		fmt.Fprintf(os.Stderr, "[integration-tier] skipping env-exclusive package(s) under a live loop: %s. Backstop: %s\n", strings.Join(envExclusive, ", "), envExclusiveBackstopNote(envExclusive))
	}
	return scoped, nil // may be empty (cycle touched only acs/) → gate skips
}

// envExclusiveEntry is the SINGLE record for one env-exclusive package: the
// package, the evidence, and where its integration tests actually run instead.
// Every consumer — envExclusivePkg, the emitted WARN, the lane-log note, the
// whole-suite filter — projects from integrationTierEnvExclusive; no prose
// restatement elsewhere is authoritative (the 2026-09-01 architecture review
// found three stale copies inside one diff — projections, not narration).
//
// SELECTION CRITERION (the only thing keeping this list a scalpel, pinned by
// TestEnvExclusive_EntriesDeclareNoCIBackstop): a package may be listed ONLY
// when the serialized retake below cannot make red-twice trustworthy AND CI
// provides no backstop. A package whose integration tier CI covers belongs IN
// the lane tier. HISTORY: internal/core, cmd/evolve and internal/phases/ship
// were excluded 2026-07-19 (8e2afef0; contention false-REDs, cycles
// 930/931/932) and the serialized retake that properly cures that contention
// landed ONE DAY LATER (3c5ed711) — the never-revisited skip shipped
// cycle-1594's red to main for 2.5 days (20e839ee; #519; #518). Two notes for
// the next adjudicator: the July evidence over-attributed cmd/evolve
// (its fleet-soak suite is in-process fakes, no real tmux —
// cmd_fleet_soak_test.go; its ~69s is CPU), and a compile-only floor would NOT
// have caught 1594 (an assertion failure, not a compile failure) — running
// the tier is the point.
type envExclusiveEntry struct {
	pkg string // module-relative package dir, e.g. "internal/bridge"
	why string // the exclusion evidence
	// backstop states where the package's integration tests actually run;
	// rendered VERBATIM into the emitted WARN. One dishonest word here
	// recreates the #483 defect: a gate asserting coverage that cannot occur.
	backstop string
}

// envExclusiveNoCIMarker is the machine-checkable half of the selection
// criterion: every entry's backstop must carry it, and the rule test asserts
// against THIS const — one home for the phrase, so rewording the prose cannot
// silently detach the data from the contract.
const envExclusiveNoCIMarker = "NOT covered by CI"

var integrationTierEnvExclusive = []envExclusiveEntry{{
	pkg:      "internal/bridge",
	why:      "requireTmux tests boot real tmux sessions; under a live wave those boots time out (13 offenders on cycle-1543, all exit=80; the same tests 7/7 PASS in 17.2s on a quiet host)",
	backstop: "internal/bridge's requireTmux tier is " + envExclusiveNoCIMarker + " (no tmux on runners — the #483 finding); its backstop is a quiet-host run (loop-boot preflight, or `go test -tags integration` with no wave active)",
}}

// envExclusiveEntryFor resolves a package pattern ("./internal/bridge/...", a
// full import path, or a bare relative dir) to its record.
func envExclusiveEntryFor(p string) (envExclusiveEntry, bool) {
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
	p = strings.TrimSuffix(p, "/")
	for _, e := range integrationTierEnvExclusive {
		if p == e.pkg || strings.HasSuffix(p, "/"+e.pkg) {
			return e, true
		}
	}
	return envExclusiveEntry{}, false
}

// envExclusivePkg reports whether a package pattern denotes an env-exclusive
// package — a projection of the record table.
func envExclusivePkg(p string) bool {
	_, ok := envExclusiveEntryFor(p)
	return ok
}

// envExclusiveBackstopNote renders each skipped package's backstop, verbatim
// from its record, deduplicated.
func envExclusiveBackstopNote(pkgs []string) string {
	seen := map[string]bool{}
	var parts []string
	for _, p := range pkgs {
		if e, ok := envExclusiveEntryFor(p); ok && !seen[e.pkg] {
			seen[e.pkg] = true
			parts = append(parts, e.why+" — "+e.backstop)
		}
	}
	return strings.Join(parts, "; ")
}

// integrationTierWholeSuite lists every module package minus /acs/ (go.yml's
// `go list ./... | grep -v /acs/` filter) minus the env-exclusive set — each
// record in integrationTierEnvExclusive names where those tests actually run
// instead — for the module-root fallback.
func integrationTierWholeSuite(ctx context.Context, run sysexec.RunFunc, dir string) ([]string, error) {
	listOut, err := sysexec.Output(ctx, run, dir, "go", "list", "./...")
	if err != nil {
		return nil, fmt.Errorf("integration-tier gate: go list: %w", err)
	}
	var pkgs []string
	for _, p := range strings.Fields(listOut) {
		if strings.Contains(p, "/acs/") || envExclusivePkg(p) {
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// apicoverEnforceChangedDefault runs `apicover -enforce` (CI go.yml "api-coverage
// enforce" step) over the enforced packages this cycle actually touched — the
// AST-level UNCOVERED (unnamed-export) check that repeatedly broke main. Scoped
// to the touched∩enforced set (O(change)); a no-op when the cycle touched no
// enforced package. FALSE-GREEN (coverage-dependent) is left to CI, matching the
// acs/regression/apicover completeness/correctness split.
func apicoverEnforceChangedDefault(req core.PhaseRequest) ([]string, error) {
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil
	}
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	changed, derivable := changedPackagesForAudit(root, req.Cycle)
	enforceBytes, err := os.ReadFile(filepath.Join(dir, ".apicover-enforce"))
	if err != nil {
		return nil, nil // no enforce list → nothing to enforce
	}
	if !derivable {
		// Underivable changed-set on a cycle WITH an enforce list: git failed
		// (no repo, bad baseRef, fleet .git/index.lock race), so we cannot prove
		// the touched∩enforced set is empty. FAIL loud (err==nil) instead of the
		// silent (nil,nil) no-op that shipped an uncovered export (cycle-581 D1).
		return []string{"changed-package set is underivable this cycle (git diff failed) — apicover -enforce gate cannot verify coverage; treat as FAIL, do not ship"}, nil
	}
	touched := ciparity.IntersectEnforced(changed, enforceBytes)
	if len(touched) == 0 {
		return nil, nil // cycle touched no enforced package
	}

	ctx, cancel := context.WithTimeout(context.Background(), apicoverTimeout)
	defer cancel()
	run := runCmd

	// Scoped coverage profile over the touched packages (apicover reads a
	// func-coverage file), then the enforce gate IN-PROCESS over just those dirs
	// — the same pipeline as go.yml, scoped, but folded into the evolve binary
	// (one-binary S1): no runtime `go build -o bin/apicover`. The scratch cover
	// files still live under the worktree's bin/, which we ensure exists (the
	// deleted build used to create it as a side effect).
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, fmt.Errorf("apicover gate: ensure bin dir: %w", err)
	}
	covPath := filepath.Join(binDir, "ciparity-cover.txt")
	defer func() { _ = os.Remove(covPath) }() // scratch profile — don't accumulate on a persistent worktree
	// Tag-parity: build the scoped coverage args through the ciparity SSOT so
	// the gate measures the SAME (tagged) coverage number CI does — an untagged
	// run under-reports a tag-gated package by up to 43 points (R1).
	testArgs := ciparity.CoverageTestArgs(covPath, touched)
	if _, err := sysexec.Output(ctx, run, dir, "go", testArgs...); err != nil {
		return nil, fmt.Errorf("apicover gate: scoped coverage run: %w", err)
	}
	funcPath := covPath + ".func.txt"
	defer func() { _ = os.Remove(funcPath) }()
	funcOut, err := sysexec.Output(ctx, run, dir, "go", "tool", "cover", "-func="+covPath)
	if err != nil {
		return nil, fmt.Errorf("apicover gate: cover -func: %w", err)
	}
	if werr := os.WriteFile(funcPath, []byte(funcOut+"\n"), 0o644); werr != nil {
		return nil, fmt.Errorf("apicover gate: write func cover: %w", werr)
	}
	dirsOut, err := sysexec.Output(ctx, run, dir, "go", append([]string{"list", "-e", "-f", "{{.Dir}}"}, touched...)...)
	if err != nil {
		return nil, fmt.Errorf("apicover gate: go list: %w", err)
	}
	dirs := strings.Fields(dirsOut)
	if len(dirs) == 0 {
		return nil, nil
	}
	// In-process enforce gate — the folded apicover.Run, not a bin/apicover
	// subprocess. Exit-code contract: 0 clean; 1 offenders → FAIL; 2 (with a
	// non-nil error) a measurement failure → also FAIL. In-process there is NO
	// exec-start failure mode (the process always "runs"), so a measurement
	// error is a real finding about the touched code — an unparseable enforced
	// package — not the fail-open infra WARN a subprocess exit-2 warranted.
	// Folding it into the offender report keeps the FAIL the old bin/apicover
	// exit-2 produced (cf. the underivable-changed-set hard-FAIL, cycle-581 D1).
	// The gate ctx bounds the measurement itself (apicover-inprocess-ctx-timeout):
	// pre-ctx, a wedged AST walk escaped apicoverTimeout entirely.
	var report bytes.Buffer
	code, runErr := apicover.Run(ctx, apicover.Config{Enforce: true, CoverPath: funcPath, Dirs: dirs}, &report)
	if code == 0 && runErr == nil {
		return nil, nil // clean
	}
	// A ctx-deadline/cancel interruption is INFRA weather, not a finding about
	// the touched code — surface it as an error so this gate fails OPEN (WARN),
	// exactly like the sibling ctx-bounded exec steps above. Real measurement
	// errors (unparseable package) stay in the offender report → FAIL.
	if runErr != nil && (errors.Is(runErr, context.DeadlineExceeded) || errors.Is(runErr, context.Canceled)) {
		return nil, fmt.Errorf("apicover gate: measurement interrupted: %w", runErr)
	}
	detail := strings.TrimSpace(report.String())
	if runErr != nil {
		detail = strings.TrimSpace(detail + "\napicover -enforce measurement error: " + runErr.Error())
	}
	return offenderLines(detail), nil // offenders or measurement error → FAIL
}

// apicoverNewPackageGraduationDefault flags changed go/internal/<pkg> packages
// that are NEW this cycle and absent from .apicover-enforce — the blind spot
// apicoverEnforceChangedDefault's IntersectEnforced silently drops (a package
// new this cycle cannot yet be in the enforce list, so the touched∩enforced
// scoping never inspects it). This is the deterministic, fail-fast half of the
// recurring warnship_apicover_ci_gap: each ungraduated package must gain an
// .apicover-enforce entry + an apicover_named_test.go before audit can PASS.
// Mirrors apicoverEnforceChangedDefault's own resolution (worktree dir, changed
// packages, enforce list); a no-op (nil,nil) when there is no module, no enforce
// list, or nothing ungraduated. go/cmd/... changes are never flagged (out of
// apicover's scope).
func apicoverNewPackageGraduationDefault(req core.PhaseRequest) ([]string, error) {
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil
	}
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	changed, derivable := changedPackagesForAudit(root, req.Cycle)
	enforceBytes, err := os.ReadFile(filepath.Join(dir, ".apicover-enforce"))
	if err != nil {
		return nil, nil // no enforce list → nothing to graduate against
	}
	if !derivable {
		// Same fail-loud reasoning as apicoverEnforceChangedDefault: an
		// underivable changed-set means we cannot prove no new package is
		// ungraduated, so FAIL loud rather than silently no-op (cycle-581 D2).
		return []string{"changed-package set is underivable this cycle (git diff failed) — apicover graduation gate cannot verify new packages; treat as FAIL, do not ship"}, nil
	}
	ungraduated := ciparity.NewUngraduatedPackages(changed, enforceBytes)
	if len(ungraduated) == 0 {
		return nil, nil
	}
	offenders := make([]string, 0, len(ungraduated))
	for _, pkg := range ungraduated {
		// Same predicate as the build-entry seam: a test-only package has no
		// production surface for the gate to inspect — flagging it here after
		// the build seam stopped doing so would just move the vacuous FAIL one
		// phase later (ciparity.PackageDirHasProductionGoFiles rationale).
		if !ciparity.PackageDirHasProductionGoFiles(dir, pkg) {
			// Never silently skip (review F2): the deferred obligation must be
			// visible in the outcome record — enrollment re-raises when the
			// production half lands (this seam has no new-this-cycle filter,
			// so ANY later change to the package re-flags it).
			fmt.Fprintf(os.Stderr, "[audit] graduation deferred: %s has no production .go surface (test-only/absent) — enrollment obligation re-raises when production code lands\n", pkg)
			continue
		}
		offenders = append(offenders, fmt.Sprintf("%s: new package absent from go/.apicover-enforce — the repo-wide apicover unnamed-export gate never inspects it. Make EXACTLY these edits:\n%s", pkg, ciparity.GraduationPrescription([]string{pkg})))
	}
	return offenders, nil
}

// changedPackagesForAudit locates this cycle's changed-package set and reports
// whether it is derivable. It prefers the build handoff when present (same
// locator the EGPS suite uses; a handoff yielding >=1 pkg is derivable), then
// falls back to a deterministic git derivation (changedpkgs.FromGitChecked vs
// HEAD). The handoff has been extinct since ~cycle 215, so the git fallback is
// what keeps the apicover gate live. The derivable flag closes the last
// fail-open hole: previously the git fallback returned nil identically whether
// the tree was git-clean (nothing changed) or the set was underivable (git
// failed), letting an underivable cycle ship with a silent PASS (cycle-581
// D1/D2, standing memory warnship_apicover_ci_gap).
func changedPackagesForAudit(projectRoot string, cycle int) ([]string, bool) {
	if projectRoot == "" {
		return nil, false
	}
	dir := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	for _, name := range []string{"handoff-build.json", "handoff-builder.json"} {
		if pkgs := changedpkgs.ChangedPackages(filepath.Join(dir, name)); len(pkgs) > 0 {
			return pkgs, true // handoff present and non-empty → derivable
		}
	}
	return changedpkgs.FromGitChecked(projectRoot, "HEAD")
}
