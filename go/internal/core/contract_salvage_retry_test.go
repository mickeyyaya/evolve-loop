package core

// contract_salvage_retry_test.go — RED-first coverage for the cycle-1300
// fleet-scoped tasks on inbox item contract-block-cli-escalation:
//
//   1. breaker-neutral-salvage-retry-when-no-escalation-family-exists
//   2. demotion-ledger-records-salvage-attempted-vs-no-remedy-possible
//
// The live gap (inbox LIVE EVIDENCE 2026-08-05). contractEscalationCLI returns
// ok=false when a phase's whole dispatch chain is ONE CLI family: there is no
// other family to escalate to. Today the ladder then does nothing at all — the
// same incapable CLI gets the same plain correction directive a third time and
// the breaker opens, so the ratchet fails OPEN purely because there was nowhere
// to escalate to. The remedy this cycle pins is a TOP-FAMILY remedy distinct
// from escalate: the correction that WOULD have escalated instead becomes a
// structured re-prompt (the verbatim validator reason carried under a distinct
// heading), spending round-2 budget on making the diagnosis explicit before the
// last-resort circuit trips.
//
// BREAKER-NEUTRAL is the load-bearing adjective and the anti-no-op axis: the
// remedy must ENRICH the re-dispatch the ladder already performs, never add a
// dispatch and never spend an extra correction. An implementation that inserts
// its own extra retry round fails TestContractEscalation_SalvageRetry_WhenNoOtherFamily's
// dispatch-count assertion.
//
// These tests drive the REAL production path — Orchestrator.RunCycle →
// reviewAndGuard's correction ladder → contractEscalationCLI — against real
// .evolve/profiles/*.json on disk. Nothing here calls the new seam directly, so
// a seam wired into nothing stays RED.

import (
	"context"
	"strings"
	"testing"
)

// salvageProbe is escalationProbe plus the ONE fact these tests turn on that
// escalationProbe does not record: the CorrectionDirective each re-dispatch
// carried. Embedding keeps the breaker/family semantics of the existing probe
// (which the escalation suite already pins) as the single source of truth —
// only Run is overridden, to capture the directive before delegating.
type salvageProbe struct {
	*escalationProbe
	directives []string // CorrectionDirective per dispatch, in order
}

func (p *salvageProbe) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.directives = append(p.directives, req.CorrectionDirective)
	return p.escalationProbe.Run(ctx, req)
}

// runSalvageCycle mirrors runEscalationCycle but wires the directive-recording
// probe as both build runner and reviewer.
func runSalvageCycle(t *testing.T, root string, probe *salvageProbe) (*fakeLedger, error) {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, led, runners, WithReviewer(probe))
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	})
	return led, err
}

// newSalvageProbe builds the probe with the standard blocking behaviour: every
// deliverable violates the contract with the SAME defect, so the block identity
// gate (constraint 4) holds and block 2 reaches the escalation decision.
func newSalvageProbe(threshold int) *salvageProbe {
	return &salvageProbe{escalationProbe: &escalationProbe{phase: "build", threshold: threshold}}
}

// blockReason is the violation escalationProbe reports for a build block. The
// structured re-prompt must carry it VERBATIM — a paraphrased or truncated
// reason is not a diagnosis.
const blockReason = "build deliverable failed contract: [missing_section] required section 'Findings' not found"

