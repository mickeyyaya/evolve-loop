package looppreflight

import (
	"fmt"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func TestMain(m *testing.M) {
	restoreHome, err := gittest.IsolateHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate HOME: %v\n", err)
		os.Exit(1)
	}
	cellBin, err := os.MkdirTemp("", "evloopcellbin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate cell-bin: %v\n", err)
		_ = restoreHome()
		os.Exit(1)
	}
	versionCellBinRoot = func() (string, error) { return cellBin, nil }
	code := tmuxtest.Main(m)
	if err := os.RemoveAll(cellBin); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: remove %s: %v\n", cellBin, err)
	}
	if err := restoreHome(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: restore HOME: %v\n", err)
	}
	os.Exit(code)
}
