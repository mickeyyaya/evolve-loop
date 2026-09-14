package ciparitygate

// changeset_test.go — the touched∧derivable decision (changeset.go): the ONE
// owner the three whole-repo gates consult.

import (
	"strings"
	"testing"
)

// Test 15 (moved: audit/ciparity_unit_test.go:162-182's decision half,
// audit/ciparity_touchedgo_derivability_test.go:105-135, and the pre-move pin
// TestChangedScopeForGate_NoModuleShortCircuitsBeforeDerivation) — the three
// outcomes over an injected change set, the module guard FIRST, and the
// preserved quirk Q1: three whole-repo gates on one underivable request emit
// three CHANGESET_UNDERIVABLE events, each naming its own gate.
func TestScope_ThreeOutcomesOverAnInjectedChangeSet(t *testing.T) {
	g1 := golden(t, "messages.golden.txt")
	root, _ := goWorktree(t)
	req := tierRequest(root, "")

	calls := 0
	counting := func(set ChangedSetFunc) ChangedSetFunc {
		return func(r string, c int) ([]string, bool) { calls++; return set(r, c) }
	}
	g, events := observed(t, fakeRunFunc(0, "", "", nil), counting(underivableSet()))
	pkgs, run, err := g.scope(gateGoVet, req)
	if pkgs != nil || run || err == nil || err.Error() != g1["changeset.underivable"] {
		t.Fatalf("underivable = (%v, %v, %v), want (nil, false, the golden WARN)", pkgs, run, err)
	}
	e := only(t, *events, CodeChangeSetUnderivable)
	if e.Origin != "Gates.GoVet" || e.Fields["gate"] != "go_vet" || e.Fields["root"] != root || e.Reason != g1["changeset.underivable"] {
		t.Errorf("CHANGESET_UNDERIVABLE names the caller's gate and root: %+v", e)
	}
	*events = nil
	for _, gate := range []func(Request) ([]string, error){g.GoVet, g.ACSDurable, g.IntegrationTier} {
		if off, err := gate(req); off != nil || err == nil || err.Error() != g1["changeset.underivable"] {
			t.Errorf("whole-repo gate on an underivable set: (%v, %v)", off, err)
		}
	}
	if len(*events) != 3 {
		t.Errorf("Q1 preserved: three gates → three events, got %v", codesOf(*events))
	}
	for i, want := range []string{"go_vet", "acs_durable", "integration_tier"} {
		if (*events)[i].Fields["gate"] != want {
			t.Errorf("event %d names %q, want %q", i, (*events)[i].Fields["gate"], want)
		}
	}

	g, events = observed(t, fakeRunFunc(0, "", "", nil), counting(fixedSet()))
	if pkgs, run, err := g.scope(gateGoVet, req); pkgs != nil || run || err != nil || len(*events) != 0 {
		t.Errorf("derivable and empty = (%v, %v, %v) events=%v, want (nil, false, nil) silent", pkgs, run, err, codesOf(*events))
	}

	g, _ = observed(t, fakeRunFunc(0, "", "", nil), counting(fixedSet("./a/...", "./b/...")))
	if pkgs, run, err := g.scope(gateGoVet, req); !run || err != nil || strings.Join(pkgs, " ") != "./a/... ./b/..." {
		t.Errorf("derivable and touched = (%v, %v, %v), want the set in order and run", pkgs, run, err)
	}

	calls = 0
	g, events = observed(t, fakeRunFunc(0, "", "", nil), counting(underivableSet()))
	noModule := Request{1, t.TempDir(), "", ""}
	if pkgs, run, err := g.scope(gateGoVet, noModule); pkgs != nil || run || err != nil || calls != 0 || len(*events) != 0 {
		t.Errorf("no go.mod: (%v, %v, %v) changedSet calls=%d — the module guard precedes derivation", pkgs, run, err, calls)
	}
}
