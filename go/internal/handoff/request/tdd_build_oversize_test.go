package request

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/plainfile"
)

func TestTDDBuild_AnOversizeTestFileFailsTheRequest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "big_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxTestFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := TDDBuild(dir); !errors.Is(err, plainfile.ErrTooLarge) {
		t.Errorf("TDDBuild = %v, want plainfile.ErrTooLarge", err)
	}
}
