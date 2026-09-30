//go:build acs

package cycle752

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	shipPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	moverPkg = "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC752_001_UnlandedCommitNeverPromotes(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_UnlandedCommitSkipsPromotion")
}

func TestC752_002_LandedCommitPromotes(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_LandedCommitPromotes")
}

func TestC752_003_MoverReroutesUnlandedProcessedPromotion(t *testing.T) {
	runGoTest(t, moverPkg, "TestPromote_ProcessedRefusedWhenNotLanded")
}

func TestC752_004_NeedsReauditTerminalNeverPromotes(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_NeedsReauditOutcomeNeverPromotes")
}

func TestC752_005_UnlandedReleaseCarriesRetryNote(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_UnlandedReleaseCarriesRetryNote")
}

func TestC752_006_LandedResidualDrainKeepsGenericReason(t *testing.T) {
	runGoTest(t, shipPkg, "TestPromoteInbox_LandedResidualReleaseKeepsGenericReason")
}
