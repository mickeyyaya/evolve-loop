package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
)

func retryableHistory() []failureadapter.Entry {
	return []failureadapter.Entry{
		{Cycle: 1, Classification: failureadapter.InfraTransient, ExpiresAt: "2099-01-01T00:00:00Z"},
	}
}

func blockHistory() []failureadapter.Entry {
	return []failureadapter.Entry{
		{Cycle: 1, Classification: failureadapter.CodeAuditFail, ExpiresAt: "2099-01-01T00:00:00Z"},
		{Cycle: 2, Classification: failureadapter.CodeAuditFail, ExpiresAt: "2099-01-01T00:00:00Z"},
	}
}

func TestRetroDecision_AdvisorChoosesEndOverRetry(t *testing.T) {
	in := base("retro")
	in.Strict = true
	in.History = retryableHistory()

	d := Route(in, &Proposal{RecoveryAction: "end", Justification: "budget nearly exhausted"})
	if d.NextPhase != PhaseEnd {
		t.Errorf("retro(retry)+advisor-end → %q, want end", d.NextPhase)
	}
	if got := d.Evidence["recovery_action"]; got != "end" {
		t.Errorf("evidence recovery_action = %v, want end", got)
	}
}

func TestRetroDecision_AdvisorChoosesRetryOverEnd(t *testing.T) {
	// Empty history makes the adapter PROCEED to end.
	d := Route(base("retro"), &Proposal{RecoveryAction: "retry"})
	if d.NextPhase != "tdd" {
		t.Errorf("retro(proceed)+advisor-retry → %q, want tdd", d.NextPhase)
	}
}

func TestRetroDecision_UnknownRecoveryActionClamped(t *testing.T) {
	in := base("retro")
	in.Strict = true
	in.History = retryableHistory() // kernel default: retry→tdd

	d := Route(in, &Proposal{RecoveryAction: "halt-and-catch-fire"})
	if d.NextPhase != "tdd" {
		t.Errorf("unknown action must keep the kernel branch, got %q", d.NextPhase)
	}
	if _, ok := d.Evidence["recovery_action"]; ok {
		t.Error("unvalidated recovery_action must not enter the evidence")
	}
	found := false
	for _, c := range d.Clamps {
		if c.Rule == "failure-proposal-clamped" && c.Proposed == "halt-and-catch-fire" {
			found = true
		}
	}
	if !found {
		t.Errorf("unknown action must be clamp-recorded; got %+v", d.Clamps)
	}
}

func TestRetroDecision_BlockVerdictNonOverridable(t *testing.T) {
	in := base("retro")
	in.Strict = true
	in.History = blockHistory()

	d := Route(in, &Proposal{RecoveryAction: "retry", Justification: "try once more"})
	if d.NextPhase != PhaseEnd {
		t.Fatalf("retro(BLOCK)+advisor-retry → %q, want end (BLOCK non-overridable)", d.NextPhase)
	}
	found := false
	for _, c := range d.Clamps {
		if c.Rule == "failure-proposal-clamped" {
			found = true
			if c.Forced != PhaseEnd {
				t.Errorf("clamp forced = %q, want end", c.Forced)
			}
		}
	}
	if !found {
		t.Errorf("BLOCK override attempt must record a failure-proposal-clamped clamp; got %+v", d.Clamps)
	}
}

func TestRetroDecision_AdvisorInsertsFaultLocalization(t *testing.T) {
	in := base("retro")
	in.Strict = true
	in.History = retryableHistory()

	d := Route(in, &Proposal{RecoveryAction: "retry", InsertPhases: []string{"fault-localization"}})
	if d.NextPhase != "fault-localization" {
		t.Errorf("retro(retry)+insert → %q, want fault-localization", d.NextPhase)
	}

	d2 := Route(in, &Proposal{RecoveryAction: "retry", InsertPhases: []string{"ship"}})
	if d2.NextPhase != "tdd" {
		t.Errorf("retro(retry)+insert(ship) → %q, want tdd (non-failure insert ignored)", d2.NextPhase)
	}
}

