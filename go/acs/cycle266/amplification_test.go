//go:build acs

package cycle266

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

func TestC266_Amp001_DuplicateDetectorHandlesNilEmptyAndNonAdjacentDuplicates(t *testing.T) {
	root := acsassert.RepoRoot(t)
	testPath := filepath.Join(root, "go", "internal", "routingtest", "cycle266_amplification_test.go")
	testSrc := `package routingtest

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func cycle266PlanWithPhases(t *testing.T, phases ...string) *router.PhasePlan {
	t.Helper()
	plan := &router.PhasePlan{}
	entries := reflect.ValueOf(plan).Elem().FieldByName("Entries")
	if !entries.IsValid() {
		t.Fatalf("PhasePlan has no Entries field")
	}
	slice := reflect.MakeSlice(entries.Type(), len(phases), len(phases))
	for i, phase := range phases {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem = reflect.New(elem.Type().Elem())
			slice.Index(i).Set(elem)
			elem = elem.Elem()
		}
		field := elem.FieldByName("Phase")
		if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.String {
			t.Fatalf("PhasePlan entry has no settable string Phase field")
		}
		field.SetString(phase)
	}
	entries.Set(slice)
	return plan
}

func TestCycle266AmplificationDuplicatePlanPhasesEdges(t *testing.T) {
	if got := duplicatePlanPhases(nil); len(got) != 0 {
		t.Fatalf("nil plan reported %d duplicates, want 0", len(got))
	}
	if got := duplicatePlanPhases(cycle266PlanWithPhases(t)); len(got) != 0 {
		t.Fatalf("empty plan reported %d duplicates, want 0", len(got))
	}
	if got := duplicatePlanPhases(cycle266PlanWithPhases(t, "scout", "build", "audit", "ship")); len(got) != 0 {
		t.Fatalf("unique plan reported %d duplicates, want 0", len(got))
	}
	if got := duplicatePlanPhases(cycle266PlanWithPhases(t, "scout", "build", "audit", "scout")); len(got) != 1 {
		t.Fatalf("non-adjacent duplicate reported %d duplicates, want 1", len(got))
	}
	if got := duplicatePlanPhases(cycle266PlanWithPhases(t, "scout", "build", "scout", "audit", "build", "build")); len(got) != 3 {
		t.Fatalf("multiple repeated phases reported %d duplicates, want 3 reports after first occurrences", len(got))
	}
}
`
	if err := os.WriteFile(testPath, []byte(testSrc), 0644); err != nil {
		t.Fatalf("write temporary amplification test: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(testPath)
	})

	goDir := filepath.Join(root, "go")
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-run", "TestCycle266AmplificationDuplicatePlanPhasesEdges", "-v",
		"./internal/routingtest/...")
	out := stdout + "\n" + stderr
	if !strings.Contains(out, "--- PASS: TestCycle266AmplificationDuplicatePlanPhasesEdges") {
		t.Fatalf("temporary duplicate-detector edge test did not pass:\n%s", out)
	}
}
