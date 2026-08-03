package regressiontia

// regressiontia_test.go — RED contract for cycle-1260 Task 1
// (`egps-regression-tia-shadow-wiring`, inbox item
// .evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json,
// P1 weight 0.91, 3rd live instance).
//
// WHY THIS PACKAGE EXISTS (deviation from scout-report's target files, recorded
// loudly per Core Rule 3). Scout targeted go/internal/acssuite/acssuite.go. That
// path is PROTECTED CONTROL PLANE — guards.ProtectedSurfaceManifest carries
// {Fragment: "/go/internal/acssuite/", Rationale: "the gate runner"} — so no
// phase of any cycle may write it (the role guard denies and ALARMS; verified
// live this phase). A cycle may not edit the gate that grades it.
//
// The boundary is not an obstacle to route around, it is a design constraint,
// and honoring it yields the better architecture: the SHADOW stage changes
// nothing about what the gate runs (that is the definition of shadow), so it
// needs no code in the gate runner at all. It is pure observability, computed
// beside the suite by the suite's own production CALLER — the audit phase's
// generateACSVerdict (internal/phases/audit/audit.go:651). Only the future
// `enforce` stage, which actually skips packages, must live inside acssuite,
// and that change is human-gated `evolve ship --class manual` OUTSIDE a cycle
// by construction. Scout's Task-1 acceptance intent is preserved verbatim:
// off/absent is byte-identical, shadow logs would-skip counts, nothing is
// skipped yet.
//
// The contract these tests freeze:
//
//	const ArtifactName = "acs-tia-shadow.json"
//	type Decision struct{ Stage; ChangedPackages; Selected; WouldSkip; WouldSkipCount }
//	func Select(patterns, scope []string, deps map[string][]string) (selected, wouldSkip []string)
//	func ChangedScope(repoRoot string, changed []string) []string
//	func Compute(stage, repoRoot, moduleDir string, changed []string) Decision
//	func Emit(workspace string, d Decision) (string, error)
//
// Safety invariant carried from the inbox item — selection must never be able
// to hide a regression class — so every fail-safe resolves toward RUNNING a
// predicate: unknown scope ⇒ skip nothing, unknown deps ⇒ skip nothing,
// unknown stage ⇒ off.
//
// RED today: package regressiontia does not exist, so this file fails to
// COMPILE — a hard non-zero exit, never a silent pass.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestSelect_SkipsOnlyDisjointRegressionPackages pins the core semantics: a
// regression sub-package is a skip candidate ONLY when its dependency set is
// known and provably disjoint from the changed-package scope.
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

	// Partition invariant: selected ⊎ wouldSkip == patterns. Selection may
	// reorder, never lose or duplicate a package.
	union := append(append([]string{}, selected...), wouldSkip...)
	sort.Strings(union)
	all := append([]string{}, patterns...)
	sort.Strings(all)
	if !reflect.DeepEqual(union, all) {
		t.Errorf("selected ∪ wouldSkip = %v, want exactly the input %v", union, all)
	}
}

// TestSelect_FailSafes is the NEGATIVE axis and the anti-no-op guard: every
// unknown must resolve toward RUNNING the predicate. An implementation that
// eagerly marks packages skippable — the exact failure mode this item exists
// to prevent — fails here, and so does a stub that returns everything as
// would-skip.
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
		// No dependency data at all: impact is unresolvable for every package.
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

// TestChangedScope_WidensByReverseDependency is the ImporterClosure wiring
// proof and the cycle-1250 reproducer. A change confined to internal/router
// must widen to internal/routingtest, which imports it (agent.go/bricks.go/
// engine.go) and holds the keystone parity invariant. Forward-only scope marks
// the routing regression package skippable and hides exactly the class that
// kept main red for 5 commits. Run against the REAL repository import graph,
// so it fails if ChangedScope forwards its input instead of calling
// changedpkgs.ImporterClosure.
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

// TestChangedScope_DegenerateInputs pins the best-effort edges: never a panic,
// never a lost input, never a widened everything from nothing.
func TestChangedScope_DegenerateInputs(t *testing.T) {
	if got := ChangedScope(repoRootForTest(t), nil); len(got) != 0 {
		t.Errorf("ChangedScope(repoRoot, nil) = %v, want empty — no changed packages means no scope", got)
	}
	if got := ChangedScope("", []string{"./internal/router/..."}); !reflect.DeepEqual(got, []string{"./internal/router/..."}) {
		t.Errorf("ChangedScope with an empty repoRoot = %v, want the input unchanged (best-effort degrade, never a loss)", got)
	}
}

// TestCompute_OffStageComputesNothing pins the production default. The
// checked-in .evolve/policy.json carries no regression_tia block, so the
// resolved stage is "off" and Compute must be inert — an empty Decision that
// Emit is never asked to write.
func TestCompute_OffStageComputesNothing(t *testing.T) {
	mod := moduleWithRegressionDirs(t, "routing", "apicover")
	for _, stage := range []string{"off", "", "bogus"} {
		d := Compute(stage, "", mod, []string{"./internal/router/..."})
		if !reflect.DeepEqual(d, Decision{}) {
			t.Errorf("Compute(stage=%q) = %+v, want the zero Decision — a dormant stage must compute nothing at all", stage, d)
		}
	}
}

// TestCompute_ShadowEnumeratesRegressionCorpus is the shadow-stage crux: with
// an underivable changed set, every discovered regression sub-package is
// SELECTED (nothing skippable), and the decision names the corpus it reasoned
// about — the would-skip evidence this staged rollout exists to collect.
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

// TestEmit_WritesArtifactAndRejectsEmptyWorkspace pins the artifact contract:
// the decision lands at <workspace>/acs-tia-shadow.json as readable JSON, and
// a missing workspace is a loud error, never a silent no-write.
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

// moduleWithRegressionDirs builds a throwaway Go module dir holding the named
// acs/regression/<sub> directories, so corpus enumeration is exercised against
// a real filesystem without depending on the repo's live corpus.
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

// repoRootForTest resolves the repository root from this test's own location
// (go/internal/regressiontia → ../../..). Derived, not hardcoded, so it is
// correct in the main tree and in every fleet worktree alike.
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
