package baseattr

import (
	"reflect"
	"testing"
)

func TestParseGoTest_KeysEachFailByPackageAndMarksAPanicUnnamed(t *testing.T) {
	t.Parallel()
	const pkgA, pkgB = "example.com/m/a", "example.com/m/b"
	for _, tc := range []struct {
		name string
		out  string
		want Reds
	}{
		{"empty output", "", Reds{}},
		{"a pass", "ok  \t" + pkgA + "\t0.01s\n", Reds{}},
		{"one test", "--- FAIL: TestX (0.00s)\nFAIL\nFAIL\t" + pkgA + "\t0.01s\n", Reds{Keys: []Key{{pkgA, "TestX"}}}},
		{"a subtest maps to its top-level test", "    --- FAIL: TestX/sub (0.00s)\n--- FAIL: TestX (0.00s)\nFAIL\t" + pkgA + "\t0.01s\n", Reds{Keys: []Key{{pkgA, "TestX"}}}},
		{"two packages", "--- FAIL: TestX (0s)\nFAIL\t" + pkgA + "\t0.1s\n--- FAIL: TestY (0s)\nFAIL\t" + pkgB + "\t0.1s\n", Reds{Keys: []Key{{pkgA, "TestX"}, {pkgB, "TestY"}}}},
		{"the same test name in two packages", "--- FAIL: TestX (0s)\nFAIL\t" + pkgA + "\t0.1s\n--- FAIL: TestX (0s)\nFAIL\t" + pkgB + "\t0.1s\n", Reds{Keys: []Key{{pkgA, "TestX"}, {pkgB, "TestX"}}}},
		{"a panic is unnamed", "--- FAIL: TestX (0s)\npanic: boom\nFAIL\t" + pkgA + "\t0.1s\n", Reds{Keys: []Key{{pkgA, "TestX"}}, Unnamed: true}},
		{"a fail with no named test is unnamed", "FAIL\t" + pkgA + " [build failed]\n", Reds{Unnamed: true}},
		{"a pending name is unnamed", "--- FAIL: TestX (0s)\n", Reds{Unnamed: true}},
		{"ok lines and noise are skipped", "ok  \t" + pkgB + "\t0.1s\nexit status 1\n--- FAIL: TestX (0s)\nFAIL\t" + pkgA + "\t0.1s\n", Reds{Keys: []Key{{pkgA, "TestX"}}}},
	} {
		if got := ParseGoTest(tc.out); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: ParseGoTest = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestKeysOf_SplitsAPackageAndATopLevelTest(t *testing.T) {
	t.Parallel()
	got := KeysOf([]string{"github.com/o/m/go/internal/dup.TestCount", "example.com/m/a.TestY/sub.case"})
	want := Reds{Keys: []Key{{"github.com/o/m/go/internal/dup", "TestCount"}, {"example.com/m/a", "TestY"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("KeysOf = %+v, want %+v", got, want)
	}
}

func TestKeysOf_ANameWithNoTestIsUnnamed(t *testing.T) {
	t.Parallel()
	got := KeysOf([]string{"github.com/o/m/go/internal/dup.TestCount", "github.com/o/m/go/internal/pkgonly"})
	if !got.Unnamed || len(got.Keys) != 1 {
		t.Errorf("KeysOf = %+v, want one key and Unnamed", got)
	}
	if KeysOf(nil).Attributable() {
		t.Error("an empty list is attributable")
	}
}

func TestFailedTests_NamesEachTopLevelFailOnceInOrder(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		out  string
		want []string
	}{
		{"no output", "", nil},
		{"a pass", "ok  \texample.com/m/a\t0.01s\n", nil},
		{"subtests fold into the parent once", "    --- FAIL: TestX/sub (0.00s)\n--- FAIL: TestX (0.00s)\n--- FAIL: TestY (0.00s)\n", []string{"TestX", "TestY"}},
		{"a pending test with no package line still counts", "--- FAIL: TestZ (0.00s)\npanic: boom\n", []string{"TestZ"}},
		{"an empty name is skipped", "--- FAIL: \n", nil},
	} {
		if got := FailedTests(tc.out); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: FailedTests = %v, want %v", tc.name, got, tc.want)
		}
	}
}