// TestContractEscalation_SalvageRetry_WhenNoOtherFamily is the cycle-1300 crux
// (task 1). A profile already on the universal-fallback family with no declared
// chain has NO escalation target — contractEscalationCLI returns ok=false. The
// correction that would have escalated must instead re-dispatch the SAME CLI
// with a STRUCTURED RE-PROMPT: the verbatim validator reason under
// contractSalvageRetryDirectiveHeading.
//
// Three assertions, and all three must hold:
//
//	(a) breaker-neutral — exactly 3 dispatches, the same count as before this
//	    feature. The remedy rides the existing re-dispatch; it neither adds a
//	    round nor spends an extra correction, so the breaker's block accounting
//	    is untouched by the retry itself.
//	(b) it is NOT a shuffle — the re-dispatch stays on the phase's own routing
//	    (ModelRoutingCLI == ""), because there is no other family to move to.
//	(c) the directive is a structured re-prompt carrying the verbatim reason —
//	    and correction 1's directive is NOT (one bad turn is not a CLI verdict;
//	    the remedy belongs to the block that would have escalated).
func TestContractEscalation_SalvageRetry_WhenNoOtherFamily(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", universalContractFallbackCLI, nil)
	probe := newSalvageProbe(neverDemotingThreshold)

	if _, err := runSalvageCycle(t, root, probe); err == nil {
		t.Fatal("expected the cycle to abort after corrections are exhausted (no CLI complies); got nil")
	}
	if len(probe.dispatched) != 3 {
		t.Fatalf("build dispatched %d times (%v), want exactly 3 (initial + 2 corrections) — the salvage retry must be BREAKER-NEUTRAL: it enriches the re-dispatch the ladder already performs, it does not add a round", len(probe.dispatched), probe.dispatched)
	}
	if len(probe.directives) != 3 {
		t.Fatalf("recorded %d directives (%v), want 3", len(probe.directives), probe.directives)
	}
	if probe.dispatched[2] != "" {
		t.Errorf("correction 2 dispatched on ModelRoutingCLI=%q, want \"\" — with no other CLI family available the remedy is a structured re-prompt on the SAME CLI, never a same-family shuffle", probe.dispatched[2])
	}
	if !strings.Contains(probe.directives[2], contractSalvageRetryDirectiveHeading) {
		t.Errorf("correction 2 directive did not carry the structured re-prompt heading %q — with no escalation target the ladder currently falls through unremedied, which is the defect this cycle closes. directive=%q", contractSalvageRetryDirectiveHeading, probe.directives[2])
	}
	if !strings.Contains(probe.directives[2], blockReason) {
		t.Errorf("correction 2 directive did not carry the VERBATIM validator reason %q — a structured re-prompt whose diagnosis is paraphrased is not a diagnosis. directive=%q", blockReason, probe.directives[2])
	}
	if strings.Contains(probe.directives[1], contractSalvageRetryDirectiveHeading) {
		t.Errorf("correction 1 directive carried the structured re-prompt heading — the remedy belongs to the block that would have ESCALATED (block %d), not to the first block. directive=%q", contractEscalateAtBlock, probe.directives[1])
	}
}

// TestContractEscalation_SalvageRetry_NotWhenEscalationTargetExists is the
// precision guard: the two remedies are DISJOINT. A phase with a real
// cross-family escalation target escalates exactly as it does today and must NOT
// also spend its round-2 budget on a structured re-prompt (double-spending the
// correction cap for one block).
func TestContractEscalation_SalvageRetry_NotWhenEscalationTargetExists(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := newSalvageProbe(neverDemotingThreshold)

	if _, err := runSalvageCycle(t, root, probe); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	if len(probe.dispatched) != 3 || probe.dispatched[2] != "codex-tmux" {
		t.Fatalf("dispatch chain = %v, want the 3rd dispatch escalated to codex-tmux (the pre-existing behaviour must be unchanged)", probe.dispatched)
	}
	for i, d := range probe.directives {
		if strings.Contains(d, contractSalvageRetryDirectiveHeading) {
			t.Errorf("dispatch %d carried the structured re-prompt heading although a real escalation target existed — escalate and salvage-retry are disjoint remedies; firing both double-spends one block's budget. directive=%q", i, d)
		}
	}
}

// TestContractEscalation_SalvageRetry_NotOnFirstBlock is the edge guard on the
// SAME trigger constraint the escalation obeys (scoping constraint 2): one
// malformed turn is a bad turn, not a CLI verdict. With no escalation target at
// all, a first block must still get the plain correction — otherwise the remedy
// fires on 99% of honest single-turn slips.
func TestContractEscalation_SalvageRetry_NotOnFirstBlock(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", universalContractFallbackCLI, nil)
	probe := newSalvageProbe(neverDemotingThreshold)
	probe.approveAfter = 1 // block once, then approve: the ladder stops after correction 1

	if _, err := runSalvageCycle(t, root, probe); err != nil {
		t.Fatalf("RunCycle should proceed after one correction: %v", err)
	}
	if len(probe.directives) != 2 {
		t.Fatalf("recorded %d directives (%v), want 2 (initial + 1 correction)", len(probe.directives), probe.directives)
	}
	if strings.Contains(probe.directives[1], contractSalvageRetryDirectiveHeading) {
		t.Errorf("the FIRST contract block produced a structured re-prompt — the remedy starts at block %d, exactly where escalation would have. directive=%q", contractEscalateAtBlock, probe.directives[1])
	}
}

