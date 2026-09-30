package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPhaseRequest_BridgeCompletion_ASourceWritingCorrectionCompletesOnWorktreeEvidence(t *testing.T) {
	req := PhaseRequest{Worktree: "/wt/cycle-1", CorrectionDirective: composeCorrection(1, "[missing_what_why] explanation doc", "")}
	if got := req.BridgeCompletion(); got != CompletionWorktreeEvidence {
		t.Fatalf("a correction re-dispatch of a source-writing phase must ask the bridge for the worktree-evidence contract; got %q", got)
	}
}

func TestPhaseRequest_BridgeCompletion_EveryOtherDispatchKeepsTheArtifactContract(t *testing.T) {
	cases := []struct {
		name string
		req  PhaseRequest
	}{
		{"a first dispatch refuses a leftover", PhaseRequest{Worktree: "/wt/cycle-1"}},
		{"a read-only phase completes only by rewriting its verdict", PhaseRequest{Worktree: "/wt/cycle-1", CorrectionDirective: "fix it", WorktreeReadOnly: true}},
		{"a degraded run has no worktree to take evidence from", PhaseRequest{CorrectionDirective: "fix it"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.BridgeCompletion(); got != "" {
				t.Fatalf("BridgeCompletion() = %q, want the artifact contract (\"\")", got)
			}
		})
	}
}

func TestPhaseRequest_BridgeCompletion_TheWorktreeFenceDecidesWhoQualifies(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	correction := PhaseRequest{Worktree: "/wt/cycle-1", CorrectionDirective: composeCorrection(1, "rejected", "")}
	for _, phase := range []Phase{PhaseAudit, Phase("adversarial-review")} {
		if got := o.withWorktreeFence(correction, phase, CycleState{}).BridgeCompletion(); got != "" {
			t.Errorf("read-only phase %s qualified for worktree evidence (%q): its stale verdict must be rewritten after dispatch", phase, got)
		}
	}
	for _, phase := range []Phase{PhaseBuild, PhaseTDD} {
		if got := o.withWorktreeFence(correction, phase, CycleState{}).BridgeCompletion(); got != CompletionWorktreeEvidence {
			t.Errorf("source-writing phase %s: BridgeCompletion() = %q, want %q", phase, got, CompletionWorktreeEvidence)
		}
	}
}

func TestComposeCorrection_AsksForAnAppendedSectionNumberedByTheRound(t *testing.T) {
	for _, remediation := range []string{"", gateARemediation} {
		got := composeCorrection(2, "[missing_what_why] docs/explain/builds/cycle-1-r1.md", remediation)
		want := "Before you finish, append a `## Correction 2` section to the deliverable"
		if !strings.Contains(got, want) {
			t.Errorf("remediation=%q: the directive must ask for the round's own appended section %q\n  got: %s", remediation, want, got)
		}
		if !strings.Contains(got, "also when the fix is in another file") {
			t.Errorf("remediation=%q: the directive must say the section is owed even when the fix lives outside the deliverable\n  got: %s", remediation, got)
		}
		if strings.Contains(got, "## Correction 1") {
			t.Errorf("remediation=%q: round 2 must not be recorded as round 1\n  got: %s", remediation, got)
		}
	}
}

func TestCorrectionRecord_StatesTheAgentsDutyNeverTheHostsCompletionRule(t *testing.T) {
	record := correctionRecord(2)
	for _, completionRule := range []string{"host", "complete"} {
		if strings.Contains(record, completionRule) {
			t.Errorf("the record request mentions %q: it tells the agent what to append and why, and the bridge alone decides completion\n  got: %s", completionRule, record)
		}
	}
}

func TestBridgeRequest_EachCompletionContractTravelsAsTheNameTheBridgeParses(t *testing.T) {
	for contract, wire := range map[CompletionContract]string{
		CompletionArtifact:         "artifact",
		CompletionStdout:           "stdout",
		CompletionGit:              "git",
		CompletionWorktreeEvidence: "worktree-evidence",
	} {
		b, err := json.Marshal(BridgeRequest{Completion: contract})
		if err != nil {
			t.Fatal(err)
		}
		if want := `"completion":"` + wire + `"`; !strings.Contains(string(b), want) {
			t.Errorf("BridgeRequest{Completion: %s} marshals as %s, want %s: the bridge's --completion flag and BRIDGE_COMPLETION read this name", contract, b, want)
		}
	}
}
