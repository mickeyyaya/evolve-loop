package admission_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
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
	want := admission.Decision{ID: "context-fill-telemetry", Rank: 1, Cells: []string{"family:context", "internal/inboxrank"}, BlockedBy: "context-fill-cap", FamilyHold: true}
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

func TestAdmit_AnAuthoredRetroOrAutoItemKeepsItsFamily(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	for _, pair := range [][2]string{{"retro-minting-files-lane", "retro-events-stream-missing"}, {"auto-scaler-fix", "auto-scaler-docs"}} {
		_, waiting := a.Admit(nil, []fleet.Todo{todo(pair[0], "docs/a.md"), todo(pair[1], "docs/b.md")}, 32)
		if len(waiting) != 1 || waiting[0].BlockedBy != pair[0] {
			t.Errorf("waiting = %+v: %s is a filed item, not a kernel-minted one, so it keeps its family", waiting, pair[1])
		}
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

func TestAdmit_FamilyHoldMarksOnlyAHoldTheFamilyRuleMakes(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	held := []admission.Holding{{Lane: "ctx-cap", Todo: todo("ctx-cap", "go/internal/quotastate/windows.go")}}
	_, byFamily := a.Admit(held, []fleet.Todo{todo("ctx-telemetry", "go/internal/inboxrank/inboxrank.go")}, 32)
	_, byPackage := a.Admit(held, []fleet.Todo{todo("other-quota", "go/internal/quotastate/state.go")}, 32)
	_, byBoth := a.Admit(held, []fleet.Todo{todo("ctx-window", "go/internal/quotastate/state.go")}, 32)
	if len(byFamily) != 1 || !byFamily[0].FamilyHold {
		t.Errorf("family-only hold = %+v, want FamilyHold", byFamily)
	}
	if len(byPackage) != 1 || byPackage[0].FamilyHold {
		t.Errorf("package hold = %+v, want no FamilyHold", byPackage)
	}
	if len(byBoth) != 1 || byBoth[0].FamilyHold {
		t.Errorf("package and family hold = %+v, want no FamilyHold: the package cell would hold it anyway", byBoth)
	}
}

func TestAdmit_EveryKernelMinterMintsAnIDWithoutAFamily(t *testing.T) {
	t.Parallel()
	a := admitter(t, nil)
	minted := []string{
		triagecap.NewDemotionLedgerRecord(1921, 1919, 1920, "x", triagecap.RemedyPending).ID,
		triagecap.NewDemotionLedgerRecord(1931, 1929, 1930, "y", triagecap.RemedyPending).ID,
		fmt.Sprintf("retro-%d-%s-%s", 1919, "fix-the-gate", "abc123"),
		fmt.Sprintf("retro-%d-%s-%s", 1920, "name-the-cause", "def456"),
		fmt.Sprintf("cycle-%d-defect-%d", 1919, 1),
		fmt.Sprintf("cycle-%d-defect-%d", 1920, 2),
		"practice-agy-changelog-1a2b3c4d-20261017T100000Z",
		"practice-claude-changelog-5e6f7a8b-20261017T103000Z",
		fmt.Sprintf("auto-retro-%d-%s", 1919, "a1"),
		fmt.Sprintf("auto-retro-%d-%s", 1920, "a2"),
	}
	candidates := make([]fleet.Todo, len(minted))
	for i, id := range minted {
		candidates[i] = todo(id, fmt.Sprintf("docs/%d.md", i))
	}
	if _, waiting := a.Admit(nil, candidates, 64); len(waiting) != 0 {
		t.Fatalf("waiting = %+v: a kernel-minted prefix names no subsystem", waiting)
	}
}

func TestKernelMinters_StillMintTheFormatsTheFamilyRuleExempts(t *testing.T) {
	t.Parallel()
	for file, format := range map[string]string{
		"core/failurelearning/remediation.go": `"retro-%d-%s-%s"`,
		"core/carryover/mint.go":              `"cycle-%d-defect-%d"`,
		"practicewatch/item.go":               `"practice-" + src.ID`,
		"retrofile/retrofile.go":              `"auto-retro-%d-%s.json"`,
		"triagecap/demotion.go":               `"auto-heuristic-demotion-triagecap-c%d-c%d"`,
	} {
		body, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), format) {
			t.Errorf("%s no longer mints %s: update kernelMinted and TestAdmit_EveryKernelMinterMintsAnIDWithoutAFamily with the new form", file, format)
		}
	}
}

func TestAdmit_AFamilyHoldBesideAPackageHoldOfAnotherLaneIsNoFamilyHold(t *testing.T) {
	t.Parallel()
	for _, optimistic := range []bool{false, true} {
		a := admitter(t, nil)
		a.Optimistic = optimistic
		held := []admission.Holding{
			{Lane: "ctx-cap", Todo: todo("ctx-cap", "go/internal/quotastate/windows.go")},
			{Lane: "lane-2", Todo: todo("other", "go/internal/inboxrank/explain.go")},
		}
		_, waiting := a.Admit(held, []fleet.Todo{todo("ctx-telemetry", "go/internal/inboxrank/inboxrank.go")}, 32)
		if len(waiting) != 1 || waiting[0].BlockedBy != "ctx-cap" || waiting[0].FamilyHold {
			t.Errorf("optimistic=%v: waiting = %+v: lane-2 holds the inboxrank cell, so the hold is not the family's alone", optimistic, waiting)
		}
	}
}
