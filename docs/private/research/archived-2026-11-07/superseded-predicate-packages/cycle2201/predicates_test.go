//go:build acs

package cycle2201

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func bridgeTests(t *testing.T, pattern string, names ...string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(root, "go"),
		Package: "./internal/bridge",
		Pattern: pattern,
		Names:   names,
		Tags:    "integration",
	})
}

func TestC2201_001_ThePrivateTmuxRowSurvivesAHostThatDelaysEachClient(t *testing.T) {
	bridgeTests(t, "^TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer$",
		"TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer")
}

func TestC2201_002_TheRowWaitsOnReadinessAndNamesAServerThatNeverStarts(t *testing.T) {
	bridgeTests(t, "^(TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep|TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady)$",
		"TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep",
		"TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady")
}

func TestC2201_003_AnUnloadedHostStillGetsAWindowOnItsOwnPty(t *testing.T) {
	bridgeTests(t, "^TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty$",
		"TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty")
}
