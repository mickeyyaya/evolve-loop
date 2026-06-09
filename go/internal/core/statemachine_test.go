package core

import (
	"testing"
)

// Phase transition graph encoded by NewStateMachine:
//
//	(start)  → intent? → scout → triage → tdd → build-planner → build → audit
//	                                                                      ├─ ship  (PASS / WARN per EGPS)
//	                                                                      └─ retro → ship (recovered) | end (BLOCK)
//	                                                                      └─ retro → tdd (RETRY)
//	build-planner is skipped (SKIPPED verdict) when EVOLVE_BUILD_PLANNER != "1".
//
// The transition table is the source of truth; tests validate every
// edge listed in §2 of the parent plan + the failure-adapter PROCEED/
// RETRY/BLOCK semantics.
func TestStateMachine_CanTransition_Allowed(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	allowed := []struct {
		from, to Phase
	}{
		{PhaseStart, PhaseIntent},
		{PhaseStart, PhaseScout}, // intent optional
		{PhaseIntent, PhaseScout},
		{PhaseScout, PhaseTriage},
		{PhaseScout, PhaseTDD}, // triage may be skipped (EVOLVE_TRIAGE_DISABLE)
		{PhaseTriage, PhaseTDD},
		{PhaseTDD, PhaseBuildPlanner},
		{PhaseTDD, PhaseBuild}, // direct edge kept for skip-through CanTransition checks
		{PhaseBuildPlanner, PhaseBuild},
		{PhaseBuild, PhaseAudit},
		{PhaseAudit, PhaseShip},
		{PhaseAudit, PhaseRetro},
		{PhaseRetro, PhaseShip}, // retro recovered → ship
		{PhaseRetro, PhaseTDD},  // retro retry → re-enter loop
		{PhaseShip, PhaseEnd},
		{PhaseRetro, PhaseEnd}, // retro BLOCK → end
	}
	for _, edge := range allowed {
		t.Run(string(edge.from)+"→"+string(edge.to), func(t *testing.T) {
			if !sm.CanTransition(edge.from, edge.to) {
				t.Errorf("CanTransition(%s, %s) = false; want true", edge.from, edge.to)
			}
		})
	}
}

func TestStateMachine_CanTransition_Disallowed(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	denied := []struct {
		from, to Phase
	}{
		{PhaseStart, PhaseShip},  // can't ship without building
		{PhaseStart, PhaseBuild}, // skipping scout/tdd
		{PhaseBuild, PhaseShip},  // must audit first
		{PhaseScout, PhaseAudit}, // skipping tdd+build
		{PhaseEnd, PhaseShip},    // terminal
		{PhaseEnd, PhaseStart},   // terminal
		{PhaseShip, PhaseRetro},  // already shipped
		// NOTE: triage→build and scout→build are now LEGAL — dynamic routing
		// skips tdd on trivial cycles (tdd is conditional-mandatory, not a hard
		// gate). The artifact-backed SpineSatisfiedUpTo gate enforces the spine.
	}
	for _, edge := range denied {
		t.Run(string(edge.from)+"→"+string(edge.to), func(t *testing.T) {
			if sm.CanTransition(edge.from, edge.to) {
				t.Errorf("CanTransition(%s, %s) = true; want false", edge.from, edge.to)
			}
		})
	}
}

// Next determines the post-phase target based on Verdict + caller hints.
// Audit verdict drives the most important branch (ship vs retro).
func TestStateMachine_Next(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	cases := []struct {
		name    string
		current Phase
		verdict string
		want    Phase
		wantErr bool
	}{
		{"audit_pass_ships", PhaseAudit, VerdictPASS, PhaseShip, false},
		{"audit_warn_ships_egps", PhaseAudit, VerdictWARN, PhaseShip, false},
		{"audit_fail_retros", PhaseAudit, VerdictFAIL, PhaseRetro, false},
		{"build_pass_audits", PhaseBuild, VerdictPASS, PhaseAudit, false},
		{"scout_pass_triages", PhaseScout, VerdictPASS, PhaseTriage, false},
		{"ship_pass_ends", PhaseShip, VerdictPASS, PhaseEnd, false},
		{"end_terminal", PhaseEnd, VerdictPASS, "", true},
		{"invalid_phase", Phase("nonsense"), VerdictPASS, "", true},
		{"start_advances_to_scout", PhaseStart, VerdictPASS, PhaseScout, false},
		{"intent_advances_to_scout", PhaseIntent, VerdictPASS, PhaseScout, false},
		{"triage_advances_to_tdd", PhaseTriage, VerdictPASS, PhaseTDD, false},
		{"tdd_advances_to_build_planner", PhaseTDD, VerdictPASS, PhaseBuildPlanner, false},
		{"build_planner_advances_to_build", PhaseBuildPlanner, VerdictPASS, PhaseBuild, false},
		{"build_planner_skipped_still_advances", PhaseBuildPlanner, VerdictSKIPPED, PhaseBuild, false},
		{"retro_defaults_to_end", PhaseRetro, VerdictPASS, PhaseEnd, false},
		{"audit_unknown_verdict_errors", PhaseAudit, VerdictSKIPPED, "", true},
		{"audit_garbage_verdict_errors", PhaseAudit, "garbage", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sm.Next(tc.current, tc.verdict)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("Next(%s, %s)=%s, want %s", tc.current, tc.verdict, got, tc.want)
			}
		})
	}
}

func TestPhase_String(t *testing.T) {
	t.Parallel()
	if PhaseScout.String() != "scout" {
		t.Errorf("Phase.String wrong: %s", PhaseScout.String())
	}
}

func TestStateMachine_CanTransition_InvalidPhase(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
	if sm.CanTransition(Phase("garbage"), PhaseScout) {
		t.Error("invalid from accepted")
	}
	if sm.CanTransition(PhaseScout, Phase("garbage")) {
		t.Error("invalid to accepted")
	}
}

func TestPhase_IsValid(t *testing.T) {
	t.Parallel()
	for _, p := range []Phase{PhaseStart, PhaseIntent, PhaseScout, PhaseTriage,
		PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro, PhaseEnd} {
		if !p.IsValid() {
			t.Errorf("Phase(%s).IsValid() = false; want true", p)
		}
	}
	if Phase("nonsense").IsValid() {
		t.Error("nonsense phase reported valid")
	}
}
