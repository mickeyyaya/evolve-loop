//go:build acs

package cycle2583

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	syncMainPackage = "./cmd/evolve"
	runBound        = 10 * time.Minute
)

func requireTestsPass(t *testing.T, names ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runBound)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-count=1", "-run", "^("+strings.Join(names, "|")+")$", syncMainPackage)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		t.Errorf("go test %v failed: %v\n%s", names, err, acsassert.OutputTail(text, 40))
	}
	for _, name := range names {
		if !strings.Contains(text, "--- PASS: "+name+" ") {
			t.Errorf("%s did not pass: a missing, skipped or failed test is not a proof\n%s", name, acsassert.OutputTail(text, 20))
		}
	}
}

func TestC2583_001_AConsoleRouteEditWithABlockerIsCarriedThroughTheMerge(t *testing.T) {
	requireTestsPass(t, "TestSyncMain_CarriesAConsoleRouteEditThroughTheMerge")
}

func TestC2583_002_AConsoleConsumeMoveIsCarriedAndAnAuthoredRewriteStillRefuses(t *testing.T) {
	requireTestsPass(t,
		"TestSyncMain_CarriesAConsoleConsumeMove",
		"TestSyncMain_AConsumeMoveThatAlsoRewritesAnAuthoredFieldStillRefuses",
	)
}

func TestC2583_003_AnUntrackedCopyEqualToOriginIsRemovedAndADifferingCopyIsKept(t *testing.T) {
	requireTestsPass(t, "TestSyncMain_RemovesUntrackedCopiesIdenticalToOrigin")
}

func TestC2583_004_ASameFieldConflictKeepsOriginAndNamesTheDroppedConsoleEdit(t *testing.T) {
	requireTestsPass(t, "TestSyncMain_ASameFieldConflictKeepsOriginAndNamesTheDroppedConsoleEdit")
}

func TestC2583_005_AHandEditOfAnAuthoredFieldStillRefusesTheSync(t *testing.T) {
	requireTestsPass(t, "TestSyncMain_AnOperatorEditToAnInboxItemStillRefuses")
}
