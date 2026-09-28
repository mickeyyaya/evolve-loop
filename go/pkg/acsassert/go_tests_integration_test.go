//go:build integration

package acsassert

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGoTests_ActualExecutionAndNegativeControls(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantOK     bool
	}{
		{"pass", "func TestExpected(t *testing.T) {}", true},
		{"fail", "func TestExpected(t *testing.T) { t.Fatal(\"broken contract\") }", false},
		{"skip", "func TestExpected(t *testing.T) { t.Skip(\"unavailable\") }", false},
		{"prefix_collision", "func TestExpectedSuffix(t *testing.T) {}", false},
		{"compile_error", "func TestExpected(t *testing.T) { doesNotCompile() }", false},
		{"hidden_by_testmain", "func TestExpected(t *testing.T) {}\nfunc TestMain(m *testing.M) { os.Exit(0) }", false},
		{"printed_pass", "func TestExpected(t *testing.T) {}\nfunc TestMain(m *testing.M) { fmt.Println(\"--- PASS: TestExpected (0.00s)\"); os.Exit(0) }", false},
		{"swallowed_failure", "func TestExpected(t *testing.T) { t.Fatal(\"broken\") }\nfunc TestMain(m *testing.M) { m.Run(); os.Exit(0) }", false},
		{"pass_then_exit_failure", "func TestExpected(t *testing.T) {}\nfunc TestMain(m *testing.M) { m.Run(); os.Exit(7) }", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range map[string]string{
				"go.mod":           "module example.test/contract\n\ngo 1.23\n",
				"contract_test.go": "package contract\nimport (\"testing\";\"fmt\";\"os\")\nvar _ = fmt.Println\nvar _ = os.Exit\n" + tc.body,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			ft := &fakeT{}
			got := GoTests(ft, GoTestSpec{Dir: dir, Package: "example.test/contract", Pattern: "^TestExpected$", Names: []string{"TestExpected"}})
			if got != tc.wantOK || (len(ft.errs) == 0) != tc.wantOK {
				t.Fatalf("GoTests = %v, errors=%v; want success=%v and corresponding assertion status", got, ft.errs, tc.wantOK)
			}
		})
	}
}

func TestGoTests_PreservesRaceAndBuildTags(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":         "module example.test/contract\n\ngo 1.23\n",
		"tagged_test.go": "//go:build oracle_tag\n\npackage contract\nimport \"testing\"\nfunc TestTagged(t *testing.T) {}\n",
		"race_test.go":   "//go:build race\n\npackage contract\nimport \"testing\"\nfunc TestRace(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, tags string
		race       bool
		tests      []string
	}{
		{"tag", "oracle_tag", false, []string{"TestTagged"}},
		{"race", "", true, []string{"TestRace"}},
		{"both", "oracle_tag", true, []string{"TestTagged", "TestRace"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			GoTests(t, GoTestSpec{Dir: dir, Package: "example.test/contract", Pattern: "^Test(Tagged|Race)$", Names: tc.tests, Race: tc.race, Tags: tc.tags})
		})
	}
}

type deadlineT struct {
	fakeT
	deadline time.Time
}

func (t *deadlineT) Deadline() (time.Time, bool) { return t.deadline, true }

func TestGoTests_RespectsCallerDeadline(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":           "module example.test/contract\n\ngo 1.23\n",
		"contract_test.go": "package contract\nimport \"testing\"\nfunc TestExpected(t *testing.T) {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ft := &deadlineT{deadline: time.Now().Add(-time.Second)}
	if GoTests(ft, GoTestSpec{Dir: dir, Package: "example.test/contract", Pattern: "^TestExpected$", Names: []string{"TestExpected"}}) || len(ft.errs) == 0 {
		t.Fatal("expired caller deadline still permitted a successful child execution")
	}
}
