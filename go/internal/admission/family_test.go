package admission_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestAdmit_TwoTodosOfOneIDFamilyWaitForEachOther(t *testing.T) {
	for _, optimistic := range []bool{false, true} {
		t.Run(fmt.Sprintf("optimistic=%v", optimistic), func(t *testing.T) {
			admitTwoSiblings(t, optimistic)
		})
	}
}

func admitTwoSiblings(t *testing.T, optimistic bool) {
	t.Helper()
	a := admitter(t, nil)
	a.Optimistic = optimistic
	candidates := []fleet.Todo{
		todo("context-fill-cap", "go/internal/quotastate/windows.go"),
		todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go"),
	}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if got := ids(admitted); !reflect.DeepEqual(got, []string{"context-fill-cap"}) {
		t.Fatalf("admitted = %v, want only context-fill-cap: its sibling shares the family", got)
	}
	want := admission.Decision{ID: "context-fill-telemetry", Rank: 1, Cells: []string{"family:context", "internal/inboxrank"}, BlockedBy: "context-fill-cap"}
	if len(waiting) != 1 || !reflect.DeepEqual(waiting[0], want) {
		t.Fatalf("waiting = %+v, want [%+v]", waiting, want)
	}
}

func TestAdmit_ALaneInFlightHoldsItsIDFamily(t *testing.T) {
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "lane-1", Todo: todo("tokenopt-cache-discipline-audit", "go/internal/quotastate/windows.go")}}
	admitted, waiting := a.Admit(held, []fleet.Todo{todo("tokenopt-session-resume-on-retry", "go/internal/inboxrank/inboxrank.go")}, 32)
	if len(admitted) != 0 || len(waiting) != 1 || waiting[0].BlockedBy != "lane-1" {
		t.Fatalf("admitted = %v, waiting = %+v: the tokenopt lane in flight holds the tokenopt family", ids(admitted), waiting)
	}
}

func TestAdmit_TodosOfDifferentIDFamiliesAreUnaffected(t *testing.T) {
	a := admitter(t, nil)
	candidates := []fleet.Todo{
		todo("tokenopt-cache-discipline-audit", "go/internal/quotastate/windows.go"),
		todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go"),
		todo("solo", "docs/a.md"),
		todo("-lone", "docs/b.md"),
		todo("-stray", "docs/c.md"),
	}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if got := ids(admitted); len(waiting) != 0 || !reflect.DeepEqual(got, []string{"tokenopt-cache-discipline-audit", "context-fill-telemetry", "solo", "-lone", "-stray"}) {
		t.Fatalf("admitted = %v, waiting = %+v: disjoint families and ids without a family run together", got, waiting)
	}
	for _, d := range admitted[2:] {
		if len(d.Cells) != 0 {
			t.Errorf("%s has no family token, cells = %v, want none", d.ID, d.Cells)
		}
	}
}

func TestAdmit_AFamilyHoldIsNoReachShadow(t *testing.T) {
	for _, optimistic := range []bool{false, true} {
		calls := 0
		a := admitter(t, nil)
		a.Optimistic = optimistic
		a.OnShadow = func(admission.ShadowAdmission) { calls++ }
		a.OnOptimistic = func(admission.ShadowAdmission) { calls++ }
		held := []admission.Holding{{Lane: "lane-1", Todo: todo("context-fill-cap", "go/internal/quotastate/windows.go")}}
		_, waiting := a.Admit(held, []fleet.Todo{todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go")}, 32)
		if len(waiting) != 1 || calls != 0 {
			t.Fatalf("optimistic=%v: waiting = %+v, sink calls = %d, want the family hold with no reach record", optimistic, waiting, calls)
		}
	}
}

func TestAdmit_KernelMintedAndPackagePathIDsHaveNoFamily(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	candidates := []fleet.Todo{
		todo("practice-agy-changelog-1", "docs/a.md"),
		todo("practice-claude-changelog-2", "docs/b.md"),
		todo("retro-1919-fix-the-gate", "docs/c.md"),
		todo("retro-1920-name-the-cause", "docs/d.md"),
		todo("cycle-1919-defect-1", "docs/e.md"),
		todo("cycle-1920-defect-2", "docs/f.md"),
		todo("go/internal/foo-bar", "docs/g.md"),
		todo("go/internal/foo-baz", "docs/h.md"),
	}
	admitted, waiting := a.Admit(nil, candidates, 32)
	if len(waiting) != 0 || len(admitted) != len(candidates) {
		t.Fatalf("waiting = %+v: kernel-minted ids and package paths name no subsystem, so none holds another", waiting)
	}
}

func TestAdmit_AnAuthoredRetroOrCycleItemKeepsItsFamily(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	_, waiting := a.Admit(nil, []fleet.Todo{todo("retro-minting-files-lane", "docs/a.md"), todo("retro-events-stream-missing", "docs/b.md")}, 32)
	if len(waiting) != 1 || waiting[0].BlockedBy != "retro-minting-files-lane" {
		t.Fatalf("waiting = %+v: a filed retro-* item without a cycle number is an authored family", waiting)
	}
}

func TestAdmit_OnWaitSeesEachHeldDecision(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	var seen []admission.Decision
	a.OnWait = func(d admission.Decision) { seen = append(seen, d) }
	_, waiting := a.Admit(nil, []fleet.Todo{todo("context-fill-cap", "go/internal/quotastate/windows.go"), todo("context-fill-telemetry", "go/internal/inboxrank/inboxrank.go")}, 32)
	if !reflect.DeepEqual(seen, waiting) || len(seen) != 1 {
		t.Fatalf("OnWait saw %+v, want the waiting decisions %+v", seen, waiting)
	}
	if !strings.HasPrefix(seen[0].Cells[0], admission.FamilyCellPrefix) {
		t.Errorf("cells = %v, want the family cell first", seen[0].Cells)
	}
}
