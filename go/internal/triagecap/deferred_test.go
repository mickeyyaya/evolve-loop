package triagecap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDeferredFloorPackages_Cycle281Replay(t *testing.T) {
	// The fixture defers cmd/evolve (basename "evolve"); its bridge item names bridge only inside hyphenated slugs.
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
			name: "deferred metadata stripped including full defer_reason prose",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n" +
				"- coverage-rest: push swarm coverage to ≥98% — priority=M, evidence=scout-report.md#x, defer_reason=budget consumed by bridge work, source=scout\n",
			want: []string{"swarm"},
		},
		{
			name: "genuine bridge floor in task prose still detected",
			artifact: "## top_n\n- fix: a bug fix\n\n" +
				"## deferred\n" +
				"- coverage-bridge: push bridge coverage to ≥98% — priority=M, defer_reason=budget consumed elsewhere\n",
			want: []string{"bridge"},
		},
		{
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

// writeDeferredCompanion omits deferred_floors for a nil slice and writes it for a non-nil, possibly empty, one.
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
		path := writeDeferredCompanion(t, dir, nil)
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
		path := filepath.Join(dir, triageDecisionFile)
		if err := os.WriteFile(path, []byte(`{"deferred_floors":"core"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ReadDeferredFloors(path); err == nil {
			t.Error("malformed deferred_floors (non-array) must return an error")
		}
	})
}

func TestDeferredFloorPackagesDecl_DeclarationPrimary(t *testing.T) {
	dir := t.TempDir()
	path := writeDeferredCompanion(t, dir, []string{"core"})
	artifact := "## top_n\n- coverage-core: core coverage ≥98%\n\n" +
		"## deferred\n- coverage-bridge: push bridge coverage to ≥98%\n"
	got := DeferredFloorPackagesDecl(artifact, path, []string{"core", "bridge"})
	if want := []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("declaration-primary deferred = %v, want %v (declaration must win over prose)", got, want)
	}
}

func TestDeferredFloorPackagesDecl_FallbackToProse(t *testing.T) {
	artifact := "## top_n\n- fix: a bug fix\n\n" +
		"## deferred\n- coverage-rest: recovery, interaction coverage to ≥98%\n"
	noCompanion := filepath.Join(t.TempDir(), "absent.json")
	got := DeferredFloorPackagesDecl(artifact, noCompanion, knownPkgsFixture)
	want := DeferredFloorPackages(artifact, knownPkgsFixture)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no companion: got %v, want prose result %v", got, want)
	}
	if len(want) == 0 {
		t.Fatal("fixture sanity: prose path should have found deferred packages")
	}
}

func TestDeferredFloorPackagesDecl_CompanionNoFieldFallsBack(t *testing.T) {
	dir := t.TempDir()
	path := writeDeferredCompanion(t, dir, nil)
	artifact := "## top_n\n- fix: a bug fix\n\n" +
		"## deferred\n- coverage-core: push core coverage to ≥98%\n"
	got := DeferredFloorPackagesDecl(artifact, path, []string{"core"})
	if want := []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("companion without deferred_floors must fall back to prose; got %v want %v", got, want)
	}
}

func TestDeferredFloorPackagesDecl_FiltersToCandidates(t *testing.T) {
	dir := t.TempDir()
	path := writeDeferredCompanion(t, dir, []string{"core", "ghostpkg"})
	got := DeferredFloorPackagesDecl("## top_n\n- x: y\n", path, []string{"core"})
	if want := []string{"core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("declared-but-untargeted package must be filtered out; got %v want %v", got, want)
	}
}

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
