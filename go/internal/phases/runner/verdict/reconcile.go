package verdict

// reconcile.go — the reconcile step: bridge teardown interpreted separately
// from the agent's deliverable verdict. The arm ORDER is load-bearing and
// preserved verbatim: stale-leftover refusal → well-formed (fall through to
// Classify) → optional degrade → ACS deterministic floor → forensic FAIL; a
// substantive bridge error hard-fails without consulting the deliverable.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// The forensic tails: the artifact's last 160 bytes (where the verdict
// sentinel lives) and acs-verdict.json's last 200.
const (
	reportTail = 160
	acsTail    = 200
)

// reconciliation is what the reconcile step hands the classify step: whether
// a teardown was overridden, the probe result whose bytes Classify must judge
// (never re-read), whether the ACS floor did the override and which codes it
// overrode, and the re-probe count for the trail.
type reconciliation struct {
	reconciled      bool
	verified        deliverable.Result
	acsFloorRescued bool
	overriddenCodes string
	attempts        int
}

// reconcile interprets the bridge outcome. It returns an early response only
// when the phase cannot safely continue to normal classification.
func (e *Engine) reconcile(ctx context.Context, d Dispatch) (reconciliation, *core.PhaseResponse, error) {
	if d.BridgeErr == nil {
		return reconciliation{}, nil, nil
	}
	// A bridge INFRA teardown — an artifact-wait timeout (exit 81) OR a
	// transient failure (exit 80/85/86: quota, liveness-exhaustion) — ends the
	// SESSION, but is not a verdict: the agent may have written its contracted
	// deliverable before the teardown (cycle-254/255 timeout false-FAIL;
	// cycle-835 quota false-FAIL). Reconcile against the deliverable: if it is
	// on disk and well-formed, trust its verdict (via Classify) instead of
	// synthesizing FAIL. Reconciliation can only UPGRADE toward the agent's
	// real verdict, never downgrade a real one.
	if core.IsInfraTeardownError(d.BridgeErr) {
		return e.reconcileTeardown(ctx, d)
	}
	// A substantive bridge error (launch/boot/safety/cost — NEITHER infra
	// sentinel) means the process failed in a way that makes any on-disk
	// output untrustworthy: hard-fail, optional or not, without consulting
	// the deliverable. No event: the C1 chokepoint codes this error.
	resp, err := substantiveFail(d)
	return reconciliation{}, &resp, err
}

// reconcileTeardown runs the CANCELLATION-IMMUNE ladder (WithoutCancel,
// deliberate): on THIS path a cancelled ctx is frequently the CAUSE of the
// teardown, not a reason to stop waiting — the tmux driver, on ctx.Err(),
// takes one final completion poll and otherwise exits ExitArtifactTimeout,
// laundering a finished session into a timeout; honoring cancellation here
// would re-open the cycles-824/825 false-FAIL class. Then the arms, in order.
func (e *Engine) reconcileTeardown(ctx context.Context, d Dispatch) (reconciliation, *core.PhaseResponse, error) {
	roots := rootsFor(d)
	s := staleGate(d, e.settle(context.WithoutCancel(ctx), d.Phase, roots))
	switch {
	case s.err == nil && s.res.OK:
		// Deliverable survived the teardown — fall through to Classify.
		return reconciliation{reconciled: true, verified: s.res, attempts: s.attempts}, nil, nil
	case e.optional:
		resp := e.degradeOptional(d, s)
		return reconciliation{}, &resp, nil
	}
	if r, ok := rescueViaACSFloor(d, s); ok {
		return r, nil, nil
	}
	resp, err := e.forensicFail(d, roots, s)
	return reconciliation{}, &resp, err
}

// staleGate is the cycle-1550 refusal: a deliverable byte-identical to the
// pre-dispatch snapshot is the PRIOR attempt's report — well-formed,
// cycle-scoped challenge token and all — and reconciling it would re-grade a
// stale verdict as this session's work. Refuse BOTH reconcile doors (the
// well-formed fall-through and the ACS floor) and let the optional/fatal arms
// handle it as untrustworthy, with the cause on record. The refusal fires
// only on an OK probe; a stale but malformed leftover stays malformed.
func staleGate(d Dispatch, s settled) settled {
	s.stale = d.HadPreDispatch && unchangedSince(d.ArtifactPath, d.PreDispatch)
	if s.stale && s.err == nil && s.res.OK {
		s.err = fmt.Errorf("deliverable at %s is byte-identical to the pre-dispatch leftover (a prior attempt's report) — reconcile refused (cycle-1550)", d.ArtifactPath)
		s.res = deliverable.Result{}
		s.refused = true
	}
	return s
}

// degradeOptional is the optional-phase soft-fail (Workstream D / cycle-120):
// no trustworthy deliverable, but an optional phase's successor is
// verdict-unconditional, so degrade to WARN and let the cycle advance.
func (e *Engine) degradeOptional(d Dispatch, s settled) core.PhaseResponse {
	msg := fmt.Sprintf("optional phase %q degraded: no trustworthy deliverable after a bridge infra teardown (%v); cycle continues", d.Phase, d.BridgeErr)
	if s.err != nil {
		msg = fmt.Sprintf("%s [deliverable unverifiable: %v]", msg, s.err)
	}
	e.warn("Engine.degradeOptional", d, CodeOptionalPhaseDegraded, msg, teardownFields(d, s))
	resp := responseBase(d)
	resp.Verdict = core.VerdictWARN
	resp.Diagnostics = []core.Diagnostic{{Severity: "warning", Message: msg}}
	return resp
}

