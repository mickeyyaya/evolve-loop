// Package ciparitygate is unit 14 of the component breakdown (ADR-0103): the
// audit phase's five CI-parity decisions — the deterministic gates that decide
// whether a cycle may ship. Each runs, against THIS cycle's worktree, the EXACT
// command .github/workflows runs (go vet ./..., the -tags acs durable suite,
// the -tags integration -race tier over the touched packages with its
// cross-lane serialized clean-env retake, apicover -enforce folded in-process
// over the touched∩enforced set, and the new-package graduation check), so a
// cycle can never ship green-locally / red-in-CI.
//
// One Gates owns the five decisions over four injected collaborators: the
// subprocess runner, the change-set Strategy (the host's locator — git and the
// build handoff never enter the leaf), a clock for the lock wait, and the
// Signal Center through an accessor read at every use. The hook contract is
// the host's, verbatim: ([]offenders, nil) → FAIL; (nil, err) → WARN, the gate
// could not run (fail-open); (nil, nil) → clean. The leaf never writes stderr;
// its ten failure modes are audit.warning WARNs under module audit, coded
// AUDIT_CIPARITY_*. Design: docs/architecture/decomposition/14-ciparity.md.
package ciparitygate

import (
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// The unit's codes — ten WARN conditions under module audit; every one fires
// only on a provoked fault or a FAIL (a clean five-gate run emits nothing).
const (
	CodeChangeSetUnderivable    signalcenter.Code = "AUDIT_CIPARITY_CHANGESET_UNDERIVABLE"
	CodeGateStepFailed          signalcenter.Code = "AUDIT_CIPARITY_GATE_STEP_FAILED"
	CodeGateFailed              signalcenter.Code = "AUDIT_CIPARITY_GATE_FAILED"
	CodeTierEnvExclusiveSkipped signalcenter.Code = "AUDIT_CIPARITY_TIER_ENV_EXCLUSIVE_SKIPPED"
	CodeTierFlakeAbsorbed       signalcenter.Code = "AUDIT_CIPARITY_TIER_FLAKE_ABSORBED"
	CodeTierDeadlineNoVerdict   signalcenter.Code = "AUDIT_CIPARITY_TIER_DEADLINE_NO_VERDICT"
	CodeTierRetakeExecFailed    signalcenter.Code = "AUDIT_CIPARITY_TIER_RETAKE_EXEC_FAILED"
	CodeTierLockUnavailable     signalcenter.Code = "AUDIT_CIPARITY_TIER_LOCK_UNAVAILABLE"
	CodeTierLogWriteFailed      signalcenter.Code = "AUDIT_CIPARITY_TIER_LOG_WRITE_FAILED"
	CodeGraduationDeferred      signalcenter.Code = "AUDIT_CIPARITY_GRADUATION_DEFERRED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeChangeSetUnderivable, "the cycle's changed-package set was underivable (git failed — e.g. a concurrent-fleet .git/index.lock race) and a whole-repo CI-parity gate (go vet, acs-durable, integration tier) SKIPPED fail-open with a WARN diagnostic; one event per gate, CI backstops the check; fields.gate, root")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeGateStepFailed, "a CI-parity gate's pre-verdict step could not run and the gate failed OPEN (the diagnostic is the WARN the audit renders): fields.step ∈ "+vocabulary(gateSteps)+"; fields.gate, err, cmd (the exec/list/fork steps)")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeGateFailed, "a CI-parity gate returned offenders — the audit FAILs and the cycle routes to retro; fields.cause ∈ "+vocabulary(gateCauses)+", fields.gate, offenders (count), first (the first offender), log (the integration-tier.log when written), exit")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierEnvExclusiveSkipped, "touched package(s) are env-exclusive under a live loop: scope=all — the integration tier skipped with a WARN; scope=mixed — the runnable remainder ran without them; fields.gate, scope, pkgs, remainder, backstop (the record's honest backstop, verbatim)")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierFlakeAbsorbed, "the integration tier was RED under fleet contention but GREEN on the serialized clean-env retake — a contention flake absorbed as a WARN, not a code defect; fields.gate, attempt1_exit, log, lock_note")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierDeadlineNoVerdict, "the serialized integration-tier retake hit its per-attempt budget with no test verdict in the truncated output — a deadline kill is not a judgment, degraded to WARN; fields.gate, budget, log, lock_note")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierRetakeExecFailed, "the serialized integration-tier retake could not START (silent before unit 14); the gate still FAILs on attempt 1's offenders — a real red is never laundered by retake infra trouble — and a GATE_FAILED cause=retake_exec_failed follows; fields.gate, err, log")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierLockUnavailable, "the cross-lane integration-tier retake lock degraded (best-effort by design — the retake ran unserialized): fields.reason ∈ "+vocabulary(lockReasons)+"; fields.gate, path, err, wait")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeTierLogWriteFailed, "integration-tier.log could not be opened or appended for an attempt (an empty Workspace is a declared no-op, not a fault); an earlier successful write keeps its pointer in the offenders; fields.gate, path, attempt, err")
	signalcenter.RegisterCode(signalcenter.ModuleAudit, CodeGraduationDeferred, "an ungraduated new package has no production .go surface (test-only or absent) so the graduation gate did not flag it; the enrollment obligation re-raises when production code lands; fields.gate, pkg, dir")
}

// Request is the ONE input shape: the four core.PhaseRequest fields the gates
// read. The host projects it once (audit.requestOf).
type Request struct {
	Cycle       int
	ProjectRoot string // the cycle-shared runtime root: <ProjectRoot>/.evolve/locks/integration-tier.lock
	Worktree    string // the shipped tree: the go module and the change set live here
	Workspace   string // core.RunWorkspacePath — where integration-tier.log AND the cycle's signals.ndjson land
}

// root is the tree the gates inspect: the worktree, else the project root —
// the ONE spelling of a belief the file used to state four times.
func (r Request) root() string {
	if r.Worktree != "" {
		return r.Worktree
	}
	return r.ProjectRoot
}

// lockRoot is deliberately REVERSED (project root first): the retake lock must
// live on the CYCLE-SHARED path so lanes contend on ONE file; a per-lane
// worktree path would defeat cross-lane serialization.
func (r Request) lockRoot() string {
	if r.ProjectRoot != "" {
		return r.ProjectRoot
	}
	return r.Worktree
}

// Timeouts are the five compiled budgets — ONE home. GoVet and ACSDurable
// bound one command; Apicover bounds the forked pre-steps AND the in-process
// measurement together; TierAttempt bounds EACH integration-tier attempt
// (first run and serialized retake separately); TierLockWait bounds the retake
// lock wait as an INDEPENDENT budget, never the attempt-1 leftovers (under the
// exact contention the retake exists to absorb, attempt 1 may have consumed
// most of the tier deadline).
type Timeouts struct {
	GoVet, ACSDurable, Apicover, TierAttempt, TierLockWait time.Duration
}

// DefaultTimeouts are the production budgets (compiled defaults, never env
// toggles; a policy.json home is follow-up 14-2).
func DefaultTimeouts() Timeouts {
	return Timeouts{GoVet: 4 * time.Minute, ACSDurable: 8 * time.Minute, Apicover: 8 * time.Minute, TierAttempt: 15 * time.Minute, TierLockWait: 5 * time.Minute}
}

// ChangedSetFunc is the change-set Strategy: (pkgs, derivable) for a cycle,
// rooted at projectRoot. The host passes its locator (the build handoff, then
// git through gitexec — outside the runner seam), so the leaf never sees git.
type ChangedSetFunc func(projectRoot string, cycle int) ([]string, bool)

// Gates owns the five CI-parity decisions. Stateless between calls: per-call
// state lives in attempt / tierLog / tierDecision values.
type Gates struct {
	run        sysexec.RunFunc
	changedSet ChangedSetFunc
	timeouts   Timeouts
	now        func() time.Time
	sleep      func(time.Duration)
	signals    func() *signalcenter.Center
}

// Option configures Gates at construction (functional options).
type Option func(*Gates)

// New builds the gates over the two required collaborators; a nil runner or
// change-set function is a programming error and panics at first use — no guard.
func New(run sysexec.RunFunc, changedSet ChangedSetFunc, opts ...Option) *Gates {
	g := &Gates{run: run, changedSet: changedSet, timeouts: DefaultTimeouts(), now: time.Now, sleep: time.Sleep}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// WithTimeouts replaces the five budgets (tests shrink one to reach a
// deadline arm without the wait).
func WithTimeouts(t Timeouts) Option { return func(g *Gates) { g.timeouts = t } }

// WithClock replaces the clock the retake lock wait reads and the 2 s poll it
// sleeps through.
func WithClock(now func() time.Time, sleep func(time.Duration)) Option {
	return func(g *Gates) { g.now, g.sleep = now, sleep }
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use. A nil accessor, or one returning nil, is the
// Null Object; SignalsWired proves a root wired one.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(g *Gates) { g.signals = c }
}

// SignalsWired reports whether the gates currently reach a Center.
func (g *Gates) SignalsWired() bool { return g.center() != nil }

func (g *Gates) center() *signalcenter.Center {
	if g.signals == nil {
		return nil
	}
	return g.signals()
}

// gateOrigin pairs an exported method's Origin with its fields.gate token; it
// is passed down into the shared steps so a shared-step event names the gate
// the triage is looking at.
type gateOrigin struct{ origin, gate string }

var (
	gateGoVet      = gateOrigin{"Gates.GoVet", "go_vet"}
	gateACSDurable = gateOrigin{"Gates.ACSDurable", "acs_durable"}
	gateTier       = gateOrigin{"Gates.IntegrationTier", "integration_tier"}
	gateEnforce    = gateOrigin{"Gates.ApicoverEnforce", "apicover_enforce"}
	gateGraduation = gateOrigin{"Gates.ApicoverGraduation", "apicover_graduation"}
)

// warn is the unit's ONE producer: an audit.warning WARN under module audit,
// stamped with the cycle and the audit phase, fields.gate on every event. kv
// are key/value PAIRS — an odd count is a programming error and panics (the
// nil-runner stance), never a silently dropped field. A nil Center is the
// Null Object (Emit on nil is a no-op).
func (g *Gates) warn(at gateOrigin, req Request, code signalcenter.Code, reason string, kv ...string) {
	if len(kv)%2 != 0 {
		panic("ciparitygate: warn needs key/value pairs, got " + strconv.Itoa(len(kv)) + " strings")
	}
	fields := map[string]string{"gate": at.gate}
	for i := 0; i < len(kv); i += 2 {
		fields[kv[i]] = kv[i+1]
	}
	g.center().Emit(signalcenter.Event{
		Cycle: req.Cycle, Phase: string(cyclestate.PhaseAudit), Module: signalcenter.ModuleAudit, Origin: at.origin,
		Kind: signalcenter.KindAuditWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}

// failed is the FAIL projection onto warn — the ONE writer of fields.cause:
// every offender return emits exactly one GATE_FAILED naming its cause, the
// offender count and the first offender, and returns the offenders unchanged.
func (g *Gates) failed(at gateOrigin, req Request, cause gateCause, offenders []string, kv ...string) []string {
	kv = append(kv, "cause", string(cause), "offenders", strconv.Itoa(len(offenders)), "first", offenders[0])
	g.warn(at, req, CodeGateFailed, strings.Join(offenders, "; "), kv...)
	return offenders
}

// stepFailed is the fail-OPEN projection onto warn — the ONE writer of
// fields.step: a pre-verdict step could not run; reason is the diagnostic the
// audit renders (the wrapped error), err the step's own failure, kv the
// step-specific fields (cmd for the exec/list/fork steps).
func (g *Gates) stepFailed(at gateOrigin, req Request, step gateStep, reason string, err error, kv ...string) {
	g.warn(at, req, CodeGateStepFailed, reason, append([]string{"step", string(step), "err", err.Error()}, kv...)...)
}
