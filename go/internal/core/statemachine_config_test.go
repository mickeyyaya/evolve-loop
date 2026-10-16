package core

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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

func TestNext_AuditDegradesToLiteralWhenCatalogUnset(t *testing.T) {
	t.Parallel()
	sm := NewStateMachine()
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
