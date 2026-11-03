//go:build acs

package cycle2130

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPackage        = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	curationTests     = `^(TestCmd_Inbox(Edit|Route|Verify|Withdraw|Curation)|TestRunShipCmd_AutoConfirmIsAnAuditedBypass|TestInboxClaim_EachRefusalHasItsExitCode)`
	refusalTests      = `^(TestQueuedVerbs_InsideAPhasePaneStillRefuse|TestCLIRoutingWrite_IsRefusedInsideAPhase|TestCodeRefresh_InsideAPhaseItRefuses|TestRatchet_TightenIsRefusedInsideAPhase|TestRunShipCmd_AutoConfirmIsRefusedInsideACell)`
	markerGuardTest   = `^TestMain_ClearsInheritedCellMarkers$`
	minCurationPasses = 20
	minRefusalPasses  = 5
)

var passLineRE = regexp.MustCompile(`(?m)^--- PASS: `)

func setCellMarkers(t *testing.T, root string) {
	t.Helper()
	for _, name := range ipcenv.CellMarkers() {
		value := "1"
		if name == "EVOLVE_PROJECT_ROOT" {
			value = root
		}
		t.Setenv(name, value)
	}
}

func runInsideCell(t *testing.T, runRegexp string) (string, int) {
	t.Helper()
	setCellMarkers(t, acsassert.RepoRoot(t))
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", runRegexp, cmdPackage)
	return stdout + stderr, code
}

func TestC2130_001_InboxAndShipVerbTestsPassWithCellMarkersInherited(t *testing.T) {
	out, code := runInsideCell(t, curationTests)
	if code != 0 {
		t.Errorf("RED: the inbox and ship verb tests fail when the parent environment carries the cell markers (exit %d):\n%s", code, acsassert.OutputTail(out, 25))
	}
	if got := len(passLineRE.FindAllString(out, -1)); got < minCurationPasses {
		t.Errorf("RED: only %d top-level PASS lines for the curation tests, want at least %d; the run must exercise them", got, minCurationPasses)
	}
}

func TestC2130_002_TestMainClearsEveryCellMarkerItInherits(t *testing.T) {
	out, code := runInsideCell(t, markerGuardTest)
	if code != 0 {
		t.Errorf("RED: TestMain leaves an inherited cell marker set (exit %d):\n%s", code, acsassert.OutputTail(out, 15))
	}
	if !strings.Contains(out, "--- PASS: TestMain_ClearsInheritedCellMarkers") {
		t.Errorf("RED: the marker guard test did not run and pass:\n%s", acsassert.OutputTail(out, 15))
	}
}

func TestC2130_003_RefusalTestsThatSetAMarkerOnPurposeStillRefuse(t *testing.T) {
	out, code := runInsideCell(t, refusalTests)
	if code != 0 {
		t.Errorf("RED: a refusal test no longer refuses (exit %d):\n%s", code, acsassert.OutputTail(out, 25))
	}
	if got := len(passLineRE.FindAllString(out, -1)); got < minRefusalPasses {
		t.Errorf("RED: only %d refusal tests passed, want at least %d; clearing the markers must not drop a refusal test", got, minRefusalPasses)
	}
}

func TestC2130_004_CellMarkerSetIsNonEmptyAndNamesThePhaseMarker(t *testing.T) {
	markers := ipcenv.CellMarkers()
	if len(markers) == 0 {
		t.Fatal("RED: ipcenv.CellMarkers is empty; the clearing would be a no-op")
	}
	found := false
	for _, name := range markers {
		if name == ipcenv.PhaseKey {
			found = true
		}
	}
	if !found {
		t.Errorf("RED: %s is not a cell marker; the inbox refusal reads it", ipcenv.PhaseKey)
	}
}
