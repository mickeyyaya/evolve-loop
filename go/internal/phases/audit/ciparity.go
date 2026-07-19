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
	goVetTimeout           = 4 * time.Minute
	acsDurableTimeout      = 8 * time.Minute
	integrationTierTimeout = 15 * time.Minute

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
func offenderLines(out string) []string {
	all := strings.Split(out, "\n")
	var keep []string
	for _, ln := range all {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, "--- FAIL") || // test failure header
			strings.HasPrefix(ln, "FAIL") || // go test package summary ("FAIL\tpkg…")
			strings.HasPrefix(ln, "panic:") || // runtime panic
			strings.HasPrefix(ln, "# ") || // build-failure package header
			strings.Contains(ln, "import cycle") ||
			strings.Contains(ln, "UNCOVERED") || // apicover offender lines
			strings.Contains(ln, "measurement error") || // apicover's synthesized infra line
			goCompilerDiagRe.MatchString(ln) { // compiler/vet diagnostics
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

// cycleTouchedGo reports whether this cycle has a build handoff naming >=1
// changed Go package — the signal that this worktree is a REAL cycle build (a
// synthetic test fixture or a docs-only cycle has none). The whole-repo gates
// (go vet, acs-durable) run only then, so they never fire against an incomplete
// module (e.g. a unit-test worktree with a bare go/ dir but no go.mod / repo
// structure).
func cycleTouchedGo(req core.PhaseRequest) bool {
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	pkgs, _ := changedPackagesForAudit(root, req.Cycle)
	return len(pkgs) > 0
}

// goVetCheckDefault runs `go vet ./...` (CI go.yml "vet + fmt" step / `make
// lint`) over the whole worktree module — catches import cycles and other
// vet-level defects a scoped build misses. No-op unless the cycle built Go.
func goVetCheckDefault(req core.PhaseRequest) ([]string, error) {
	if !cycleTouchedGo(req) {
		return nil, nil
	}
	return runCIGate(req, "go vet ./...", goVetTimeout, "go", "vet", "./...")
}

// acsDurableCheckDefault runs the durable ACS regression suite with -tags acs
// (CI ci.yml acs-durable gate / `make test-acs-durable`) — catches flagregistry
// / flag-ceiling / skills-drift regressions invisible without the acs build tag.
// No-op unless the cycle built Go.
func acsDurableCheckDefault(req core.PhaseRequest) ([]string, error) {
	if !cycleTouchedGo(req) {
		return nil, nil
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
	if !cycleTouchedGo(req) {
		return nil, nil
	}
	dir := moduleDirForReq(req)
	if dir == "" {
		return nil, nil // no go module in the worktree → nothing to check
	}
	root := req.Worktree
	if root == "" {
		root = req.ProjectRoot
	}
	// cycleTouchedGo guarantees a derivable, non-empty change-set here: an
	// underivable set (git diff failed) yields no packages, so cycleTouchedGo
	// already returned false above. The derivable bool is therefore always true
	// at this point and intentionally discarded. (The pre-existing gap that an
	// underivable cycle silently skips this AND the sibling whole-repo gates —
	// go vet / acs-durable, all gated on cycleTouchedGo — is tracked separately
	// as the cycleTouchedGo-derivability-propagation defect; fixing it here would
	// turn transient git-index.lock hiccups into new WARNs, out of scope.)
	changed, _ := changedPackagesForAudit(root, req.Cycle)
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
	// discarding a shippable cycle costs far more); a retake that itself dies
	// (exec failure / retake-deadline kill) falls back to attempt-1 offenders —
	// possibly contended data, but a real red must never be laundered by retake
	// infra trouble.
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
	// Red-then-red: genuine. The serialized clean-env retake is the truthful
	// attempt — its offenders name the real failure.
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
// Scoping is load-bearing for RELIABILITY, not just speed. The old unconditional
// whole-suite `go test -race -tags integration ./...` run is parallel-unsafe
// under the loop's contended LOCAL environment — two fleet lanes running it
// concurrently plus real tmux/git — so heavy env-dependent tests (TestFleetSoak
// spawns tmux fleets, TestShipFromWorktree drives real git worktrees) flaked the
// gate EVERY cycle, producing false-REDs on tests CI passes. CI runs the same
// command once, isolated, and stays green; scoping means a cycle that only
// touched, say, internal/bridge never runs the fleet/ship tests at all. The rare
// module-root fallback is derivable (not the contention-correlated git-failure
// case), so it does not reintroduce the flake. Whole-repo integration coverage
// remains CI's job — the identical backstop apicover-enforce relies on.
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
		// could-not-run path) instead of a false FAIL. CI is the backstop.
		return nil, fmt.Errorf("touched package(s) %s are env-exclusive under a live loop — their integration tests (full RunCycle orchestrators over real git, tmux fleets, real git worktrees) false-RED the tier under fleet contention while CI, isolated, stays green (cycles 930/931/932); CI's integration-tier step remains the backstop (ADR-0069)", strings.Join(envExclusive, ", "))
	}
	if len(envExclusive) > 0 {
		// Mixed scope: run the runnable remainder; name the skips in the lane log.
		fmt.Fprintf(os.Stderr, "[integration-tier] skipping env-exclusive package(s) under a live loop (CI covers them): %s\n", strings.Join(envExclusive, ", "))
	}
	return scoped, nil // may be empty (cycle touched only acs/) → gate skips
}

// integrationTierEnvExclusive names the packages whose integration-tagged tests
// demand an EXCLUSIVE local environment and therefore cannot run reliably inside
// the loop's contended runtime: internal/core (full RunCycle orchestrators over
// real git — the proven false-RED producer of cycles 930/931/932 and cycle-3 in
// a second repo: identical noise-offender fingerprint each time, green in
// isolation), cmd/evolve (TestFleetSoak spawns real tmux fleets), and
// internal/phases/ship (TestShipFromWorktree drives real git worktrees). CI runs
// the exact tier once, isolated, and stays green — per-cycle parity for these
// packages is CI's job (the same ADR-0069 rationale that scoped the tier in the
// first place).
var integrationTierEnvExclusive = []string{
	"internal/core",
	"cmd/evolve",
	"internal/phases/ship",
}

// envExclusivePkg reports whether a package pattern ("./internal/core/...", a
// full import path, or a bare relative dir) denotes an env-exclusive package.
func envExclusivePkg(p string) bool {
	p = strings.TrimSuffix(strings.TrimPrefix(p, "./"), "/...")
	p = strings.TrimSuffix(p, "/")
	for _, ex := range integrationTierEnvExclusive {
		if p == ex || strings.HasSuffix(p, "/"+ex) {
			return true
		}
	}
	return false
}

// integrationTierWholeSuite lists every module package minus /acs/ (go.yml's
// `go list ./... | grep -v /acs/` filter) minus the env-exclusive set (their
// integration tests cannot run reliably inside the loop's contended runtime —
// see integrationTierEnvExclusive; CI covers them isolated) — for the
// module-root fallback.
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
		offenders = append(offenders, fmt.Sprintf("%s: new package absent from go/.apicover-enforce — add it + an apicover_named_test.go", pkg))
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
