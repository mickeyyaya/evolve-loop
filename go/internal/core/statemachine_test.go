package core

import (
	"testing"
)

// Phase transition graph encoded by NewStateMachine:
//
//	(start)  → intent? → scout → triage → tdd → build → audit
//	                                                      ├─ ship  (PASS / WARN per EGPS)
//	                                                      └─ retro → ship (recovered) | end (BLOCK)
//	                                                      └─ retro → tdd (RETRY)
//
// The transition table is the source of truth; tests validate every
// edge listed in §2 of the parent plan + the failure-adapter PROCEED/
// RETRY/BLOCK semantics.
func TestStateMachine_CanTransition_Allowed(t *testing.T) {
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
		{PhaseTDD, PhaseBuild},
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
	sm := NewStateMachine()
	denied := []struct {
		from, to Phase
	}{
		{PhaseStart, PhaseShip},   // can't ship without building
		{PhaseStart, PhaseBuild},  // skipping scout/tdd
		{PhaseBuild, PhaseShip},   // must audit first
		{PhaseScout, PhaseAudit},  // skipping tdd+build
		{PhaseEnd, PhaseShip},     // terminal
		{PhaseEnd, PhaseStart},    // terminal
		{PhaseShip, PhaseRetro},   // already shipped
		{PhaseTriage, PhaseBuild}, // must go through tdd
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
	if PhaseScout.String() != "scout" {
		t.Errorf("Phase.String wrong: %s", PhaseScout.String())
	}
}

func TestPhase_IsValid(t *testing.T) {
	for _, p := range []Phase{PhaseStart, PhaseIntent, PhaseScout, PhaseTriage,
		PhaseTDD, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro, PhaseEnd} {
		if !p.IsValid() {
			t.Errorf("Phase(%s).IsValid() = false; want true", p)
		}
	}
	if Phase("nonsense").IsValid() {
		t.Error("nonsense phase reported valid")
	}
}
