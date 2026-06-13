package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// R9.3: the deferred/dropped floor vocabulary — packages whose coverage
// floors triage explicitly pushed OUT of this cycle. TDD predicates binding
// these floors is the cycle-280 failure mode (builder starved the committed
// task while clearing deferred-task gates).

func TestDeferredFloorPackages_Cycle281Replay(t *testing.T) {
	// cycle-281 deferred floor items name cmd/evolve ("evolve" is the package
	// basename); the bridge item's only package reference is inside hyphenated
	// slug compounds, which are single tokens — not mentions.
	pkgs := append([]string{"evolve"}, knownPkgsFixture...)
	got := DeferredFloorPackages(readFixture(t, "triage-cycle281.md"), pkgs)
	want := []string{"evolve"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeferredFloorPackages(cycle-281) = %v, want %v", got, want)
	}
}

func TestDeferredFloorPackages_Table(t *testing.T) {
	tests := []struct {
		name     string
		artifact string
		want     []string
	}{
		{
			name:     "no deferred section",
			artifact: "## top_n\n- coverage-x: bridge coverage ≥98%\n",
			want:     nil,
		},
		{
			name: "deferred floor item names packages",
			artifact: "## top_n\n- coverage-x: bridge coverage ≥98%\n\n" +
				"## deferred (carry to NEXT cycle's carryoverTodos)\n" +
				"- coverage-rest: recovery, interaction coverage to ≥98%\n",
			want: []string{"interaction", "recovery"},
		},
		{
			name: "dropped floor item counts too",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## dropped (rejected with reason)\n" +
				"- coverage-no: evalgate to 95% coverage — reason=descoped\n",
			want: []string{"evalgate"},
		},
		{
			name: "non-floor deferred items contribute nothing",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n- refactor-later: tidy the recovery package\n",
			want: nil,
		},
		{
			// Pins the metadata-strip semantics on deferred items: the
			// contract fields' own vocabulary never counts, and the ENTIRE
			// defer_reason value (to end of line) is stripped — defer
			// reasons are scheduling prose that routinely references OTHER
			// work. Cycle 310 (soak #3d) proved the earlier tail-matchable
			// reading wrong: "co-scheduling with the looppreflight blocker
			// fix" made Gate C block the COMMITTED package's predicates.
			name: "deferred metadata stripped including full defer_reason prose",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n" +
				"- coverage-rest: push swarm coverage to ≥98% — priority=M, evidence=scout-report.md#x, defer_reason=budget consumed by bridge work, source=scout\n",
			want: []string{"swarm"},
		},
		{
			// Counterpart to the strip pin: bridge as the item's ACTUAL
			// floor package (in the task prose, not defer_reason) is still
			// detected after the defer_reason strip.
			name: "genuine bridge floor in task prose still detected",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n" +
				"- coverage-bridge: push bridge coverage to ≥98% — priority=M, defer_reason=budget consumed elsewhere\n",
			want: []string{"bridge"},
		},
		{
			// Cycle-310 verbatim replay (soak #3d): the deferred ledger-seal
			// item's defer_reason references the committed looppreflight
			// blocker — that mention must NOT make looppreflight a deferred
			// floor package (it blocked the committed task's own predicates).
			name: "cycle-310 replay: defer_reason referencing committed work does not count",
			artifact: "## top_n\n" +
				"- looppreflight-env-seams: Convert defaultTmuxSessions to var-seam; add deterministic branch tests; cover saveVersionCache write-error path — priority=H, evidence=scout-report §Task 3, source=scout\n\n" +
				"## deferred\n" +
				"- ledger-seal-io-coverage: Roundtrip tests for writeSegment/rewriteLive/readSegment; lift floor to ≥ 85% — priority=H, defer_reason=same as above; co-scheduling with the looppreflight blocker fix risks repeating the phantom-floor capacity failure\n",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeferredFloorPackages(tt.artifact, knownPkgsFixture)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeferredFloorPackages = %v, want %v", got, tt.want)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// ADR-0046 Layer 1 (cycle 305): declaration-primary deferred floors.
//
// New API this file pins (Builder implements in deferred.go, mirroring the
// shipped committed-floor path in floors.go):
//
//   - ReadDeferredFloors(companionPath) ([]string, bool, error)
//       reads triage-decision.json's deferred_floors[]; missing file / missing
//       field is NOT an error (returns nil,false,nil → caller falls back to
//       prose), parallel to ReadDeclaredFloors.
//   - DeferredFloorPackagesDecl(artifact, companionPath, candidatePkgs) []string
//       declaration-PRIMARY: when deferred_floors[] is present, the declared
//       packages (filtered to candidatePkgs, sorted, distinct) are authoritative
//       and prose is ignored; otherwise it falls back to prose DeferredFloorPackages.
//   - DeferredFloorDivergence(artifact, companionPath, knownPkgs) string
//       the deferred analog of FloorDivergenceCorrective: an actionable, non-empty
//       message when prose-deferred packages and deferred_floors[] disagree; ""
//       when they agree or no declaration exists. The triage-floors guard prints it.
//
// These are RED until Builder adds the three functions: the test package
// fails to compile against the absent symbols.
// ----------------------------------------------------------------------------

// writeDeferredCompanion writes a triage-decision.json companion into dir.
// A nil deferredFloors writes a companion WITHOUT the deferred_floors field
// (the "present file, absent field" fallback case); a non-nil (possibly empty)
// slice writes the field. committedFloors, when non-nil, is written too so the
// divergence/fallback tests can exercise a realistic two-field companion.
func writeDeferredCompanion(t *testing.T, dir string, deferredFloors []string) string {
	t.Helper()
	quote := func(xs []string) string {
		q := make([]string, len(xs))
		for i, x := range xs {
			q[i] = `"` + x + `"`
		}
		return "[" + joinComma(q) + "]"
	}
	var body string
	if deferredFloors == nil {
		body = `{"cycle":305,"top_n":[]}`
	} else {
		body = `{"cycle":305,"deferred_floors":` + quote(deferredFloors) + `}`
	}
	path := filepath.Join(dir, triageDecisionFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

// TestReadDeferredFloors pins the declaration reader's missing-file /
// missing-field fail-open contract and its happy path — exactly mirroring
// ReadDeclaredFloors (floors.go:85) so the two floor kinds share one shape.
func TestReadDeferredFloors(t *testing.T) {
	t.Run("present field returns floors", func(t *testing.T) {
		dir := t.TempDir()
		path := writeDeferredCompanion(t, dir, []string{"core", "bridge"})
		got, ok, err := ReadDeferredFloors(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("ok=false; deferred_floors was present so ok must be true")
		}
		if want := []string{"core", "bridge"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("missing file is not an error", func(t *testing.T) {
		got, ok, err := ReadDeferredFloors(filepath.Join(t.TempDir(), "nope.json"))
		if err != nil {
			t.Fatalf("missing companion must fail open, got error: %v", err)
		}
		if ok || got != nil {
			t.Errorf("missing companion: got (%v, ok=%v), want (nil, false)", got, ok)
		}
	})

	t.Run("present file without deferred_floors field falls through", func(t *testing.T) {
		dir := t.TempDir()
		path := writeDeferredCompanion(t, dir, nil) // companion, but no deferred_floors
		got, ok, err := ReadDeferredFloors(path)
		if err != nil {
			t.Fatalf("absent field must not error, got: %v", err)
		}
		if ok || got != nil {
			t.Errorf("absent field: got (%v, ok=%v), want (nil, false)", got, ok)
		}
	})

	t.Run("malformed deferred_floors is an error", func(t *testing.T) {
		dir := t.TempDir()
		// deferred_floors as a string, not an array → a genuine schema error
		// the caller must surface (not silently fail open).
		path := filepath.Join(dir, triageDecisionFile)
		if err := os.WriteFile(path, []byte(`{"deferred_floors":"core"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadDeferredFloors(path); err == nil {
			t.Error("malformed deferred_floors (non-array) must return an error")
		}
	})
}

// TestDeferredFloorPackagesDecl_DeclarationPrimary: when the companion declares
// deferred_floors, those packages are authoritative and contradictory prose is
// ignored. The declared set is filtered to the caller's candidate packages
// (the floor-predicate targets) so a declaration can only ever block a package
// some predicate actually binds.
func TestDeferredFloorPackagesDecl_DeclarationPrimary(t *testing.T) {
	dir := t.TempDir()
	path := writeDeferredCompanion(t, dir, []string{"core"})
	// Prose disagrees: it defers `bridge`, not `core`. Declaration must win.
	artifact := "## top_n\n- coverage-core: core coverage ≥98%\n\n" +
		"## deferred\n- coverage-bridge: push bridge coverage to ≥98%\n"
	got := DeferredFloorPackagesDecl(artifact, path, []string{"core", "bridge"})
	if want := []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("declaration-primary deferred = %v, want %v (declaration must win over prose)", got, want)
	}
}

// TestDeferredFloorPackagesDecl_FallbackToProse: with no companion present, the
// wrapper degrades to the legacy prose scanner — preserving the shipped R9.3
// behavior for older artifacts.
func TestDeferredFloorPackagesDecl_FallbackToProse(t *testing.T) {
	artifact := "## top_n\n- fix: a bug fix\n\n" +
		"## deferred\n- coverage-rest: recovery, interaction coverage to ≥98%\n"
	noCompanion := filepath.Join(t.TempDir(), "absent.json")
	got := DeferredFloorPackagesDecl(artifact, noCompanion, knownPkgsFixture)
	want := DeferredFloorPackages(artifact, knownPkgsFixture) // [interaction recovery]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no companion: got %v, want prose result %v", got, want)
	}
	if len(want) == 0 {
		t.Fatal("fixture sanity: prose path should have found deferred packages")
	}
}

// TestDeferredFloorPackagesDecl_CompanionNoFieldFallsBack: a companion that
// exists but omits deferred_floors must still fall back to prose (the field is
// optional; its absence is not "zero deferred floors").
func TestDeferredFloorPackagesDecl_CompanionNoFieldFallsBack(t *testing.T) {
	dir := t.TempDir()
	path := writeDeferredCompanion(t, dir, nil) // present file, no deferred_floors
	artifact := "## top_n\n- fix: a bug fix\n\n" +
		"## deferred\n- coverage-core: push core coverage to ≥98%\n"
	got := DeferredFloorPackagesDecl(artifact, path, []string{"core"})
	if want := []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("companion without deferred_floors must fall back to prose; got %v want %v", got, want)
	}
}

// TestDeferredFloorPackagesDecl_FiltersToCandidates: a declared package that no
// floor predicate targets must NOT appear in the result — declarations bound
// the gate, they do not invent new bindings.
func TestDeferredFloorPackagesDecl_FiltersToCandidates(t *testing.T) {
	dir := t.TempDir()
	path := writeDeferredCompanion(t, dir, []string{"core", "ghostpkg"})
	got := DeferredFloorPackagesDecl("## top_n\n- x: y\n", path, []string{"core"})
	if want := []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("declared-but-untargeted package must be filtered out; got %v want %v", got, want)
	}
}

// TestDeferredFloorDivergence pins the guard's reporting helper: a non-empty,
// actionable message when prose-deferred packages and the declaration disagree,
// "" when they agree or no declaration exists. Mirrors FloorDivergenceCorrective.
func TestDeferredFloorDivergence(t *testing.T) {
	t.Run("agreement is silent", func(t *testing.T) {
		dir := t.TempDir()
		path := writeDeferredCompanion(t, dir, []string{"core"})
		artifact := "## top_n\n- x: y\n\n## deferred\n- coverage-core: core coverage ≥98%\n"
		if msg := DeferredFloorDivergence(artifact, path, knownPkgsFixture); msg != "" {
			t.Errorf("matching prose/declaration must be silent, got %q", msg)
		}
	})

	t.Run("divergence is reported", func(t *testing.T) {
		dir := t.TempDir()
		path := writeDeferredCompanion(t, dir, []string{"core"})
		// Prose defers bridge; declaration defers core → divergence.
		artifact := "## top_n\n- x: y\n\n## deferred\n- coverage-bridge: bridge coverage ≥98%\n"
		msg := DeferredFloorDivergence(artifact, path, knownPkgsFixture)
		if msg == "" {
			t.Fatal("prose/declaration divergence must produce a non-empty corrective message")
		}
	})

	t.Run("no declaration is silent", func(t *testing.T) {
		artifact := "## top_n\n- x: y\n\n## deferred\n- coverage-core: core coverage ≥98%\n"
		noCompanion := filepath.Join(t.TempDir(), "absent.json")
		if msg := DeferredFloorDivergence(artifact, noCompanion, knownPkgsFixture); msg != "" {
			t.Errorf("no declaration → nothing to diverge from; want \"\", got %q", msg)
		}
	})
}
