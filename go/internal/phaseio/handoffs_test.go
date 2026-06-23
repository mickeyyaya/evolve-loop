package phaseio

import "testing"

// TestHandoffs_AuditAccessor_AbsentReturnsFalse is the named RED anchor for
// 3.1: an absent typed view yields the zero value AND ok=false, never a
// fabricated present view. This is the "tolerant of fields it doesn't use" (P5)
// contract — a phase that reads Upstream.Audit() must be able to distinguish
// "no audit yet" from "audit said zero".
func TestHandoffs_AuditAccessor_AbsentReturnsFalse(t *testing.T) {
	h := NewHandoffs(HandoffsInit{}) // nothing populated

	// AuditView holds a map, so it is not == comparable; check ok + a field.
	if v, ok := h.Audit(); ok || v.RedCount != 0 || v.DefectsBySeverity != nil {
		t.Fatalf("absent Audit: got (%+v, %v), want (zero, false)", v, ok)
	}
	if v, ok := h.Scout(); ok || v != (ScoutView{}) {
		t.Fatalf("absent Scout: got (%+v, %v), want (zero, false)", v, ok)
	}
	if _, ok := h.Triage(); ok {
		t.Fatalf("absent Triage: ok=true, want false")
	}
	if _, ok := h.Build(); ok {
		t.Fatalf("absent Build: ok=true, want false")
	}
	if _, ok := h.Generic("anything"); ok {
		t.Fatalf("absent Generic key: ok=true, want false")
	}
	if d := h.Degraded(); len(d) != 0 {
		t.Fatalf("absent Degraded: %v, want empty", d)
	}
}

// TestHandoffs_ZeroValue_Empty pins the documented contract that the zero
// Handoffs is a valid, safe empty value (no constructor required to read it).
func TestHandoffs_ZeroValue_Empty(t *testing.T) {
	var h Handoffs
	if _, ok := h.Scout(); ok {
		t.Errorf("zero Handoffs Scout ok=true")
	}
	if _, ok := h.Generic("x"); ok {
		t.Errorf("zero Handoffs Generic ok=true")
	}
	if d := h.Degraded(); len(d) != 0 {
		t.Errorf("zero Handoffs Degraded = %v", d)
	}
}

func TestHandoffs_PresentViews_RoundTrip(t *testing.T) {
	h := NewHandoffs(HandoffsInit{
		Scout:  &ScoutView{CycleSizeEstimate: "small", ItemCount: 3, CarryoverCount: 1, BacklogSize: 7},
		Triage: &TriageView{CycleSize: "medium", PhaseSkip: []string{"tdd"}},
		Build:  &BuildView{Verdict: "PASS", ACSGreen: 10, ACSRed: 0, ACSTotal: 10, SeverityMax: "HIGH", FilesTouched: 4, DiffLOC: 120},
		Audit:  &AuditView{Verdict: "PASS", Confidence: 0.9, RedCount: 0, DefectsBySeverity: map[string]int{"HIGH": 1}},
	})

	s, ok := h.Scout()
	if !ok || s.CycleSizeEstimate != "small" || s.ItemCount != 3 || s.BacklogSize != 7 {
		t.Fatalf("Scout round-trip: (%+v, %v)", s, ok)
	}
	tr, ok := h.Triage()
	if !ok || tr.CycleSize != "medium" || len(tr.PhaseSkip) != 1 || tr.PhaseSkip[0] != "tdd" {
		t.Fatalf("Triage round-trip: (%+v, %v)", tr, ok)
	}
	b, ok := h.Build()
	if !ok || b.Verdict != "PASS" || b.SeverityMax != "HIGH" || b.DiffLOC != 120 {
		t.Fatalf("Build round-trip: (%+v, %v)", b, ok)
	}
	a, ok := h.Audit()
	if !ok || a.Confidence != 0.9 || a.DefectsBySeverity["HIGH"] != 1 {
		t.Fatalf("Audit round-trip: (%+v, %v)", a, ok)
	}
}

// TestHandoffs_Audit_NilDefectMap covers the real case of an audit with zero
// defects emitting a nil DefectsBySeverity — the sealed copy must stay nil-safe.
func TestHandoffs_Audit_NilDefectMap(t *testing.T) {
	h := NewHandoffs(HandoffsInit{Audit: &AuditView{Verdict: "PASS", RedCount: 0}})
	a, ok := h.Audit()
	if !ok || a.Verdict != "PASS" || a.DefectsBySeverity != nil {
		t.Fatalf("nil-defect audit: (%+v, %v)", a, ok)
	}
	if a.DefectsBySeverity["HIGH"] != 0 { // reading a nil map is safe
		t.Fatalf("nil map read non-zero")
	}
}

