package journeys_test

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/journeys"
)

const goRoot = "../.."

func TestRegistry_EveryActiveRowNamesATestInTheTree(t *testing.T) {
	t.Parallel()
	reg, err := journeys.Load(filepath.Join(goRoot, ".."))
	if err != nil {
		t.Fatal(err)
	}

	missing, err := journeys.MissingTests(goRoot, reg.Rows)

	if err != nil || len(missing) != 0 {
		t.Fatalf("rows %v name a test that is not in the tree (%v): fix the row, or retire it with a reason", missing, err)
	}
}
