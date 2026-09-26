package verdict

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

// Forensic tail widths for the report and acs-verdict.json.
const (
	reportTail = 160
	acsTail    = 200
)

// reconciliation is what reconcile hands classify; verified holds the bytes Classify judges,
// never re-read.
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
	// An infra teardown ends the session, not the verdict: the agent may have
	// written its deliverable first, and reconciling only moves toward that verdict.
	if core.IsInfraTeardownError(d.BridgeErr) {
		return e.reconcileTeardown(ctx, d)
	}
	// Any other bridge error makes on-disk output untrustworthy: hard-fail, optional or not.
	// No event here; core's phase-outcome chokepoint codes the returned error.
	resp, err := substantiveFail(d)
	return reconciliation{}, &resp, err
}

// reconcileTeardown settles under context.WithoutCancel, because a cancelled ctx is often what caused
// the teardown and a late deliverable must still be caught. The arm order below is load-bearing.
func (e *Engine) reconcileTeardown(ctx context.Context, d Dispatch) (reconciliation, *core.PhaseResponse, error) {
	roots := rootsFor(d)
	// Read staleness before the probe: the verifier may repair and rewrite a
	// leftover, which must still be refused as a prior attempt's report.
	stale := staleOf(d)
	s := staleGate(d, stale, e.settle(context.WithoutCancel(ctx), identityOf(d), d.Phase, roots))
	switch {
	case s.err == nil && s.res.OK:
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

// staleGate refuses a deliverable unchanged since the pre-dispatch snapshot: it is a prior attempt's
// report, token and all, so neither reconcile door may accept it. Only an OK probe is refused.
func staleGate(d Dispatch, stale bool, s settled) settled {
	s.stale = stale
	if s.stale && s.err == nil && s.res.OK {
		s.err = fmt.Errorf("deliverable at %s is byte-identical to the pre-dispatch leftover (a prior attempt's report) — reconcile refused (cycle-1550)", d.ArtifactPath)
		s.res = deliverable.Result{}
		s.refused = true
	}
	return s
}

// staleOf reports whether the deliverable still matches the pre-dispatch snapshot.
func staleOf(d Dispatch) bool {
	return d.HadPreDispatch && unchangedSince(d.ArtifactPath, d.PreDispatch)
}

// degradeOptional turns an optional phase's untrustworthy teardown into WARN, because its successor
// runs regardless of its verdict.
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

// rescueViaACSFloor consults the non-LLM ACS verdict before a mandatory phase FAILs: a report can
// verify not-OK at teardown yet be valid. With no probed bytes it reads the file once and adopts path
// and bytes, so the rescue and Classify judge the same bytes. A stale leftover never rescues.
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

// forensicFail is the mandatory-phase hard-fail. Its message and event record why reconcile declined,
// not only that the bridge tore down.
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

// substantiveFail is the hard-fail for a bridge error that is neither infra sentinel.
func substantiveFail(d Dispatch) (core.PhaseResponse, error) {
	resp := responseBase(d)
	resp.Verdict = core.VerdictFAIL
	resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: d.BridgeErr.Error()}}
	return resp, fmt.Errorf("%s: bridge: %w", d.Phase, d.BridgeErr)
}

// responseBase carries the six fields every response shares.
func responseBase(d Dispatch) core.PhaseResponse {
	return core.PhaseResponse{Phase: d.Phase, ArtifactsDir: d.Workspace, CostUSD: d.Bridge.CostUSD, Tokens: d.Bridge.Tokens, DurationMS: d.DurationMS, BootMS: d.Bridge.BootMS}
}

// teardownFields is the field vocabulary both teardown arms share.
func teardownFields(d Dispatch, s settled) map[string]string {
	return map[string]string{
		"teardown": teardownKind(d.BridgeErr), "exit": strconv.Itoa(d.Bridge.ExitCode), "cause": cause(s), "verr": errString(s.err),
		"stale_leftover": strconv.FormatBool(s.stale), "settle_attempts": strconv.Itoa(s.attempts), "deliverable": d.ArtifactPath,
	}
}

// cause names why the deliverable was not trusted; an absent file counts as malformed
// (codes=missing_artifact).
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
