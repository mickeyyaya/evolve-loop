// Package gatesignal reports each contract gate decision to the Signal Center.
// The gate stays the one verifier; this package only reports.
package gatesignal

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const Origin = "Reviewer.Review"

const (
	CodeVerified   signalcenter.Code = "GATE_CONTRACT_VERIFIED"
	CodeRejected   signalcenter.Code = "GATE_CONTRACT_REJECTED"
	CodeWouldBlock signalcenter.Code = "GATE_CONTRACT_WOULD_BLOCK"
	CodeSalvaged   signalcenter.Code = "GATE_CONTRACT_SALVAGED"
	CodeDemoted    signalcenter.Code = "GATE_CONTRACT_DEMOTED"
	CodeFailOpen   signalcenter.Code = "GATE_CONTRACT_FAIL_OPEN"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeVerified, "the phase's declared deliverables were found in place (fields name the artifact, its size, the agent-owed files and the effects verified) and the phase advanced")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeRejected, "the gate refused the deliverable at enforce; the reason is the correction directive (one [code] message per violation), fields carry the codes and the breaker count (0 for a breaker-exempt phase, an optional evaluate phase whose exhausted ladder degrades to SKIPPED) — the orchestrator's ladder re-dispatches")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeWouldBlock, "the deliverable violated its contract but the stage (shadow/advisory, or the report-size gate's) lets the phase advance; the reason is what enforce would have refused")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeSalvaged, "a sole recoverable bad_verdict was repaired on disk and re-verified clean; the phase advanced on the repaired artifact")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeDemoted, "the breaker opened after N consecutive blocks and demoted enforce→advisory; the phase advanced UNVERIFIED — inspect the failing phase and policy.gates.contract_gate")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeFailOpen, "the gate could not decide (unknown phase, read fault) and failed open; re-dispatching an agent cannot fix this — the reason is the error")
}

type Check struct {
	Cycle int
	RunID string
	Phase string
}

type Verified struct {
	Artifact string
	Bytes    int
	Owed     []string
	Effects  []string
	Stage    string
}

type Reporter struct{ signals *signalcenter.Center }

func New(c *signalcenter.Center) *Reporter { return &Reporter{signals: c} }

func (r *Reporter) Wired() bool { return r.signals != nil }

func prefixed(c Check, reason string) string { return c.Phase + ": " + reason }

func (r *Reporter) Verified(c Check, v Verified) {
	reason := prefixed(c, "deliverables verified — ")
	if v.Artifact == "" {
		reason += "no declared artifact"
	} else {
		reason += v.Artifact + " (" + strconv.Itoa(v.Bytes) + " B)"
	}
	if len(v.Owed) > 0 {
		reason += ", owed: " + strings.Join(v.Owed, ",")
	}
	if len(v.Effects) > 0 {
		reason += ", effects: " + strings.Join(v.Effects, ",")
	}
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityInfo, CodeVerified, reason, map[string]string{
		"artifact": v.Artifact, "bytes": strconv.Itoa(v.Bytes), "owed": strings.Join(v.Owed, ","), "effects": strings.Join(v.Effects, ","), "stage": v.Stage,
	})
}

func (r *Reporter) Rejected(c Check, reason string, codes []string, blocks, threshold int) {
	r.emit(c, signalcenter.KindGateRejected, signalcenter.SeverityWarn, CodeRejected, reason, map[string]string{
		"codes": strings.Join(codes, ","), "blocks": strconv.Itoa(blocks), "threshold": strconv.Itoa(threshold),
	})
}

func (r *Reporter) WouldBlock(c Check, reason string, codes []string, stage string, salvageable bool) {
	fields := map[string]string{"stage": stage, "codes": strings.Join(codes, ",")}
	if salvageable {
		fields["salvage"] = "would"
	}
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityWarn, CodeWouldBlock, reason+" (stage="+stage+", would-block)", fields)
}

func (r *Reporter) Salvaged(c Check, artifact, pattern string) {
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityInfo, CodeSalvaged,
		prefixed(c, "bad_verdict salvaged ("+pattern+"); approved on the repaired "+artifact),
		map[string]string{"artifact": artifact, "pattern": pattern})
}

func (r *Reporter) Demoted(c Check, reason string, blocks int) {
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityWarn, CodeDemoted,
		prefixed(c, "circuit open after "+strconv.Itoa(blocks)+" consecutive contract blocks — enforce demoted to advisory; last: "+reason),
		map[string]string{"blocks": strconv.Itoa(blocks)})
}

func (r *Reporter) FailOpen(c Check, err error) {
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityWarn, CodeFailOpen, prefixed(c, "ambiguity, failing open: "+err.Error()), nil)
}

func (r *Reporter) emit(c Check, kind signalcenter.Kind, sev signalcenter.Severity, code signalcenter.Code, reason string, fields map[string]string) {
	kept := make(map[string]string, len(fields))
	for k, v := range fields {
		if v != "" {
			kept[k] = v
		}
	}
	fields = kept
	r.signals.Emit(signalcenter.Event{
		Cycle: c.Cycle, RunID: c.RunID, Phase: c.Phase, Module: signalcenter.ModuleGateContract, Origin: Origin,
		Kind: kind, Severity: sev, Code: code, Reason: reason, Fields: fields,
	})
}
