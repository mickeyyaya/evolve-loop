package testaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const weakNamedTest = "package fx\n\nimport \"testing\"\n\nfunc %s(t *testing.T) {\n\t_ = 1\n}\n"

const assertingNamedTest = "package fx\n\nimport \"testing\"\n\nfunc %s(t *testing.T) {\n\tt.Fatal(\"x\")\n}\n"

const centralWithOneRow = `{"packages":["pkg/a"],"rows":[{"package":"pkg/a","test":"TestFixedLater","rule":"no-assertion"}]}`

func TestRebaseline_RefusesNewRowsEvenWhenTheCountDoesNotGrow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		central string
	}{
		{name: "local shard, one fixed and one added"},
		{name: "central shard, one fixed and one added", central: centralWithOneRow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			centralPath := filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath))
			tracked := filepath.Join(root, "pkg", "a", shardRelPath)
			mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), fmt.Sprintf(weakNamedTest, "TestFixedLater"))
			if tc.central != "" {
				tracked = centralPath
				mustWrite(t, centralPath, tc.central)
			} else if err := Rebaseline(root, "pkg/a", Config{}); err != nil {
				t.Fatalf("seed rebaseline: %v", err)
			}
			mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), fmt.Sprintf(assertingNamedTest, "TestFixedLater"))
			mustWrite(t, filepath.Join(root, "pkg", "a", "b_test.go"), fmt.Sprintf(weakNamedTest, "TestAddedWeak"))
			before, err := os.ReadFile(tracked)
			if err != nil {
				t.Fatalf("read seeded baseline: %v", err)
			}
			err = Rebaseline(root, "pkg/a", Config{})
			if err == nil {
				t.Fatal("an equal-count swap was accepted: the ratchet admitted TestAddedWeak")
			}
			if !strings.Contains(fmt.Sprint(err), "TestAddedWeak") {
				t.Errorf("refusal does not name TestAddedWeak: %v", err)
			}
			after, readErr := os.ReadFile(tracked)
			if readErr != nil || string(before) != string(after) {
				t.Errorf("a refused rebaseline changed the baseline: %v\n%s\n--\n%s", readErr, before, after)
			}
		})
	}
}

func TestRebaseline_RefusesGrowthFromAnEmptyCentralPackage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	centralPath := filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath))
	mustWrite(t, centralPath, `{"packages":["pkg/a"],"rows":[]}`)
	mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), fmt.Sprintf(weakNamedTest, "TestFirstWeakInCentral"))
	before, err := os.ReadFile(centralPath)
	if err != nil {
		t.Fatal(err)
	}
	err = Rebaseline(root, "pkg/a", Config{})
	if err == nil {
		t.Fatal("growth from zero rows in a central package was accepted")
	}
	if !strings.Contains(fmt.Sprint(err), "TestFirstWeakInCentral") {
		t.Errorf("refusal does not name TestFirstWeakInCentral: %v", err)
	}
	after, readErr := os.ReadFile(centralPath)
	if readErr != nil || string(before) != string(after) {
		t.Errorf("a refused rebaseline changed the central file: %v\n%s\n--\n%s", readErr, before, after)
	}
}

func TestRebaseline_AcceptsAShrinkToASubsetOfTheOldRows(t *testing.T) {
	t.Parallel()
	for _, central := range []bool{false, true} {
		name := "local"
		if central {
			name = "central"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			centralPath := filepath.Join(root, filepath.FromSlash(centralBaselinesRelPath))
			mustWrite(t, filepath.Join(root, "pkg", "a", "a_test.go"), fmt.Sprintf(weakNamedTest, "TestKept"))
			mustWrite(t, filepath.Join(root, "pkg", "a", "b_test.go"), fmt.Sprintf(weakNamedTest, "TestDropped"))
			if central {
				mustWrite(t, centralPath, `{"packages":["pkg/a"],"rows":[{"package":"pkg/a","test":"TestDropped","rule":"no-assertion"},{"package":"pkg/a","test":"TestKept","rule":"no-assertion"}]}`)
			} else if err := Rebaseline(root, "pkg/a", Config{}); err != nil {
				t.Fatalf("seed rebaseline: %v", err)
			}
			mustWrite(t, filepath.Join(root, "pkg", "a", "b_test.go"), fmt.Sprintf(assertingNamedTest, "TestDropped"))
			if err := Rebaseline(root, "pkg/a", Config{}); err != nil {
				t.Fatalf("a shrink to a subset of the old rows was refused: %v", err)
			}
			if err := CheckTree(root, Config{}); err != nil {
				t.Errorf("CheckTree after the shrink = %v, want nil", err)
			}
			if central {
				got, _, err := readCentralBaselines(centralPath)
				if err != nil || len(got.Rows) != 1 || got.Rows[0].Test != "TestKept" {
					t.Errorf("central rows after the shrink = %v (%v), want only TestKept", got.Rows, err)
				}
			}
		})
	}
}
