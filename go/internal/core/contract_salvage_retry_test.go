package core

import (
	"context"
	"strings"
	"testing"
)

type salvageProbe struct {
	*escalationProbe
	directives []string
}

func (p *salvageProbe) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.directives = append(p.directives, req.CorrectionDirective)
	return p.escalationProbe.Run(ctx, req)
}

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

func newSalvageProbe(threshold int) *salvageProbe {
	return &salvageProbe{escalationProbe: &escalationProbe{phase: "build", threshold: threshold}}
}

const blockReason = "build deliverable failed contract: [missing_section] required section 'Findings' not found"

func TestContractEscalation_SalvageRetry_WhenNoOtherFamily(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", nil)
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

func TestContractEscalation_SalvageRetry_NotWhenEscalationTargetExists(t *testing.T) {
	t.Parallel()

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

func TestContractEscalation_SalvageRetry_NotOnFirstBlock(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", nil)
	probe := newSalvageProbe(neverDemotingThreshold)
	probe.approveAfter = 1

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

func TestContractEscalation_SalvageRetry_NotOnNonContractRejection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", nil)
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

func TestContractEscalation_SalvageRetry_LedgerRecordsSalvageAttempted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", nil)
	probe := newSalvageProbe(3)

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

func TestContractEscalation_SalvageRetry_WarnDistinguishesAttemptFromNoRemedy(t *testing.T) {
	t.Parallel()
	reason := "triage deliverable failed contract: [missing_failure_block] schema_version 2 required"

	tried := formatContractGateDemotionWarn("triage", "claude-tmux", contractDispatch{salvageRetried: true}, reason)
	if !strings.Contains(tried, "salvage retry attempted") {
		t.Errorf("salvageRetried=true WARN must SAY the salvage retry was attempted and failed: %s", tried)
	}
	if strings.Contains(tried, "did NOT run") {
		t.Errorf("salvageRetried=true WARN still claims no remedy ran — that is the misleading line this task removes: %s", tried)
	}
	for _, want := range []string{"WARN", "CONTRACT GATE DEMOTED", "triage", "claude-tmux", "advisory", "missing_failure_block"} {
		if !strings.Contains(tried, want) {
			t.Errorf("demotion WARN missing %q: %s", want, tried)
		}
	}

	noRemedy := formatContractGateDemotionWarn("triage", "claude-tmux", contractDispatch{}, reason)
	if !strings.Contains(noRemedy, "did NOT run") {
		t.Errorf("with neither escalation nor salvage retry, the WARN must still report that no remedy ran: %s", noRemedy)
	}
	if strings.Contains(noRemedy, "salvage retry attempted") {
		t.Errorf("no remedy was attempted, but the WARN claims one was: %s", noRemedy)
	}
}
