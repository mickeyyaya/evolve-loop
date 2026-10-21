package regressiontia

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeCycle(t *testing.T, runs string, cycle string, tia any, verdict any) {
	t.Helper()
	dir := filepath.Join(runs, "cycle-"+cycle)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{ArtifactName: tia, "acs-verdict.json": verdict} {
		if v == nil {
			continue
		}
		raw, ok := v.(string)
		if !ok {
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			raw = string(b)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func red(predicate string, regression bool) map[string]any {
	return map[string]any{"predicate": predicate, "result": "red", "is_regression": regression}
}

func TestRollup_CountsTheShadowDecisionsAndNamesEachMiss(t *testing.T) {
	t.Parallel()
	runs := t.TempDir()
	writeCycle(t, runs, "10", Decision{Stage: "shadow", Selected: []string{"./acs/regression/a"}, WouldSkip: []string{"./acs/regression/b", "./acs/regression/c", "./acs/regression/d"}, WouldSkipCount: 3, DurationMS: 40},
		map[string]any{"results": []any{red("go/acs/b/...:TestB", true), red("go/acs/a/...:TestA", true), red("go/acs/cycle10/...:TestX", false), red("go/acs/d/...:TestD", false), red("go/acs/c:TestC", true),
			map[string]any{"predicate": "go/acs/c/...:TestC2", "result": "green", "is_regression": true}}})
	writeCycle(t, runs, "11", Decision{Stage: "shadow", Selected: []string{"./acs/regression/a", "./acs/regression/b"}, DurationMS: 90}, map[string]any{"results": []any{}})
	writeCycle(t, runs, "12", Decision{Stage: "shadow", WouldSkip: []string{"./acs/regression/a"}, WouldSkipCount: 1}, nil)
	writeCycle(t, runs, "13", nil, map[string]any{"results": []any{}})
	writeCycle(t, runs, "14", "{broken", nil)
	writeCycle(t, runs, "15", Decision{Stage: "shadow", Selected: []string{"./acs/regression/a"}}, "{broken")

	if err := os.MkdirAll(filepath.Join(runs, "cycle-x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runs, "cycle-16"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Rollup(runs)

	if err != nil {
		t.Fatal(err)
	}
	want := Summary{
		Cycles: 4, Selected: 4, WouldSkip: 4, TotalMS: 130, MaxMS: 90, NoVerdict: []int{12},
		Misses: []Miss{{Cycle: 10, Package: "./acs/regression/b", Predicate: "go/acs/b/...:TestB"},
			{Cycle: 10, Package: "./acs/regression/c", Predicate: "go/acs/c:TestC"}},
		Unreadable: []string{"cycle-14/" + ArtifactName, "cycle-15/acs-verdict.json"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
}

func realVerdict(t *testing.T, redPredicate string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cycle-2027-acs-verdict.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	for _, r := range v["results"].([]any) {
		if row := r.(map[string]any); row["predicate"] == redPredicate {
			row["result"] = "red"
		}
	}
	return v
}

func TestRollup_ARealCycleVerdictNamesTheMissOfASkippedRegressionPackage(t *testing.T) {
	t.Parallel()
	const predicate = "go/acs/acsnames/...:TestPredicateNamesResolveToTests"
	decision := Decision{Stage: "shadow", Selected: []string{"./acs/regression/handoff"}, WouldSkip: []string{"./acs/regression/acsnames"}, WouldSkipCount: 1}
	green, missed := t.TempDir(), t.TempDir()
	writeCycle(t, green, "2027", decision, realVerdict(t, ""))
	writeCycle(t, missed, "2027", decision, realVerdict(t, predicate))

	clean, err := Rollup(green)
	if err != nil || len(clean.Misses) != 0 || len(clean.Unreadable) != 0 {
		t.Fatalf("the real all-green verdict of cycle 2027 = (%+v, %v), want no miss", clean, err)
	}
	got, err := Rollup(missed)
	want := []Miss{{Cycle: 2027, Package: "./acs/regression/acsnames", Predicate: predicate}}
	if err != nil || !reflect.DeepEqual(got.Misses, want) {
		t.Fatalf("misses = %+v (err %v), want %+v: acssuite writes go/acs/<base>/...:<Test> for a regression predicate", got.Misses, err, want)
	}
}

func TestRollup_AMissingRunsDirIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := Rollup(filepath.Join(t.TempDir(), "no-runs")); err == nil {
		t.Fatal("a runs dir that does not exist must be an error, not an empty soak")
	}
}

func TestSkippedPackage_APredicateWithNoBaseMatchesNothing(t *testing.T) {
	t.Parallel()
	for _, predicate := range []string{"go/acs/", ":TestX", ""} {
		if pkg, ok := skippedPackage([]string{"./acs/regression/a", "."}, predicate); ok {
			t.Errorf("skippedPackage(%q) = %q: a predicate with no base names no package", predicate, pkg)
		}
	}
}
