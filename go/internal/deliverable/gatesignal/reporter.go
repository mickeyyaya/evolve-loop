// Package gatesignal is the contract gate's Signal Center producer (ADR-0101
// S2b, design §15.5): every decision deliverable.Reviewer.Review reaches is
// ONE event under module gate.contract — gate.passed when the phase advanced
// (clean, would-block, salvaged, demoted, fail-open) or gate.rejected when it
// did not — so the orchestrator's listener and the triage reader see, per phase
// boundary, that the deliverables were checked, what was found where, and why
// the phase moved on or was sent back. The gate stays the ONE verifier; this
// package only reports.
package gatesignal

import (
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Origin names the gate's one decision point.
const Origin = "Reviewer.Review"

// The gate's codes, one per decision it can reach.
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
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeRejected, "the gate refused the deliverable at enforce; the reason is the correction directive (one [code] message per violation), fields carry the codes and the breaker count — the orchestrator's ladder re-dispatches")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeWouldBlock, "the deliverable violated its contract but the stage (shadow/advisory, or the report-size gate's) lets the phase advance; the reason is what enforce would have refused")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeSalvaged, "a sole recoverable bad_verdict was repaired on disk and re-verified clean; the phase advanced on the repaired artifact")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeDemoted, "the breaker opened after N consecutive blocks and demoted enforce→advisory; the phase advanced UNVERIFIED — inspect the failing phase and policy.gates.contract_gate")
	signalcenter.RegisterCode(signalcenter.ModuleGateContract, CodeFailOpen, "the gate could not decide (unknown phase, read fault) and failed open; re-dispatching an agent cannot fix this — the reason is the error")
}

// Check identifies the review a decision belongs to.
type Check struct {
	Cycle int
	RunID string
	Phase string
}

// Verified is what a clean verification found in place: the declared set the
// gate knew to look for ("it knows where to search").
type Verified struct {
	Artifact string   // the primary's basename; "" when the contract declares no file
	Bytes    int      // the primary's size
	Owed     []string // agent-owed secondaries verified present, non-empty, parsed
	Effects  []string // declared effects verified
	Stage    string
}

// Reporter is the producer. A nil Center is the Null Object (Emit on nil is
// a no-op); the production root always wires one and Wired proves it.
type Reporter struct{ signals *signalcenter.Center }

// New builds a Reporter over c.
func New(c *signalcenter.Center) *Reporter { return &Reporter{signals: c} }

// Wired reports whether a Center was injected — the root's wiring proof.
func (r *Reporter) Wired() bool { return r.signals != nil }

// prefixed opens a reason with the phase the decision belongs to — the
// prefix every non-delegated reason here shares.
func prefixed(c Check, reason string) string { return c.Phase + ": " + reason }

// Verified reports a clean verification: gate.passed INFO.
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

// Rejected reports a block at enforce: gate.rejected WARN. reason is the
// gate's summary — the correction directive the ladder re-dispatches with.
func (r *Reporter) Rejected(c Check, reason string, codes []string, blocks, threshold int) {
	r.emit(c, signalcenter.KindGateRejected, signalcenter.SeverityWarn, CodeRejected, reason, map[string]string{
		"codes": strings.Join(codes, ","), "blocks": strconv.Itoa(blocks), "threshold": strconv.Itoa(threshold),
	})
}

// WouldBlock reports a violation the stage let through: gate.passed WARN.
// salvageable says a shadow-stage salvage would have repaired the verdict.
func (r *Reporter) WouldBlock(c Check, reason string, codes []string, stage string, salvageable bool) {
	fields := map[string]string{"stage": stage, "codes": strings.Join(codes, ",")}
	if salvageable {
		fields["salvage"] = "would"
	}
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityWarn, CodeWouldBlock, reason+" (stage="+stage+", would-block)", fields)
}

// Salvaged reports a repaired-and-re-verified verdict: gate.passed INFO.
func (r *Reporter) Salvaged(c Check, artifact, pattern string) {
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityInfo, CodeSalvaged,
		prefixed(c, "bad_verdict salvaged ("+pattern+"); approved on the repaired "+artifact),
		map[string]string{"artifact": artifact, "pattern": pattern})
}

// Demoted reports the breaker opening: gate.passed WARN, the phase advanced unverified.
func (r *Reporter) Demoted(c Check, reason string, blocks int) {
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityWarn, CodeDemoted,
		prefixed(c, "circuit open after "+strconv.Itoa(blocks)+" consecutive contract blocks — enforce demoted to advisory; last: "+reason),
		map[string]string{"blocks": strconv.Itoa(blocks)})
}

// FailOpen reports an ambiguity the gate could not decide: gate.passed WARN.
func (r *Reporter) FailOpen(c Check, err error) {
	r.emit(c, signalcenter.KindGatePassed, signalcenter.SeverityWarn, CodeFailOpen, prefixed(c, "ambiguity, failing open: "+err.Error()), nil)
}

// emit raises one event; empty-valued fields are dropped into a copy, the
// caller's map is never mutated.
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