func TestHandoffs_Generic_NamespacedLookup(t *testing.T) {
	h := NewHandoffs(HandoffsInit{
		Generic: map[string]any{"security.severity_max": "HIGH", "build.diff_loc": float64(42)},
	})
	if v, ok := h.Generic("security.severity_max"); !ok || v != "HIGH" {
		t.Fatalf("Generic hit: (%v, %v)", v, ok)
	}
	if v, ok := h.Generic("build.diff_loc"); !ok || v != float64(42) {
		t.Fatalf("Generic numeric hit: (%v, %v)", v, ok)
	}
	if _, ok := h.Generic("absent.key"); ok {
		t.Fatalf("Generic miss: ok=true, want false")
	}
}

// TestHandoffs_Generic_NestedValue_Sealed proves the generic plane is deep-sealed:
// a nested map/slice value (JSON objects/arrays are valid signal values) must be
// copied so mutating the source after construction does not leak through Generic.
func TestHandoffs_Generic_NestedValue_Sealed(t *testing.T) {
	nested := map[string]any{"inner": "orig"}
	arr := []any{"a", map[string]any{"k": "orig"}}
	h := NewHandoffs(HandoffsInit{Generic: map[string]any{"obj": nested, "arr": arr}})

	// Mutate the sources after construction.
	nested["inner"] = "MUTATED"
	arr[0] = "MUTATED"
	arr[1].(map[string]any)["k"] = "MUTATED"

	got, _ := h.Generic("obj")
	if got.(map[string]any)["inner"] != "orig" {
		t.Fatalf("nested map value leaked: %v", got)
	}
	gotArr, _ := h.Generic("arr")
	ga := gotArr.([]any)
	if ga[0] != "a" || ga[1].(map[string]any)["k"] != "orig" {
		t.Fatalf("nested slice value leaked: %v", gotArr)
	}
}

func TestHandoffs_Degraded_ListsReadMisses(t *testing.T) {
	h := NewHandoffs(HandoffsInit{Degraded: []string{"handoff-build.json: permission denied"}})
	d := h.Degraded()
	if len(d) != 1 || d[0] != "handoff-build.json: permission denied" {
		t.Fatalf("Degraded: %v", d)
	}
}

// TestHandoffs_Sealed_NoMutationViaInitOrAccessor proves the seal (P5
// idempotency / no shared mutable state): mutating the init maps/slices AFTER
// construction, or mutating a returned slice/copy, must not change the sealed
// Handoffs.
func TestHandoffs_Sealed_NoMutationViaInitOrAccessor(t *testing.T) {
	gen := map[string]any{"k": "v"}
	deg := []string{"miss-1"}
	skip := []string{"tdd"}
	bySev := map[string]int{"HIGH": 1}

	h := NewHandoffs(HandoffsInit{
		Triage:   &TriageView{CycleSize: "small", PhaseSkip: skip},
		Audit:    &AuditView{DefectsBySeverity: bySev},
		Generic:  gen,
		Degraded: deg,
	})

	// Mutate every source after construction.
	gen["k"] = "MUTATED"
	gen["new"] = "X"
	deg[0] = "MUTATED"
	skip[0] = "MUTATED"
	bySev["HIGH"] = 999

	if v, _ := h.Generic("k"); v != "v" {
		t.Fatalf("Generic leaked init mutation: %v", v)
	}
	if _, ok := h.Generic("new"); ok {
		t.Fatalf("Generic leaked init insertion")
	}
	gotDeg := h.Degraded()
	if len(gotDeg) != 1 || gotDeg[0] != "miss-1" {
		t.Fatalf("Degraded leaked init mutation: %v", gotDeg)
	}
	// Mutating the returned Degraded slice must not affect the next read.
	gotDeg[0] = "MUTATED-RETURN"
	if again := h.Degraded(); again[0] != "miss-1" {
		t.Fatalf("Degraded leaked accessor mutation: %v", again)
	}
	tr, _ := h.Triage()
	if tr.PhaseSkip[0] != "tdd" {
		t.Fatalf("Triage.PhaseSkip leaked init mutation: %v", tr.PhaseSkip)
	}
	a, _ := h.Audit()
	if a.DefectsBySeverity["HIGH"] != 1 {
		t.Fatalf("Audit.DefectsBySeverity leaked init mutation: %v", a.DefectsBySeverity)
	}
}
