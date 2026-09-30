// Package ciparitygate runs the audit phase's five CI-parity gates, the exact
// commands CI runs, against a cycle's worktree so no cycle ships green locally
// and red in CI. See docs/architecture/packages/internal-phases-audit-ciparitygate.md.
package ciparitygate

import (
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

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

type Request struct {
	Cycle       int
	ProjectRoot string
	Worktree    string
	Workspace   string
}

func (r Request) root() string {
	if r.Worktree != "" {
		return r.Worktree
	}
	return r.ProjectRoot
}

func (r Request) lockRoot() string {
	if r.ProjectRoot != "" {
		return r.ProjectRoot
	}
	return r.Worktree
}

type Timeouts struct {
	GoVet, ACSDurable, Apicover, TierAttempt, TierLockWait time.Duration
}

func DefaultTimeouts() Timeouts {
	return Timeouts{GoVet: 4 * time.Minute, ACSDurable: 8 * time.Minute, Apicover: 8 * time.Minute, TierAttempt: 15 * time.Minute, TierLockWait: 5 * time.Minute}
}

type ChangedSetFunc func(projectRoot string, cycle int) ([]string, bool)

type Gates struct {
	run        sysexec.RunFunc
	changedSet ChangedSetFunc
	timeouts   Timeouts
	now        func() time.Time
	sleep      func(time.Duration)
	signals    func() *signalcenter.Center
}

type Option func(*Gates)

func New(run sysexec.RunFunc, changedSet ChangedSetFunc, opts ...Option) *Gates {
	g := &Gates{run: run, changedSet: changedSet, timeouts: DefaultTimeouts(), now: time.Now, sleep: time.Sleep}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

func WithTimeouts(t Timeouts) Option { return func(g *Gates) { g.timeouts = t } }

func WithClock(now func() time.Time, sleep func(time.Duration)) Option {
	return func(g *Gates) { g.now, g.sleep = now, sleep }
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(g *Gates) { g.signals = c }
}

func (g *Gates) SignalsWired() bool { return g.center() != nil }

func (g *Gates) center() *signalcenter.Center {
	if g.signals == nil {
		return nil
	}
	return g.signals()
}

type gateOrigin struct{ origin, gate string }

var (
	gateGoVet      = gateOrigin{"Gates.GoVet", "go_vet"}
	gateACSDurable = gateOrigin{"Gates.ACSDurable", "acs_durable"}
	gateTier       = gateOrigin{"Gates.IntegrationTier", "integration_tier"}
	gateEnforce    = gateOrigin{"Gates.ApicoverEnforce", "apicover_enforce"}
	gateGraduation = gateOrigin{"Gates.ApicoverGraduation", "apicover_graduation"}
)

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

func (g *Gates) failed(at gateOrigin, req Request, cause gateCause, offenders []string, kv ...string) []string {
	kv = append(kv, "cause", string(cause), "offenders", strconv.Itoa(len(offenders)), "first", offenders[0])
	g.warn(at, req, CodeGateFailed, strings.Join(offenders, "; "), kv...)
	return offenders
}

func (g *Gates) stepFailed(at gateOrigin, req Request, step gateStep, reason string, err error, kv ...string) {
	g.warn(at, req, CodeGateStepFailed, reason, append([]string{"step", string(step), "err", err.Error()}, kv...)...)
}
