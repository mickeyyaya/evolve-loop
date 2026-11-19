package regressiontia

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSkippable_IsFalseForEachEscapeHatchOrAnEmptyClosureAndTrueOtherwise(t *testing.T) {
	t.Parallel()
	for _, hatch := range []string{"os", "os/exec", "os/user", "syscall", "net", "net/http"} {
		if skippable([]string{"example.com/m/internal/leaf", hatch}) {
			t.Errorf("skippable with %s in the closure = true, want false", hatch)
		}
	}
	if !skippable([]string{"example.com/m/internal/leaf"}) {
		t.Error("a closure with no escape hatch is skippable")
	}
	if skippable(nil) {
		t.Error("an empty closure is not skippable")
	}
}

func TestSplit_PartitionsTheCorpusAndTreatsUnknownAsIO(t *testing.T) {
	t.Parallel()
	patterns := []string{"./acs/regression/a", "./acs/regression/b", "./acs/regression/c"}
	reach := map[string][]string{
		"./acs/regression/a": {"example.com/m/internal/leaf"},
		"./acs/regression/b": {"example.com/m/internal/leaf", "os"},
	}

	pure, io := Split(patterns, reach)

	if !reflect.DeepEqual(pure, []string{"./acs/regression/a"}) {
		t.Errorf("pure = %v, want only a", pure)
	}
	if !reflect.DeepEqual(io, []string{"./acs/regression/b", "./acs/regression/c"}) {
		t.Errorf("io = %v, want b (reaches os) and c (no dependency data)", io)
	}
}

func TestCompute_ReportsThePureAndIOCounts(t *testing.T) {
	repoRoot, moduleDir := stdlibOnlyThroughTestingModule(t)

	d := Compute("shadow", repoRoot, moduleDir, []string{"./internal/other/..."})

	if d.PureCount != 1 || d.IOCount != 1 {
		t.Errorf("PureCount=%d IOCount=%d, want 1 and 1", d.PureCount, d.IOCount)
	}
}

func TestRegressionCorpus_CountsThePurePredicates(t *testing.T) {
	moduleDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	patterns := corpus(moduleDir)
	reach, _ := listCorpusDeps(moduleDir, patterns)

	pure, io := Split(patterns, reach)

	if len(pure)+len(io) != len(patterns) || len(patterns) == 0 {
		t.Fatalf("pure %d + io %d != corpus %d", len(pure), len(io), len(patterns))
	}
	if len(pure) != recordedPurePredicates {
		t.Errorf("pure predicates = %d, want %d (the count in the package note); update both when the corpus changes: %v", len(pure), recordedPurePredicates, pure)
	}
}

const recordedPurePredicates = 0

func TestSplitAndSkippableDepsAgreeOnAnEmptyClosure(t *testing.T) {
	const pkg = "./acs/regression/empty"
	reach := map[string][]string{pkg: {}}

	pure, io := Split([]string{pkg}, reach)
	skipped := skippableDeps(reach, "example.com/x")

	if len(pure) != 0 || len(io) != 1 {
		t.Errorf("Split pure=%v io=%v: a predicate with an empty closure is unknown and must count as I/O", pure, io)
	}
	if _, ok := skipped[pkg]; ok {
		t.Errorf("skippableDeps kept %s: an empty closure must never be skippable", pkg)
	}
}