func TestAuditFail_AdvisorChoosesMemoOverFullRetro(t *testing.T) {
	in := base("audit")
	in.Verdict = "FAIL"
	in.Completed = []string{"scout", "build", "audit"}

	d := Route(in, &Proposal{LearningRichness: "memo"})
	if d.NextPhase != "memo" {
		t.Errorf("audit(FAIL)+richness=memo → %q, want memo", d.NextPhase)
	}
	if got := d.Evidence["learning_richness"]; got != "memo" {
		t.Errorf("evidence learning_richness = %v, want memo", got)
	}

	for _, richness := range []string{"", "full", "garbage"} {
		d := Route(in, &Proposal{LearningRichness: richness})
		if d.NextPhase != "retrospective" {
			t.Errorf("audit(FAIL)+richness=%q → %q, want retrospective", richness, d.NextPhase)
		}
	}
}

func TestAuditFail_RichnessNeverSuppressesLearning(t *testing.T) {
	in := base("audit")
	in.Verdict = "FAIL"
	in.Completed = []string{"scout", "build", "audit"}
	in.Cfg.PhaseEnable["memo"] = config.EnableOff

	d := Route(in, &Proposal{LearningRichness: "memo"})
	if d.NextPhase != "retrospective" {
		t.Fatalf("audit(FAIL)+richness=memo(disabled) → %q, want retrospective (floor wins)", d.NextPhase)
	}
	if !hasClamp(d, "failure-proposal-clamped") {
		t.Errorf("suppressed memo choice must record a clamp; got %+v", d.Clamps)
	}

	in.Cfg.PhaseEnable["retrospective"] = config.EnableOff
	d2 := Route(in, &Proposal{LearningRichness: "memo"})
	if got := d2.Evidence["learning_richness"]; got != "memo" {
		t.Errorf("evidence learning_richness = %v, want memo (never silently dropped)", got)
	}
	if !hasClamp(d2, "failure-proposal-clamped") {
		t.Errorf("non-applicable memo choice must record a clamp; got %+v", d2.Clamps)
	}
}

func TestShouldPropose_RetroIsBranchTransition(t *testing.T) {
	in := base("retro")
	in.Plan = &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true}}}
	if !shouldPropose(in) {
		t.Error("retrospective must be a branch transition (proposer consulted on failure paths)")
	}
}

func TestAuditFail_RoutesPerFailurePolicyNotEnableVar(t *testing.T) {
	auditFail := func() RouteInput {
		in := base("audit")
		in.Verdict = "FAIL"
		in.Completed = []string{"scout", "build", "audit"}
		return in
	}

	t.Run("memo route beats enable-chain off", func(t *testing.T) {
		in := auditFail()
		in.Cfg.AuditFailRoutesTo = "memo"
		in.Cfg.PhaseEnable["retrospective"] = config.EnableOff
		d := Route(in, nil)
		if d.NextPhase != "memo" {
			t.Errorf("audit(FAIL) with failure_floor route=memo → %q, want memo (policy is the one surface)", d.NextPhase)
		}
	})

	t.Run("retrospective route beats enable-chain off", func(t *testing.T) {
		in := auditFail()
		in.Cfg.AuditFailRoutesTo = "retrospective"
		in.Cfg.PhaseEnable["retrospective"] = config.EnableOff
		d := Route(in, nil)
		if d.NextPhase != "retrospective" {
			t.Errorf("audit(FAIL) with failure_floor route=retrospective → %q, want retrospective", d.NextPhase)
		}
	})

	t.Run("memo route with agreeing memo proposal records no clamp", func(t *testing.T) {
		in := auditFail()
		in.Cfg.AuditFailRoutesTo = "memo"
		d := Route(in, &Proposal{LearningRichness: "memo"})
		if d.NextPhase != "memo" {
			t.Fatalf("audit(FAIL) route=memo + richness=memo → %q, want memo", d.NextPhase)
		}
		if got := d.Evidence["learning_richness"]; got != "memo" {
			t.Errorf("evidence learning_richness = %v, want memo", got)
		}
		if hasClamp(d, "failure-proposal-clamped") {
			t.Errorf("agreeing proposal must not be clamp-recorded; got %+v", d.Clamps)
		}
	})

	t.Run("legacy path unset falls back to enable-chain", func(t *testing.T) {
		in := auditFail()
		in.Cfg.PhaseEnable["retrospective"] = config.EnableOff
		d := Route(in, nil)
		if d.NextPhase != PhaseEnd {
			t.Errorf("audit(FAIL) legacy path with EnableOff → %q, want end", d.NextPhase)
		}
	})
}
