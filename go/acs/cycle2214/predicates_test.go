//go:build acs

package cycle2214

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

func TestC2214_001_ThePrivateTmuxRowSurvivesALoadedHostWhoseSessionShellEndsAtOnce(t *testing.T) {
	bridgeTests(t,
		"^(TestPrivateTmuxRow_TheServerOutlivesTheSessionShellThatEndsBeforeTheWindowStarts|TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer|TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty)$",
		"TestPrivateTmuxRow_TheServerOutlivesTheSessionShellThatEndsBeforeTheWindowStarts",
		"TestPrivateTmuxRow_AClientThatConnectsLateStillFindsItsServer",
		"TestPrivateTmuxRow_APromptClientStillGetsAWindowOnItsOwnPty")
}

func TestC2214_002_TheRowWaitsOnReadinessAndReportsTheRealCauseOfAServerThatNeverStarts(t *testing.T) {
	bridgeTests(t,
		"^(TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep|TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady|TestPrivateTmuxRow_AServerThatNeverStartsIsReportedWithTheCauseOfTmux)$",
		"TestPrivateTmuxRow_TheRowWaitsOnReadinessNotOnASleep",
		"TestPrivateTmuxRow_AServerThatNeverStartsIsReportedAsNotReady",
		"TestPrivateTmuxRow_AServerThatNeverStartsIsReportedWithTheCauseOfTmux")
}
