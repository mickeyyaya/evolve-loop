package fleet

import (
	"encoding/json"
	"fmt"
	"testing"
)

// specScopeUnion collects every Env-scoped todo id and fails the test on an id owned by two specs.
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
