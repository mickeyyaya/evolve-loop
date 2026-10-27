package selfheal

import (
	"slices"
	"testing"
	"time"
)

func TestIncident_ClosesOnlyWithTheFourConditions(t *testing.T) {
	t.Parallel()
	open := Incident{ID: "inc-2026-10-27-0a1b2c3d", State: StateOpen, OpenedAt: t0}
	all := Conditions{ReproGreen: true, SignalClear: true, QualityHeld: true, Retrospective: true}
	cases := []struct {
		name    string
		cond    Conditions
		missing string
	}{
		{"no green reproduction", Conditions{SignalClear: true, QualityHeld: true, Retrospective: true}, CondReproGreen},
		{"the signal fired in the last wave", Conditions{ReproGreen: true, QualityHeld: true, Retrospective: true}, CondSignalClear},
		{"a quality index regressed", Conditions{ReproGreen: true, SignalClear: true, Retrospective: true}, CondQualityHeld},
		{"no recovery retrospective", Conditions{ReproGreen: true, SignalClear: true, QualityHeld: true}, CondRetrospective},
	}
	for _, c := range cases {
		got, missing := open.Close(c.cond, t0.Add(time.Hour))
		if got.State != StateOpen || got.ClosedAt != nil || !slices.Equal(missing, []string{c.missing}) {
			t.Errorf("%s: the incident must stay open and name %q as missing, got state %s missing %v", c.name, c.missing, got.State, missing)
		}
	}
	got, missing := open.Close(all, t0.Add(time.Hour))
	if got.State != StateClosed || got.ClosedAt == nil || !got.ClosedAt.Equal(t0.Add(time.Hour)) || len(missing) != 0 {
		t.Fatalf("with all four conditions the incident closes at the pass time: %+v %v", got, missing)
	}
	if open.State != StateOpen || open.ClosedAt != nil {
		t.Fatal("Close returns a new record and leaves the receiver as it was")
	}
}

func TestIncident_CloseNamesEachMissingConditionInOrder(t *testing.T) {
	t.Parallel()
	_, missing := Incident{State: StateOpen}.Close(Conditions{}, t0)
	if !slices.Equal(missing, []string{CondReproGreen, CondSignalClear, CondQualityHeld, CondRetrospective}) {
		t.Fatalf("missing = %v", missing)
	}
}

func TestTierOf_EachClassHasATierAndProtectedEvidenceIsTheKernel(t *testing.T) {
	t.Parallel()
	for _, c := range Classes() {
		if tierOf(c, nil) == "" {
			t.Errorf("class %s has no tier", c)
		}
	}
	if tierOf(ClassFlakyTest, nil) != TierItem || tierOf(ClassRepairLoop, nil) != TierPipeline {
		t.Fatal("a flaky test is item code (T2); a repair loop is pipeline code (T1)")
	}
	if tierOf(ClassFlakyTest, []string{"go/internal/guards/role.go"}) != TierKernel {
		t.Fatal("evidence on a protected surface is T0 for each class")
	}
}

func TestIncidentID_IsStableForOneFingerprintAndDay(t *testing.T) {
	t.Parallel()
	a, b := incidentID("quality:QUALITY_PQ1_BREACH", t0), incidentID("quality:QUALITY_PQ1_BREACH", t0.Add(time.Hour))
	if a != b || a != "inc-2026-10-27-"+fingerprintHash("quality:QUALITY_PQ1_BREACH") || len(fingerprintHash("x")) != 8 {
		t.Fatalf("ids %q %q", a, b)
	}
	if incidentID("main-red:main", t0) == a {
		t.Fatal("two fingerprints get two ids")
	}
}
