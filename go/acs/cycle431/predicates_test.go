//go:build acs

package cycle431

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgeImportPath     = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	panestreamImportPath = "github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func runBridgeTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, bridgeImportPath,
	)
	return stdout, stderr, code
}

func runRaceSuite(t *testing.T, runFilter string, pkgs ...string) (string, string, int) {
	t.Helper()
	args := []string{"test", "-race", "-count=1"}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout, stderr, code
}

func TestC431_001_DriverRoutesStateThroughCenter(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_SignalCenterStateWins")
	if code != 0 {
		t.Errorf("C431_001: driver-uses-SignalCenter test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_002_BooleanFallbackRetired(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestStopEvent_BooleanFallbackRetired")
	if code != 0 {
		t.Errorf("C431_002: boolean-fallback-retired test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_003_BridgeAndPanestreamRaceGreen(t *testing.T) {
	_, stderr, code := runRaceSuite(t, "", bridgeImportPath, panestreamImportPath)
	if code != 0 {
		t.Errorf("C431_003: bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_004_ApicoverEnforceClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "apicover431")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", goDir, "-o", binPath, "./cmd/apicover",
	); code != 0 {
		t.Fatalf("C431_004: build apicover binary exit=%d: %s", code, stderr)
	}

	coverPath := filepath.Join(tmp, "coverage431.txt")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1",
		"-coverprofile="+coverPath,
		"./internal/bridge/...", "./internal/bridge/panestream/...",
	); code != 0 {
		t.Fatalf("C431_004: coverage run exit=%d: %s", code, stderr)
	}

	funcOut, funcErr, code, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+coverPath)
	if code != 0 {
		t.Fatalf("C431_004: go tool cover -func exit=%d: %s", code, funcErr)
	}
	funcPath := filepath.Join(tmp, "coverage431.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("C431_004: write func profile: %v", err)
	}

	dirOut, dirErr, code, _ := acsassert.SubprocessOutput(
		"go", "list", "-C", goDir, "-f", "{{.Dir}}",
		"./internal/bridge", "./internal/bridge/panestream",
	)
	if code != 0 {
		t.Fatalf("C431_004: go list package dirs exit=%d: %s", code, dirErr)
	}
	dirs := strings.Fields(dirOut)
	if len(dirs) != 2 {
		t.Fatalf("C431_004: expected 2 package dirs, got %v", dirs)
	}

	args := append([]string{"-cover", funcPath, "-enforce"}, dirs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(binPath, args...)
	if code != 0 {
		t.Errorf("C431_004: apicover -enforce exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}

func TestC431_005_CenterProbeActuallyInvoked(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_SignalCenterProbeActuallyInvoked")
	if code != 0 {
		t.Errorf("C431_005: center-probe-invoked test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_006_RenderWedgeOverridePreserved(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant")
	if code != 0 {
		t.Errorf("C431_006: render-wedge-preserved test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_007_ConvergingProducingNeverCapped(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_Converging_ProducingNeverCapped")
	if code != 0 {
		t.Errorf("C431_007: converging-never-capped test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_008_BusyStagnantBoundedThenPause(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_BusyStagnant_BoundedThenPause")
	if code != 0 {
		t.Errorf("C431_008: busy-stagnant-bounded test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_009_DeadPaneHungNotConverging(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_DeadPane_HungIsNotConverging")
	if code != 0 {
		t.Errorf("C431_009: dead-pane-hung test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_010_EvidenceSurvivesEmptyCapture(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_EvidenceSurvivesEmptyCapture")
	if code != 0 {
		t.Errorf("C431_010: evidence-survives test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC431_011_WedgeCorpusRaceGreen(t *testing.T) {
	_, stderr, code := runRaceSuite(t, "Wedge|Converging|BusyStagnant|DeadPane|Evidence", bridgeImportPath)
	if code != 0 {
		t.Errorf("C431_011: wedge corpus -race exit non-zero\nstderr=%s", stderr)
	}
}

func TestC431_012_CorpusUsesRealReviewer(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestWedgeCorpus_UsesRealDeterministicReviewer")
	if code != 0 {
		t.Errorf("C431_012: uses-real-reviewer test exit=%d\nstderr=%s", code, stderr)
	}
}
