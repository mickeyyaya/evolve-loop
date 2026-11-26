package opscmd

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

var cellBinRoot string

func fixedCellBinRoot() (string, error) { return cellBinRoot, nil }

func TestMain(m *testing.M) {
	doctorUsageEvidenceFn = func(string, bridge.Deps) (usageevidence.Explain, func()) { return nil, func() {} }
	root, err := os.MkdirTemp("", "evopscellbin")
	if err != nil {
		os.Exit(1)
	}
	cellBinRoot = root
	code := m.Run()
	_ = os.RemoveAll(root)
	os.Exit(code)
}
