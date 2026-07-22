package fleet

// triageplan_amplify_test.go — test-amplification (salvaged from cycle 465)
// for the PlanFromTriage contract. Black-box against the spec only (tdd
// handoff + build-report contract lines): floors → one Todo per DISTINCT id
// → PlanCycles disjoint lanes; cards are a fallback when floors are
// absent/empty, never a union; malformed input rejects with zero specs;
// degenerate counts and large-scale inputs never panic or over-schedule.

import (
	"encoding/json"
	"fmt"
	"testing"
)

// specScopeUnion collects every Env-scoped todo id across specs, failing the
// test on any id that appears in more than one spec (cross-lane disjointness).
func specScopeUnion(t *testing.T, specs []CycleSpec) map[string]bool {
	t.Helper()
	union := map[string]bool{}
	for i, spec := range specs {
		for id := range scopeIDs(spec) {
			if union[id] {
				t.Errorf("todo id %q appears in more than one lane spec (spec[%d]) — lanes must be pairwise disjoint", id, i)
			}
			union[id] = true
		}
	}
	return union
}

// TestPlanFromTriage_DuplicateFloorsNeverOverSchedule (edge): triage output
// repeating one floor id must collapse to one Todo per DISTINCT id — 2
// distinct ids over count=3 yield exactly 2 specs, and no spec's scope may
// list an id twice. Kills an adapter that skips dedup and either pads lanes
// with duplicates or ships a "core,core,core" scope to a lane's triage.
func TestPlanFromTriage_DuplicateFloorsNeverOverSchedule(t *testing.T) {
	decisionJSON := []byte(`{"committed_floors":["core","core","audit","core"]}`)
	specs, _, err := PlanFromTriage(decisionJSON, nil, 3, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d, want 2 (duplicate floors dedup to 2 distinct ids; empty buckets yield no spec)", len(specs))
	}
	for i, spec := range specs {
		seen := map[string]bool{}
		for _, id := range spec.Scope {
			if seen[id] {
				t.Errorf("spec[%d].Scope lists id %q more than once — duplicates must collapse before partitioning", i, id)
			}
			seen[id] = true
		}
	}
	union := specScopeUnion(t, specs)
	if !union["core"] || !union["audit"] || len(union) != 2 {
		t.Errorf("scoped ids = %v, want exactly {core, audit}", union)
	}
}

// TestPlanFromTriage_FloorsTakePrecedenceOverCards (negative/precedence):
// when committed_floors is present and non-empty, the caller's card packages
// are dead weight — they must NOT be merged in. Kills an adapter that unions
// floors+cards and over-schedules lanes triage never committed.
func TestPlanFromTriage_FloorsTakePrecedenceOverCards(t *testing.T) {
	decisionJSON := []byte(`{"committed_floors":["bridge"]}`)
	specs, _, err := PlanFromTriage(decisionJSON, []string{"core", "audit"}, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1 (single floor, cards are fallback-only)", len(specs))
	}
	union := specScopeUnion(t, specs)
	if !union["bridge"] || len(union) != 1 {
		t.Errorf("scoped ids = %v, want exactly {bridge} — card packages must not leak into a floors-derived plan", union)
	}
}

// TestPlanFromTriage_NonPositiveCountNeverPanicsOrOverSchedules (edge/OOD):
// count<=0 is a degenerate wave. Whatever the adapter chooses (explicit
// reject or PlanCycles' documented clamp-to-one-lane), it must never panic,
// never return specs alongside an error, and never spread work across more
// than one lane.
func TestPlanFromTriage_NonPositiveCountNeverPanicsOrOverSchedules(t *testing.T) {
	for _, count := range []int{0, -1} {
		t.Run(fmt.Sprintf("count=%d", count), func(t *testing.T) {
			specs, _, err := PlanFromTriage([]byte(`{"committed_floors":["core","audit"]}`), nil, count, nil)
			if err != nil {
				if len(specs) != 0 {
					t.Errorf("error return carried %d specs, want 0 (never both)", len(specs))
				}
				return
			}
			if len(specs) != 1 {
				t.Fatalf("len(specs) = %d, want 1 (non-positive count clamps to a single lane, per Partition's n<1 contract)", len(specs))
			}
			union := specScopeUnion(t, specs)
			if !union["core"] || !union["audit"] {
				t.Errorf("scoped ids = %v, want both committed floors in the single clamped lane", union)
			}
		})
	}
}

