package regressiontia

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestSelect_SkipsOnlyDisjointRegressionPackages(t *testing.T) {
	patterns := []string{
		"./acs/regression/routing",
		"./acs/regression/apicover",
	}
	deps := map[string][]string{
		"./acs/regression/routing":  {"./internal/router/...", "./internal/routingtest/..."},
		"./acs/regression/apicover": {"./internal/apicover/..."},
	}
	scope := []string{"./internal/router/...", "./internal/routingtest/..."}

	selected, wouldSkip := Select(patterns, scope, deps)

	if !reflect.DeepEqual(wouldSkip, []string{"./acs/regression/apicover"}) {
		t.Errorf("wouldSkip = %v, want only ./acs/regression/apicover — the sole package whose deps are disjoint from the changed scope", wouldSkip)
	}
	if !reflect.DeepEqual(selected, []string{"./acs/regression/routing"}) {
		t.Errorf("selected = %v, want [./acs/regression/routing]", selected)
	}

	union := append(append([]string{}, selected...), wouldSkip...)
	sort.Strings(union)
	all := append([]string{}, patterns...)
	sort.Strings(all)
	if !reflect.DeepEqual(union, all) {
		t.Errorf("selected ∪ wouldSkip = %v, want exactly the input %v", union, all)
	}
}

func TestSelect_FailSafes(t *testing.T) {
	patterns := []string{"./acs/regression/routing", "./acs/regression/apicover"}
	deps := map[string][]string{
		"./acs/regression/routing":  {"./internal/router/..."},
		"./acs/regression/apicover": {"./internal/apicover/..."},
	}

	t.Run("empty scope skips nothing", func(t *testing.T) {
		selected, wouldSkip := Select(patterns, nil, deps)
		if len(wouldSkip) != 0 {
			t.Errorf("wouldSkip = %v with an empty changed scope, want empty — an underivable scope means UNKNOWN impact, never zero impact", wouldSkip)
		}
		if !reflect.DeepEqual(selected, patterns) {
			t.Errorf("selected = %v, want the full input %v", selected, patterns)
		}
	})

	t.Run("unknown deps skip nothing", func(t *testing.T) {
		selected, wouldSkip := Select(patterns, []string{"./internal/bridge/..."}, map[string][]string{})
		if len(wouldSkip) != 0 {
			t.Errorf("wouldSkip = %v with no dependency data, want empty — a package whose deps could not be resolved must always run", wouldSkip)
		}
		if !reflect.DeepEqual(selected, patterns) {
			t.Errorf("selected = %v, want the full input %v", selected, patterns)
		}
	})

	t.Run("empty patterns yield empty decision", func(t *testing.T) {
		selected, wouldSkip := Select(nil, []string{"./internal/router/..."}, deps)
		if len(selected) != 0 || len(wouldSkip) != 0 {
			t.Errorf("Select(nil, ...) = (%v, %v), want both empty", selected, wouldSkip)
		}
	})
}

func TestChangedScope_WidensByReverseDependency(t *testing.T) {
	repoRoot := repoRootForTest(t)

	got := ChangedScope(repoRoot, []string{"./internal/router/..."})

	if !containsStr(got, "./internal/router/...") {
		t.Errorf("scope = %v, want the input ./internal/router/... retained — closure only ever widens", got)
	}
	if !containsStr(got, "./internal/routingtest/...") {
		t.Errorf("scope = %v, want ./internal/routingtest/... included — routingtest imports router; without reverse-dependency widening the cycle-1250 miss recurs", got)
	}
}

func TestChangedScope_DegenerateInputs(t *testing.T) {
	if got := ChangedScope(repoRootForTest(t), nil); len(got) != 0 {
		t.Errorf("ChangedScope(repoRoot, nil) = %v, want empty — no changed packages means no scope", got)
	}
	if got := ChangedScope("", []string{"./internal/router/..."}); !reflect.DeepEqual(got, []string{"./internal/router/..."}) {
		t.Errorf("ChangedScope with an empty repoRoot = %v, want the input unchanged (best-effort degrade, never a loss)", got)
	}
}

func TestCompute_OffStageComputesNothing(t *testing.T) {
	mod := moduleWithRegressionDirs(t, "routing", "apicover")
	for _, stage := range []string{"off", "", "bogus"} {
		d := Compute(stage, "", mod, []string{"./internal/router/..."})
		if !reflect.DeepEqual(d, Decision{}) {
			t.Errorf("Compute(stage=%q) = %+v, want the zero Decision — a dormant stage must compute nothing at all", stage, d)
		}
	}
}

func TestCompute_ShadowEnumeratesRegressionCorpus(t *testing.T) {
	mod := moduleWithRegressionDirs(t, "apicover", "routing")

	d := Compute("shadow", "", mod, nil)

	if d.Stage != "shadow" {
		t.Errorf("Stage = %q, want \"shadow\"", d.Stage)
	}
	want := []string{"./acs/regression/apicover", "./acs/regression/routing"}
	if !reflect.DeepEqual(d.Selected, want) {
		t.Errorf("Selected = %v, want the whole discovered corpus %v when the changed scope is underivable", d.Selected, want)
	}
	if len(d.WouldSkip) != 0 || d.WouldSkipCount != 0 {
		t.Errorf("WouldSkip = %v (count %d), want empty — an underivable scope must never mark a predicate skippable", d.WouldSkip, d.WouldSkipCount)
	}
	if d.WouldSkipCount != len(d.WouldSkip) {
		t.Errorf("WouldSkipCount = %d but len(WouldSkip) = %d — the count must be a projection of the list, never an independent tally", d.WouldSkipCount, len(d.WouldSkip))
	}
}

func TestEmit_WritesArtifactAndRejectsEmptyWorkspace(t *testing.T) {
	ws := t.TempDir()
	d := Decision{
		Stage:           "shadow",
		ChangedPackages: []string{"./internal/router/..."},
		Selected:        []string{"./acs/regression/routing"},
		WouldSkip:       []string{"./acs/regression/apicover"},
		WouldSkipCount:  1,
	}

	path, err := Emit(ws, d)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if want := filepath.Join(ws, ArtifactName); path != want {
		t.Errorf("Emit returned %q, want %q", path, want)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read emitted artifact: %v", err)
	}
	var back Decision
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("emitted artifact is not valid JSON: %v", err)
	}
	if !reflect.DeepEqual(back, d) {
		t.Errorf("round-tripped decision = %+v, want %+v — the artifact is the operator's only view of what selection WOULD have done", back, d)
	}

	if _, err := Emit("", d); err == nil {
		t.Error("Emit(\"\", d) returned nil error — a missing workspace must fail loudly, not silently drop the evidence")
	}
}

func moduleWithRegressionDirs(t *testing.T, subs ...string) string {
	t.Helper()
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, s := range subs {
		if err := os.MkdirAll(filepath.Join(mod, "acs", "regression", s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return mod
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestChangedScope_SelectsTestOnlyImporters(t *testing.T) {
	got := ChangedScope(repoRootForTest(t), []string{"./internal/core/..."})
	for _, p := range got {
		if p == "./internal/routingeval/..." {
			return
		}
	}
	t.Errorf("ChangedScope(./internal/core/...) omits the test-only importer ./internal/routingeval/...; got %d patterns", len(got))
}
