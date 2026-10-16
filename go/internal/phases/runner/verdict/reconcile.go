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

const (
	reportTail = 160
	acsTail    = 200
)

type reconciliation struct {
	reconciled      bool
	verified        deliverable.Result
	acsFloorRescued bool
	overriddenCodes string
	attempts        int
}

func (e *Engine) reconcile(ctx context.Context, d Dispatch) (reconciliation, *core.PhaseResponse, error) {
	if d.BridgeErr == nil {
		return reconciliation{}, nil, nil
	}
	if core.IsInfraTeardownError(d.BridgeErr) {
		return e.reconcileTeardown(ctx, d)
	}
	resp, err := substantiveFail(d)
	return reconciliation{}, &resp, err
}

func (e *Engine) reconcileTeardown(ctx context.Context, d Dispatch) (reconciliation, *core.PhaseResponse, error) {
	roots := rootsFor(d)
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

func staleGate(d Dispatch, stale bool, s settled) settled {
	s.stale = stale
	if s.stale && s.err == nil && s.res.OK {
		s.err = fmt.Errorf("deliverable at %s is byte-identical to the pre-dispatch leftover (a prior attempt's report) — reconcile refused (cycle-1550)", d.ArtifactPath)
		s.res = deliverable.Result{}
		s.refused = true
	}
	return s
}

func staleOf(d Dispatch) bool {
	return d.HadPreDispatch && unchangedSince(d.ArtifactPath, d.PreDispatch)
}

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

func substantiveFail(d Dispatch) (core.PhaseResponse, error) {
	resp := responseBase(d)
	resp.Verdict = core.VerdictFAIL
	resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: d.BridgeErr.Error()}}
	return resp, fmt.Errorf("%s: bridge: %w", d.Phase, d.BridgeErr)
}

func responseBase(d Dispatch) core.PhaseResponse {
	return core.PhaseResponse{Phase: d.Phase, ArtifactsDir: d.Workspace, CostUSD: d.Bridge.CostUSD, Tokens: d.Bridge.Tokens, DurationMS: d.DurationMS, BootMS: d.Bridge.BootMS}
}

func teardownFields(d Dispatch, s settled) map[string]string {
	return map[string]string{
		"teardown": teardownKind(d.BridgeErr), "exit": strconv.Itoa(d.Bridge.ExitCode), "cause": cause(s), "verr": errString(s.err),
		"stale_leftover": strconv.FormatBool(s.stale), "settle_attempts": strconv.Itoa(s.attempts), "deliverable": d.ArtifactPath,
	}
}

func cause(s settled) string {
	switch {
	case s.refused:
		return "stale_leftover"
	case s.err != nil:
		return "unverifiable"
	}
	return "malformed"
}

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