// TestPlanFromTriage_WrongTypeDecisionFieldsRejected (negative): valid JSON
// whose committed_floors is the wrong TYPE is malformed for this schema and
// must reject with zero specs — falling back to cards here would silently
// mask a corrupted triage artifact ("rejects, not guesses").
func TestPlanFromTriage_WrongTypeDecisionFieldsRejected(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"floors-is-a-string", `{"committed_floors":"core"}`},
		{"floors-is-a-number-array", `{"committed_floors":[1,2,3]}`},
		{"document-is-a-bare-number", `42`},
		{"document-is-an-array", `["core","audit"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specs, _, err := PlanFromTriage([]byte(tc.in), []string{"core"}, 2, nil)
			if err == nil {
				t.Fatalf("PlanFromTriage(%s) returned nil error — wrong-typed decision JSON must reject, never guess or fall back", tc.in)
			}
			if len(specs) != 0 {
				t.Errorf("PlanFromTriage(%s) returned %d specs alongside the error, want 0", tc.in, len(specs))
			}
		})
	}
}

// TestPlanFromTriage_DegenerateDecisionBytesFailSafe (edge/OOD): empty bytes
// and a bare JSON null sit between "malformed" and "absent floors". Either
// fail-safe outcome is acceptable — an explicit reject (error + zero specs)
// or a clean fallback to the card packages — but never a panic, never a
// partial/unscoped plan, and never an error that still carries specs.
func TestPlanFromTriage_DegenerateDecisionBytesFailSafe(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"empty-bytes", []byte{}},
		{"nil-bytes", nil},
		{"bare-null", []byte(`null`)},
		{"empty-floors-null", []byte(`{"committed_floors":null}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			specs, _, err := PlanFromTriage(tc.in, []string{"core"}, 2, nil)
			if err != nil {
				if len(specs) != 0 {
					t.Errorf("error return carried %d specs, want 0 (never both)", len(specs))
				}
				return
			}
			if len(specs) != 1 {
				t.Fatalf("len(specs) = %d, want 1 (nil-error path must be the exact single-card fallback plan)", len(specs))
			}
			union := specScopeUnion(t, specs)
			if !union["core"] || len(union) != 1 {
				t.Errorf("scoped ids = %v, want exactly {core} (card fallback)", union)
			}
		})
	}
}

// TestPlanFromTriage_LargeScaleAllFloorsScheduledDisjoint (limit): 100
// distinct floors over count=4 must fill exactly 4 lanes, keep every lane's
// scope pairwise disjoint, and schedule ALL 100 ids — PlanCycles' one-file
// todos can never collide, so nothing may be silently deferred or dropped.
func TestPlanFromTriage_LargeScaleAllFloorsScheduledDisjoint(t *testing.T) {
	floors := make([]string, 100)
	for i := range floors {
		floors[i] = fmt.Sprintf("pkg%03d", i)
	}
	raw, err := json.Marshal(map[string][]string{"committed_floors": floors})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	specs, _, err := PlanFromTriage(raw, nil, 4, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 4 {
		t.Fatalf("len(specs) = %d, want 4 (100 disjoint floors fill every lane)", len(specs))
	}
	union := specScopeUnion(t, specs)
	if len(union) != 100 {
		t.Errorf("scheduled %d distinct ids, want all 100 — no floor may be silently dropped or deferred", len(union))
	}
	for i, spec := range specs {
		if len(spec.Scope) == 0 {
			t.Errorf("spec[%d] has an empty Scope — empty buckets must yield no spec at all", i)
		}
	}
}

// TestPlanFromTriage_PathLikeFloorIDsSurviveVerbatim (edge): floor ids are
// package/path-shaped strings; slashed and non-ASCII ids must survive the
// adapter → PlanCycles round trip without normalization mangling identity.
func TestPlanFromTriage_PathLikeFloorIDsSurviveVerbatim(t *testing.T) {
	floors := []string{"go/internal/fleet", "docs/architecture", "パッケージ"}
	raw, err := json.Marshal(map[string][]string{"committed_floors": floors})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	specs, _, err := PlanFromTriage(raw, nil, 3, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	union := specScopeUnion(t, specs)
	for _, want := range floors {
		if !union[want] {
			t.Errorf("scoped ids = %v, missing floor %q verbatim — ids must not be rewritten", union, want)
		}
	}
	if len(union) != len(floors) {
		t.Errorf("scheduled %d distinct ids, want %d", len(union), len(floors))
	}
}
