package repocontract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
)

func TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".test-dup-baseline"))
	if err != nil {
		t.Fatalf("go/.test-dup-baseline must exist and hold the count and the date: %v", err)
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		t.Fatal(err)
	}

	report, err := testdup.Scan(root, baseline.Date)
	if err != nil {
		t.Fatal(err)
	}

	if err := testdup.Check(report, baseline); err != nil {
		t.Error(err)
	}
}
