package ciparitygate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// tierParallelism bounds the local integration-tier gate's -p (concurrent
// package test binaries) and -parallel (in-package t.Parallel tests). CI runs
// unbounded on an isolated box; the per-cycle gate shares a contended machine
// with concurrent fleet lanes, where an unbounded `go test -race ./...` spawns
// enough git subprocesses to race on pipe FDs (EBADF, Path:"|0") and to spike
// memory (the race detector is 5-10x) until clean isolated tests fail at
// 0.00s on mkdir. Bounding concurrency shrinks that footprint, including on
// the whole-suite fallback. (Raising RLIMIT_NOFILE would NOT help: the Go
// 1.19+ runtime already lifts the soft limit to the hard max at startup, and
// EBADF here is a concurrent-spawn pipe race, not a soft-limit exhaustion.)
// This changes -race goroutine interleavings vs CI's unbounded run, so it is a
// fail-open aid — a local pass that misses a race does not block, and CI
// (isolated + unbounded) still catches it — not strict outcome-parity.
const tierParallelism = 4

// tierArgs is the ONE `go test` vector of the tier; its order is an on-disk
// contract through the integration-tier.log header. Faithful to CI on the
// tier and flags (-race IS included — a genuine data race in a touched
// package must fail the gate; only -cover is dropped, a CI-only concern per
// ADR-0069).
func tierArgs(pkgs []string) []string {
	p := strconv.Itoa(tierParallelism)
	return append([]string{"test", "-race", "-count=1", "-p", p, "-parallel", p, "-tags", "integration"}, pkgs...)
}

// attempt is one run of the tier: its output, exit code, start error, and
// whether its ctx deadline had fired by the time it returned (recorded for
// both attempts, consulted only for the retake — the original never checked
// attempt 1's ctx).
type attempt struct {
	out, errOut string
	code        int
	err         error
	deadlineHit bool
}

func runAttempt(ctx context.Context, run sysexec.RunFunc, dir string, args []string) attempt {
	out, errOut, code, err := sysexec.Capture(ctx, run, dir, "go", args...)
	return attempt{out: out, errOut: errOut, code: code, err: err, deadlineHit: errors.Is(ctx.Err(), context.DeadlineExceeded)}
}

// IntegrationTier runs the `-tags integration` test tier (go.yml's "test …
// incl. integration tier" step: `go test -tags integration $(go list ./... |
// grep -v /acs/)`) against the cycle worktree, over the cycle's TOUCHED
// packages (see tierScope), not the whole module. It closes the parity hole
// one tier above go vet: TestFleetSoak went CI-red under a green per-cycle
// audit because ciparity never built the integration tier. No-op unless the
// cycle built Go; a red first attempt is retaken once, serialized (retake).
func (g *Gates) IntegrationTier(req Request) ([]string, error) {
	changed, shouldRun, scopeErr := g.scope(gateTier, req)
	if scopeErr != nil || !shouldRun {
		return nil, scopeErr
	}
	dir := moduleDir(req.root())
	ctx, cancel := context.WithTimeout(context.Background(), g.timeouts.TierAttempt)
	defer cancel()
	pkgs, err := g.tierPackages(ctx, req, dir, changed)
	if err != nil {
		return nil, err // fail-open → WARN
	}
	if len(pkgs) == 0 {
		return nil, nil // cycle touched only acs/ (or nothing testable) → gate skips
	}
	args := tierArgs(pkgs)
	// CI-parity env scrub: CI runs the tier with a CLEAN environment; inheriting
	// the lane's os.Environ() leaked EVOLVE_*/session vars into env-sensitive
	// integration tests and false-REDded them deterministically (cycles
	// 950/955). Every attempt runs scrubbed.
	scrubbed := scrubbedRun(g.run)
	first := runAttempt(ctx, scrubbed, dir, args)
	if first.err != nil {
		wrapped := fmt.Errorf("integration-tier gate could not run: %w", first.err) // fail-open → WARN
		g.stepFailed(gateTier, req, stepTierAttempt, wrapped.Error(), first.err, "cmd", "go "+strings.Join(args, " "))
		return nil, wrapped
	}
	if first.code == 0 {
		return nil, nil // clean
	}
	return g.retake(req, dir, args, scrubbed, first)
}

// tierPackages is the scope fork: a module-root change lists the whole
// module; an all-env-exclusive scope skips with the WARN error and
// TIER_ENV_EXCLUSIVE_SKIPPED scope=all; a mixed scope runs the runnable
// remainder and records the skips as scope=mixed (the lane-log line it
// replaces, D1).
func (g *Gates) tierPackages(ctx context.Context, req Request, dir string, changed []string) ([]string, error) {
	scoped, envExclusive, wholeModule := tierScope(changed)
	if wholeModule {
		pkgs, err := g.wholeSuite(ctx, dir)
		if err != nil {
			g.stepFailed(gateTier, req, stepTierList, err.Error(), err, "cmd", "go list ./...")
		}
		return pkgs, err
	}
	if len(envExclusive) == 0 {
		return scoped, nil // may be empty (cycle touched only acs/) → gate skips
	}
	skipped, backstop := strings.Join(envExclusive, ", "), envExclusiveBackstopNote(envExclusive)
	if len(scoped) == 0 {
		// Everything in scope is env-exclusive: surface a visible WARN instead
		// of a false FAIL; each record names its own evidence and backstop.
		err := fmt.Errorf("touched package(s) %s are env-exclusive under a live loop. Backstop: %s (ADR-0069)", skipped, backstop)
		g.warn(gateTier, req, CodeTierEnvExclusiveSkipped, err.Error(), "scope", "all", "pkgs", skipped, "remainder", "0", "backstop", backstop)
		return nil, err
	}
	g.warn(gateTier, req, CodeTierEnvExclusiveSkipped, "skipping env-exclusive package(s) under a live loop: "+skipped+". Backstop: "+backstop,
		"scope", "mixed", "pkgs", skipped, "remainder", strconv.Itoa(len(scoped)), "backstop", backstop)
	return scoped, nil
}

