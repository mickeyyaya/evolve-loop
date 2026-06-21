package core

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// specForCatalog adapts a test catalog to the StateMachine.specFor accessor,
// canonicalizing the name exactly as the orchestrator's specFor does.
func specForCatalog(cat phasespec.Catalog) func(Phase) (phasespec.PhaseSpec, bool) {
	return func(p Phase) (phasespec.PhaseSpec, bool) { return cat.Get(canonicalCatalogName(p)) }
}

func mustCatalog(t *testing.T, specs ...phasespec.PhaseSpec) phasespec.Catalog {
	t.Helper()
	cat, err := phasespec.Catalog{}.Merge(specs)
	if err != nil {
		t.Fatalf("catalog merge: %v", err)
	}
	return cat
}

// TestNext_AuditVerdictBranchFromSpec proves Next() resolves the audit verdict
// branch from the descriptor's on_pass/on_fail, NOT the hardcoded case
// PhaseAudit. The catalog INVERTS the targets vs the literal table (PASS→retro,
// FAIL→ship) so the assertion can only pass if the spec is consulted. RED until
// Next reads sm.specFor (the literal gives PASS→ship).
func TestNext_AuditVerdictBranchFromSpec(t *testing.T) {
	t.Parallel()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "retrospective", OnFail: "ship"})
	sm := NewStateMachine().WithCatalog(specForCatalog(cat))

	for _, c := range []struct {
		v    string
		want Phase
	}{
		{VerdictPASS, PhaseRetro},
		{VerdictWARN, PhaseRetro},
		{VerdictFAIL, PhaseShip},
	} {
		got, err := sm.Next(PhaseAudit, c.v)
		if err != nil || got != c.want {
			t.Errorf("Next(audit,%q) = (%q,%v), want (%q,nil) — verdict branch must come from spec.OnPass/OnFail", c.v, got, err, c.want)
		}
	}
}

// TestNext_AuditDegradesToLiteralWhenCatalogUnset is the degrade characterization:
// a bare StateMachine (no WithCatalog) keeps the exact literal audit branch.
// Green before AND after the Next rewrite — the safety net for catalog-less SMs.
func TestNext_AuditDegradesToLiteralWhenCatalogUnset(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine() // no catalog
	for _, c := range []struct {
		v    string
		want Phase
	}{
		{VerdictPASS, PhaseShip},
		{VerdictWARN, PhaseShip},
		{VerdictFAIL, PhaseRetro},
	} {
		got, err := sm.Next(PhaseAudit, c.v)
		if err != nil || got != c.want {
			t.Errorf("bare Next(audit,%q) = (%q,%v), want (%q,nil) — must degrade to the literal table", c.v, got, err, c.want)
		}
	}
}

// TestWiredStateMachine_ReproducesAuditOracle proves the SHIPPED config shape
// (audit on_pass:ship / on_fail:retrospective) reproduces the frozen oracle's
// audit cells byte-identically when the catalog is wired — the live-path
// byte-identity proof for S1.
func TestWiredStateMachine_ReproducesAuditOracle(t *testing.T) {
	t.Parallel()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "ship", OnFail: "retrospective"})
	sm := NewStateMachine().WithCatalog(specForCatalog(cat))
	for v, want := range nextGoldenAudit {
		got, err := sm.Next(PhaseAudit, v)
		gotErr := ""
		if err != nil {
			gotErr = err.Error()
		}
		if got != want.next || gotErr != want.errText {
			t.Errorf("wired Next(audit,%q) = (%q,%q), want (%q,%q) — must match the frozen oracle", v, got, gotErr, want.next, want.errText)
		}
	}
}

// TestNext_UnresolvableTargetErrorsLoudly guards the fail-loudly contract: a
// descriptor whose on_pass/on_fail names a phase that does not resolve (an
// operator typo) must surface ErrTransitionInvalid — never the silent ("",nil)
// success that would let a ship-intended cycle skip its successor unnoticed.
func TestNext_UnresolvableTargetErrorsLoudly(t *testing.T) {
	t.Parallel()
	cat := mustCatalog(t, phasespec.PhaseSpec{Name: "audit", OnPass: "no-such-phase", OnFail: "also-bogus"})
	sm := NewStateMachine().WithCatalog(specForCatalog(cat))

	for _, v := range []string{VerdictPASS, VerdictWARN, VerdictFAIL} {
		got, err := sm.Next(PhaseAudit, v)
		if got != "" || !errors.Is(err, ErrTransitionInvalid) {
			t.Errorf("Next(audit,%q) = (%q,%v), want (\"\", ErrTransitionInvalid) — an unresolvable on_pass/on_fail must fail loudly, not return an empty phase with nil error", v, got, err)
		}
	}
}
