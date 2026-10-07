package opscmd

import (
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageevidence"
)

func TestMain(m *testing.M) {
	doctorUsageEvidenceFn = func(string, bridge.Deps) (usageevidence.Explain, func()) { return nil, func() {} }
	os.Exit(m.Run())
}
