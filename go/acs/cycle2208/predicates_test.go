//go:build acs

package cycle2208

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdPkg = "./cmd/evolve"

func requireTestsPass(t *testing.T, names ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", cmdPkg)
	if err != nil || code != 0 {
		t.Fatalf("RED: exit=%d err=%v\n%s\n%s", code, err, acsassert.OutputTail(out, 40), acsassert.OutputTail(errOut, 20))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 40))
		}
	}
}

func TestC2208_001_SyncMainRefusesOnLiveLaneLeaseWithoutRootCycleState(t *testing.T) {
	requireTestsPass(t,
		"TestSyncMain_RefusesOnLiveLaneLeaseWithoutRootCycleState",
		"TestSyncMain_RefusesWhenOneOfSeveralLaneLeasesIsLive",
	)
}

func TestC2208_002_SyncMainStillMergesWhenNoRunIsLive(t *testing.T) {
	requireTestsPass(t,
		"TestSyncMain_MergesWhenNoRunIsLive",
		"TestSyncMain_MergesQuietDivergedTree",
	)
}

func TestC2208_003_SyncMainStillRefusesOnClassicRootCycleStateLease(t *testing.T) {
	requireTestsPass(t, "TestSyncMain_RefusesOnLiveLease")
}
