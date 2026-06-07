package router

// failure_proposal_test.go — failure floor Phase 3: advisor failure-path
// vocabulary. The Proposal gains LearningRichness ("full"|"memo") and
// RecoveryAction ("retry"|"end"), applied ABOVE the deterministic floor:
// the failure-adapter's BLOCK verdicts are non-overridable, richness can
// only choose WHICH learning phase runs (never none), and every clamp is
// recorded — same "LLM proposes, kernel disposes" shape as applyProposal.

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

// Advisor may end a retryable failure early (e.g. budget judgment).
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

// Advisor may retry when the kernel default is proceed-to-end.
func TestRetroDecision_AdvisorChoosesRetryOverEnd(t *testing.T) {
	// Empty history → adapter PROCEED → end; the advisor upgrades to retry.
	d := Route(base("retro"), &Proposal{RecoveryAction: "retry"})
	if d.NextPhase != "tdd" {
		t.Errorf("retro(proceed)+advisor-retry → %q, want tdd", d.NextPhase)
	}
}

// Unrecognized recovery actions neither route nor enter the evidence —
// they are clamped, keeping the kernel branch.
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

// The load-bearing invariant: failure-adapter BLOCK is the floor — no
// advisor proposal may resurrect a blocked cycle; the attempt is clamped.
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

// On the retry path the advisor may localize first: fault-localization /
// bug-reproduction precede tdd in canonical order, so the walk continues
// naturally into the retry after they run.
func TestRetroDecision_AdvisorInsertsFaultLocalization(t *testing.T) {
	in := base("retro")
	in.Strict = true
	in.History = retryableHistory()

	d := Route(in, &Proposal{RecoveryAction: "retry", InsertPhases: []string{"fault-localization"}})
	if d.NextPhase != "fault-localization" {
		t.Errorf("retro(retry)+insert → %q, want fault-localization", d.NextPhase)
	}

	// Only failure-scoped phases may be inserted here.
	d2 := Route(in, &Proposal{RecoveryAction: "retry", InsertPhases: []string{"ship"}})
	if d2.NextPhase != "tdd" {
		t.Errorf("retro(retry)+insert(ship) → %q, want tdd (non-failure insert ignored)", d2.NextPhase)
	}
}

// Richness picks WHICH learning phase runs after an audit FAIL — never
// none. "memo" routes the lightweight memo phase; anything else keeps
// the full retrospective.
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

// Floor invariant: richness can choose memo only when memo is enabled;
// a disabled memo phase keeps the full retrospective (clamped, recorded).
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

	// Retrospective disabled entirely (→ end): the memo proposal still
	// must not vanish silently — evidence + clamp survive.
	in.Cfg.PhaseEnable["retrospective"] = config.EnableOff
	d2 := Route(in, &Proposal{LearningRichness: "memo"})
	if got := d2.Evidence["learning_richness"]; got != "memo" {
		t.Errorf("evidence learning_richness = %v, want memo (never silently dropped)", got)
	}
	if !hasClamp(d2, "failure-proposal-clamped") {
		t.Errorf("non-applicable memo choice must record a clamp; got %+v", d2.Clamps)
	}
}

// The retro transition is a branch transition: under a plan-driven
// advisory cycle the proposer must be consulted there (failure paths are
// exactly where new objective signals appear).
func TestShouldPropose_RetroIsBranchTransition(t *testing.T) {
	in := base("retro")
	in.Plan = &PhasePlan{Entries: []PhasePlanEntry{{Phase: "build", Run: true}}}
	if !shouldPropose(in) {
		t.Error("retrospective must be a branch transition (proposer consulted on failure paths)")
	}
}

// Phase 4a: the audit-FAIL route comes from ONE surface —
// policy.json:failure_floor — not the deprecated env-flag enable chain.
// Policy wins when both are set.
func TestAuditFail_RoutesPerFailurePolicyNotEnableVar(t *testing.T) {
	auditFail := func() RouteInput {
		in := base("audit")
		in.Verdict = "FAIL"
		in.Completed = []string{"scout", "build", "audit"}
		return in
	}

	// Deprecated env-flag path says OFF — policy must still win.
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

	// Policy already routed memo and the advisor proposes memo richness:
	// the proposal AGREES with the decision — evidence recorded, but a
	// clamp here would be forensic noise (nothing was forced).
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

	// Unset (legacy) keeps the deprecated enable-chain behavior.
	t.Run("legacy path unset falls back to enable-chain", func(t *testing.T) {
		in := auditFail()
		in.Cfg.PhaseEnable["retrospective"] = config.EnableOff
		d := Route(in, nil)
		if d.NextPhase != PhaseEnd {
			t.Errorf("audit(FAIL) legacy path with EnableOff → %q, want end", d.NextPhase)
		}
	})
}
