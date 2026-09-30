//go:build acs

package cycle308

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

var (
	inboxOnce  sync.Once
	inboxOut   string
	shipOnce   sync.Once
	shipOut    string
	triageOnce sync.Once
	triageOut  string
	preOnce    sync.Once
	preOut     string
)

func runInbox(t *testing.T) string {
	t.Helper()
	inboxOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestReleaseCycleProcessing|TestInboxRelease", "./internal/inboxmover/")
		inboxOut = stdout + "\n" + stderr
	})
	return inboxOut
}

func runShip(t *testing.T) string {
	t.Helper()
	shipOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestInboxPromote_UnfinishedClaimReleasedOnShip", "./internal/phases/ship/")
		shipOut = stdout + "\n" + stderr
	})
	return shipOut
}

func runTriage(t *testing.T) string {
	t.Helper()
	triageOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestCommittedFloorCount_Malformed|TestCommittedFloorCount_Absent|TestDeferredFloorPackagesDecl_Malformed|TestDeferredFloorPackagesDecl_Absent",
			"./internal/triagecap/")
		triageOut = stdout + "\n" + stderr
	})
	return triageOut
}

func runPreflight(t *testing.T) string {
	t.Helper()
	preOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestCLIVersionInventory|TestVersionDrift", "./internal/looppreflight/")
		preOut = stdout + "\n" + stderr
	})
	return preOut
}

func requirePass(t *testing.T, out string, names ...string) {
	t.Helper()
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a gated test FAILed:\n%s", tail(out, 40))
	}
	for _, n := range names {
		if !topLevelPassed(out, n) {
			t.Errorf("RED: missing `--- PASS: %s` (build failure or assertion not met):\n%s", n, tail(out, 40))
		}
	}
}

func TestC308_001_InboxReleaseCycleProcessing(t *testing.T) {
	out := runInbox(t)
	requirePass(t, out,
		"TestReleaseCycleProcessing_ReleasesScopedCycle",
		"TestInboxRelease_FailedCycleReleasesAllClaimed",
		"TestInboxRelease_DoubleClaimRaceIsWarn",
	)
}

func TestC308_002_InboxResidualReleasedOnShip(t *testing.T) {
	out := runShip(t)
	requirePass(t, out, "TestInboxPromote_UnfinishedClaimReleasedOnShip")
}

func TestC308_003_CmdLoopFailTerminalWiresRelease(t *testing.T) {
	out := runInbox(t)
	requirePass(t, out, "TestInboxRelease_FailedCycleReleasesAllClaimed")

	cmdLoop := filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileContains(t, cmdLoop, "ReleaseCycleProcessing") {
		t.Errorf("RED: cmd_loop.go does not call inboxmover.ReleaseCycleProcessing — the cycle-fail terminal release is unwired (cycle-307 seam trap)")
	}
}

func TestC308_004_CommittedFloorMalformedSurfaces(t *testing.T) {
	out := runTriage(t)
	requirePass(t, out,
		"TestCommittedFloorCount_MalformedFieldSurfaces",
		"TestCommittedFloorCount_AbsentCompanionFallsBackSilently",
	)
}

func TestC308_005_DeferredFloorMalformedSurfaces(t *testing.T) {
	out := runTriage(t)
	requirePass(t, out,
		"TestDeferredFloorPackagesDecl_MalformedFieldSurfaces",
		"TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently",
	)
}

func TestC308_006_VersionInventoryAndDrift(t *testing.T) {
	out := runPreflight(t)
	requirePass(t, out,
		"TestCLIVersionInventory",
		"TestCLIVersionInventory_LandsInPreflight",
		"TestVersionDrift_Fires_On_Synthetic_Transition",
		"TestVersionDrift_NoWarnWhenVersionUnchanged",
		"TestVersionDrift_NoWarnWhenNoPriorRecord",
	)
}
