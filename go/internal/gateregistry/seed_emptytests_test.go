package gateregistry

import (
	"context"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
)

func TestEmptyTestsEntry_CoversItsPackAndNamesNoBaseline(t *testing.T) {
	t.Parallel()
	e := emptytestsEntry()
	if e.Name != "empty-tests" || e.Kind != gatecontract.KindScanner || e.Stage != gatecontract.StageEnforce {
		t.Errorf("entry = %+v, want an enforced scanner named empty-tests", e)
	}
	if !slices.Equal(e.Covers, []string{"./internal/emptytests/..."}) || !slices.Equal(e.Files, []string{"go/internal/emptytests/"}) {
		t.Errorf("Covers = %q, Files = %q", e.Covers, e.Files)
	}
	if len(e.Inputs) != 1 || e.Inputs[0].Path != "go/" || e.Inputs[0].Class != gatecontract.ClassSubject {
		t.Errorf("Inputs = %+v, want the go/ subject alone and no baseline", e.Inputs)
	}
	if got := entryNamed(t, "empty-tests"); !slices.Equal(got.Covers, e.Covers) {
		t.Errorf("Entries() holds empty-tests with Covers %q", got.Covers)
	}
}

func TestEmptyTestsEntry_RunIsRedOnAnEmptyTestAndGreenOtherwise(t *testing.T) {
	t.Parallel()
	red := t.TempDir()
	lay(t, red, map[string]string{"go/p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"})
	verdict, err := emptytestsEntry().Run(context.Background(), gatecontract.Input{Root: red})
	if err != nil || len(verdict.Reds) != 1 {
		t.Errorf("Run = %+v, %v, want one red", verdict, err)
	}
	green := t.TempDir()
	lay(t, green, map[string]string{"go/p/p_test.go": "package p\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) { t.Skip(\"why\") }\n"})
	verdict, err = emptytestsEntry().Run(context.Background(), gatecontract.Input{Root: green})
	if err != nil || len(verdict.Reds) != 0 {
		t.Errorf("Run = %+v, %v, want no red", verdict, err)
	}
}