// retake is the red-first-attempt path. Under a live fleet the -race tier
// also starves for CPU/IO (cycle-943: one package took 469s then failed;
// green in isolation), so a single red is not yet evidence: RETAKE ONCE under
// a cross-lane exclusive lock (isolation on demand — the root cause is
// contention, and serialization removes it). Both attempts persist to
// integration-tier.log (state.json truncates; the artifact is the one-grep
// diagnosis) — attempt 1 BEFORE the lock is sought, so a kill during a
// contended wait never loses it. DELIBERATE trade-offs: worst-case gate
// wall-clock doubles (attempt 1 + a fresh TierAttempt budget — red paths
// only, and a false FAIL discarding a shippable cycle costs far more). The
// verdict is decideTier's.
func (g *Gates) retake(req Request, dir string, args []string, run sysexec.RunFunc, first attempt) ([]string, error) {
	log := &tierLog{workspace: req.Workspace, args: args}
	g.logAttempt(req, log, 1, " (lane env, contended)", first)
	release, lockNote := g.acquireLock(gateTier, req)
	retakeCtx, retakeCancel := context.WithTimeout(context.Background(), g.timeouts.TierAttempt)
	second := runAttempt(retakeCtx, run, dir, args)
	retakeCancel()
	release()
	if second.err == nil {
		g.logAttempt(req, log, 2, " (serialized retake"+lockNote+")", second)
	}
	d := decideTier(first, second, log.path, g.timeouts.TierAttempt)
	switch d.code {
	case CodeTierRetakeExecFailed:
		g.warn(gateTier, req, d.code, "serialized retake could not run: "+second.err.Error(), "err", second.err.Error(), "log", log.path)
	case CodeTierFlakeAbsorbed:
		g.warn(gateTier, req, d.code, d.err.Error(), "attempt1_exit", strconv.Itoa(first.code), "log", log.path, "lock_note", lockNote)
	case CodeTierDeadlineNoVerdict:
		g.warn(gateTier, req, d.code, d.err.Error(), "budget", g.timeouts.TierAttempt.String(), "log", log.path, "lock_note", lockNote)
	}
	if d.err != nil {
		return nil, d.err
	}
	return g.failed(gateTier, req, d.cause, d.offenders, "log", log.path), nil
}

// tierDecision is decideTier's verdict: offenders (FAIL, with cause) or an
// error (WARN), plus the WARN code of a non-FAIL outcome or the pre-FAIL fact
// a retake start failure records.
type tierDecision struct {
	offenders []string
	err       error
	code      signalcenter.Code
	cause     gateCause
}

// decideTier is the PURE four-way table over the two attempts. Retake
// trouble splits by what it can testify to: an EXEC failure falls back to
// attempt-1 offenders (possibly contended data, but a real red must never be
// laundered by retake infra trouble); red-then-green is a contention flake,
// absorbed as a visible WARN — never a false FAIL that discards a shippable
// cycle, and never silent; a DEADLINE kill first reports any offenders the
// truncated output already flushed (go test emits each completed package's
// verdict before the SIGKILL — evidence outranks the budget), and only a
// marker-free truncation degrades to the fail-open WARN, because a bare exit
// code from a killed run is not a test verdict; red-then-red inside budget is
// genuine — the serialized clean-env retake is the truthful attempt.
func decideTier(first, second attempt, logPath string, budget time.Duration) tierDecision {
	if second.err != nil {
		return tierDecision{offenders: offendersWithLogPointer(first, logPath), code: CodeTierRetakeExecFailed, cause: causeRetakeExecFailed}
	}
	if second.code == 0 {
		return tierDecision{code: CodeTierFlakeAbsorbed, err: fmt.Errorf("integration tier was RED under fleet contention but GREEN on a serialized clean-env retake — contention flake absorbed, not a code defect (%s)", tierWhere(logPath))}
	}
	if second.deadlineHit {
		if hasOffenderMarker(second.out + "\n" + second.errOut) {
			return tierDecision{offenders: offendersWithLogPointer(second, logPath), cause: causeDeadlineWithMarkers}
		}
		return tierDecision{code: CodeTierDeadlineNoVerdict, err: fmt.Errorf("integration tier exceeded its %s budget on the serialized retake with no test verdict in the truncated output — a deadline kill is not a judgment; degraded to WARN (%s)", budget, tierWhere(logPath))}
	}
	return tierDecision{offenders: offendersWithLogPointer(second, logPath), cause: causeRetakeRed}
}

func tierWhere(logPath string) string {
	if logPath != "" {
		return "both attempts: " + logPath
	}
	return "integration-tier.log unavailable"
}