// TestContractEscalation_SalvageRetry_NotOnNonContractRejection is the scoping
// guard inherited from constraint 3: the other gates chained at this seam
// (evalgate / topngate / triagecap / the build floor) report Blocks==0 because
// they keep no contract-block counter. A wrong-task binding is not a
// format-compliance failure, so a structured re-prompt of a CONTRACT violation
// is not the remedy — the ladder must behave exactly as before.
func TestContractEscalation_SalvageRetry_NotOnNonContractRejection(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", universalContractFallbackCLI, nil)
	probe := newSalvageProbe(neverDemotingThreshold)
	rev := &recordingReviewer{
		default_: ReviewResult{Approve: true},
		decide:   map[string]ReviewResult{"build": {Approve: false, Reason: "build report task slug outside triage top_n"}},
	}
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithReviewer(rev))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true,
	}); err == nil {
		t.Fatal("expected abort after corrections exhausted; got nil")
	}
	for i, d := range probe.directives {
		if strings.Contains(d, contractSalvageRetryDirectiveHeading) {
			t.Errorf("dispatch %d carried the structured re-prompt heading for a Blocks==0 rejection — only a CONTRACT block earns this remedy. directive=%q", i, d)
		}
	}
}

// TestContractEscalation_SalvageRetry_LedgerRecordsSalvageAttempted is task 2's
// acceptance criterion. When the structured re-prompt ALSO blocks, the circuit
// still opens as the last resort — but the demotion record must now distinguish
// "a remedy was tried and failed" from "no remedy was possible". Without it an
// operator reading the ledger cannot tell an incapable CLI from an unremedied
// ladder, which is the exact ambiguity the live-evidence note is about.
func TestContractEscalation_SalvageRetry_LedgerRecordsSalvageAttempted(t *testing.T) {
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", universalContractFallbackCLI, nil)
	probe := newSalvageProbe(3) // third consecutive block opens the circuit

	led, err := runSalvageCycle(t, root, probe)
	if err != nil {
		t.Fatalf("a demoted gate approves, so the cycle must complete: %v", err)
	}
	var demoted *LedgerEntry
	for i := range led.entries {
		if led.entries[i].Kind == ledgerKindContractGateDemoted {
			demoted = &led.entries[i]
		}
	}
	if demoted == nil {
		kinds := make([]string, 0, len(led.entries))
		for _, e := range led.entries {
			kinds = append(kinds, e.Kind)
		}
		t.Fatalf("no %q ledger entry — the circuit must still open as the last resort (kinds=%v)", ledgerKindContractGateDemoted, kinds)
	}
	if !strings.Contains(demoted.Action, "salvage_attempted=true") {
		t.Errorf("demotion ledger Action=%q, want it to record salvage_attempted=true — a demotion where a structured re-prompt was tried and failed is a DIFFERENT diagnosis from one where no remedy existed, and recurrence analytics cannot separate them from an escalated=false entry alone", demoted.Action)
	}
	if !strings.Contains(demoted.Action, "escalated=false") {
		t.Errorf("demotion ledger Action=%q, want escalated=false preserved — the salvage retry is NOT an escalation and must not be reported as one", demoted.Action)
	}
}

// TestContractEscalation_SalvageRetry_WarnDistinguishesAttemptFromNoRemedy pins
// the operator-facing half of task 2. The current WARN's "escalation did NOT
// run" line is the only thing an operator sees, and after this cycle it is
// MISLEADING when a structured re-prompt was in fact attempted: a line an
// operator trusts and acts on must be false only when it is false.
func TestContractEscalation_SalvageRetry_WarnDistinguishesAttemptFromNoRemedy(t *testing.T) {
	t.Parallel()
	reason := "triage deliverable failed contract: [missing_failure_block] schema_version 2 required"

	tried := formatContractGateDemotionWarn("triage", universalContractFallbackCLI, contractDispatch{salvageRetried: true}, reason)
	if !strings.Contains(tried, "salvage retry attempted") {
		t.Errorf("salvageRetried=true WARN must SAY the salvage retry was attempted and failed: %s", tried)
	}
	if strings.Contains(tried, "did NOT run") {
		t.Errorf("salvageRetried=true WARN still claims no remedy ran — that is the misleading line this task removes: %s", tried)
	}
	for _, want := range []string{"WARN", "CONTRACT GATE DEMOTED", "triage", universalContractFallbackCLI, "advisory", "missing_failure_block"} {
		if !strings.Contains(tried, want) {
			t.Errorf("demotion WARN missing %q: %s", want, tried)
		}
	}

	// Negative axis: nothing was tried, so nothing may be claimed.
	noRemedy := formatContractGateDemotionWarn("triage", universalContractFallbackCLI, contractDispatch{}, reason)
	if !strings.Contains(noRemedy, "did NOT run") {
		t.Errorf("with neither escalation nor salvage retry, the WARN must still report that no remedy ran: %s", noRemedy)
	}
	if strings.Contains(noRemedy, "salvage retry attempted") {
		t.Errorf("no remedy was attempted, but the WARN claims one was: %s", noRemedy)
	}
}
