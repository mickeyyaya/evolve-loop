package opscmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

var cellBinRoot string

func fixedCellBinRoot() (string, error) { return cellBinRoot, nil }

func TestMain(m *testing.M) {
	restoreHome, err := gittest.IsolateHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: isolate HOME: %v\n", err)
		os.Exit(1)
	}
	doctorUsageEvidenceFn = func(string, bridge.Deps) (usageevidence.Explain, func()) { return nil, func() {} }
	root, err := os.MkdirTemp("", "evopscellbin")
	if err != nil {
		_ = restoreHome()
		os.Exit(1)
	}
	cellBinRoot = root
	code := m.Run()
	_ = os.RemoveAll(root)
	if err := restoreHome(); err != nil {
		fmt.Fprintf(os.Stderr, "TestMain: restore HOME: %v\n", err)
	}
	os.Exit(code)
}