// rescueViaACSFloor consults the NON-LLM ground truth before a mandatory
// phase is discarded as FAIL: the teardown-time Verify can return not-OK on a
// report that standalone-verifies OK (a session-lifecycle artifact — the
// agent wrote a valid report, then idled without the completion handshake
// until ctx-cancel), NOT a defect. The verified snapshot is BOTH the rescue's
// evidence and the classified content: when the probe produced no bytes (an
// infra read fault, or an artifact that appeared after the last probe), fall
// back to ONE disk read and adopt path + bytes into the snapshot, so the
// rescue and the classification judge the same bytes and Classify never reads
// a third time. The read runs before the stale check (order preserved); a
// stale leftover never rescues.
func rescueViaACSFloor(d Dispatch, s settled) (reconciliation, bool) {
	res := s.res
	if res.Content == "" {
		if data, readErr := os.ReadFile(d.ArtifactPath); readErr == nil {
			res.ArtifactPath, res.Content = d.ArtifactPath, string(data)
		}
	}
	if s.stale || !acsFloorRescues(d.Phase, d.Workspace, res.Content) {
		return reconciliation{}, false
	}
	return reconciliation{reconciled: true, verified: res, acsFloorRescued: true, overriddenCodes: forensicCodes(res.Violations), attempts: s.attempts}, true
}

// forensicFail is the mandatory-phase hard-fail: no trustworthy deliverable
// (absent/malformed/unverifiable), enriched with the well-formedness
// violation when there is one — including the stale-leftover refusal, so the
// forensic dig sees WHY reconcile declined, not only that the bridge timed
// out. The event carries the roots the probe judged, the codes, the on-disk
// state and the cause (the retro's cycle-3 ask: a teardown false-FAIL used to
// record no reasoning).
func (e *Engine) forensicFail(d Dispatch, roots phasecontract.Roots, s settled) (core.PhaseResponse, error) {
	msg := d.BridgeErr.Error()
	if s.err == nil && len(s.res.Violations) > 0 {
		msg = fmt.Sprintf("%s; deliverable not trustworthy: %s", msg, s.res.Violations[0].Message)
	} else if s.err != nil {
		msg = fmt.Sprintf("%s; deliverable not trustworthy: %v", msg, s.err)
	}
	fields := teardownFields(d, s)
	fields["codes"] = forensicCodes(s.res.Violations)
	fields["roots"] = fmt.Sprintf("ws=%s wt=%s evolve=%s", roots.Workspace, roots.Worktree, roots.EvolveDir)
	fields["report"] = forensicSnapshot(d.ArtifactPath, reportTail)
	fields["acs"] = forensicSnapshot(filepath.Join(d.Workspace, "acs-verdict.json"), acsTail)
	e.warn("Engine.forensicFail", d, CodeTeardownFail, msg, fields)
	resp := responseBase(d)
	resp.Verdict = core.VerdictFAIL
	resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: msg}}
	return resp, fmt.Errorf("%s: bridge: %w", d.Phase, d.BridgeErr)
}

// substantiveFail is the hard-fail for a bridge error that is neither infra
// sentinel: the bare error as the diagnostic, the same wrapped return.
func substantiveFail(d Dispatch) (core.PhaseResponse, error) {
	resp := responseBase(d)
	resp.Verdict = core.VerdictFAIL
	resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: d.BridgeErr.Error()}}
	return resp, fmt.Errorf("%s: bridge: %w", d.Phase, d.BridgeErr)
}

// responseBase is the six fields every response literal shared.
func responseBase(d Dispatch) core.PhaseResponse {
	return core.PhaseResponse{Phase: d.Phase, ArtifactsDir: d.Workspace, CostUSD: d.Bridge.CostUSD, Tokens: d.Bridge.Tokens, DurationMS: d.DurationMS, BootMS: d.Bridge.BootMS}
}

// teardownFields is the vocabulary the teardown arms share: which teardown,
// the exit code, why the deliverable was not trusted, the probe error, the
// stale flag, the re-probe count and the artifact.
func teardownFields(d Dispatch, s settled) map[string]string {
	return map[string]string{
		"teardown": teardownKind(d.BridgeErr), "exit": strconv.Itoa(d.Bridge.ExitCode), "cause": cause(s), "verr": errString(s.err),
		"stale_leftover": strconv.FormatBool(s.stale), "settle_attempts": strconv.Itoa(s.attempts), "deliverable": d.ArtifactPath,
	}
}

// cause names why the deliverable was not trusted: the stale refusal fired,
// the probe errored (no contract / IO fault), or the probe judged it
// malformed (absent is codes=missing_artifact).
func cause(s settled) string {
	switch {
	case s.refused:
		return "stale_leftover"
	case s.err != nil:
		return "unverifiable"
	}
	return "malformed"
}

// teardownKind names the infra sentinel: timeout (exit 81) or transient.
func teardownKind(err error) string {
	if errors.Is(err, core.ErrArtifactTimeout) {
		return "timeout"
	}
	return "transient"
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
