//go:build acs

package cycle434

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

func runPanestreamTest(t *testing.T, runFilter string) (string, string, int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runFilter, panestreamImportPath,
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

func TestC434_001_AutoResponderBusyGatePreserved(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestAutoResponderTick_BusyGateViaCenter_SuppressesEscalate")
	if code != 0 {
		t.Errorf("C434_001: busy-gate-preserved test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_002_AutoResponderIdleStillEscalates(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestAutoResponderTick_IdleGateViaCenter_Escalates")
	if code != 0 {
		t.Errorf("C434_002: idle-still-escalates test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_003_IdleReachedFiresOnceViaFacade(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestChannelE2E_RealFixtures_ClaudeSpan")
	if code != 0 {
		t.Errorf("C434_003: idle_reached-via-facade e2e test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_004_NoDirectChromeParseAtAutoResponderTick(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestAutoResponderTick_NoDirectChromeParse")
	if code != 0 {
		t.Errorf("C434_004: no-direct-chrome-parse (autorespond) test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_005_NoDirectChromeParseAtIdleReachedBracket(t *testing.T) {
	_, stderr, code := runBridgeTest(t, "TestRunTmuxREPL_NoDirectChromeParseAtIdleReachedBracket")
	if code != 0 {
		t.Errorf("C434_005: no-direct-chrome-parse (idle_reached bracket) test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_006_BusyOfEdgeCasesNoPanic(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestSignalCenter_BusyOf_EmptyPaneUnknownProfileNoPanic|TestSignalCenter_BusyOf_NilReceiverSafe")
	if code != 0 {
		t.Errorf("C434_006: BusyOf edge-case tests exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_007_BusyOfStatelessNoSessionMutation(t *testing.T) {
	_, stderr, code := runPanestreamTest(t, "TestSignalCenter_BusyOf_MatchesStandalonePaneBusy|TestSignalCenter_BusyOf_StatelessNoSessionMutation")
	if code != 0 {
		t.Errorf("C434_007: BusyOf stateless/delegation tests exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_008_BridgeAndPanestreamRaceGreen(t *testing.T) {
	_, stderr, code := runRaceSuite(t, "", bridgeImportPath, panestreamImportPath)
	if code != 0 {
		t.Errorf("C434_008: bridge+panestream -race suite exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC434_009_ApicoverEnforceClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	tmp := t.TempDir()

	binPath := filepath.Join(tmp, "apicover434")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build", "-C", goDir, "-o", binPath, "./cmd/apicover",
	); code != 0 {
		t.Fatalf("C434_009: build apicover binary exit=%d: %s", code, stderr)
	}

	coverPath := filepath.Join(tmp, "coverage434.txt")
	if _, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1",
		"-coverprofile="+coverPath,
		"./internal/bridge/...", "./internal/bridge/panestream/...",
	); code != 0 {
		t.Fatalf("C434_009: coverage run exit=%d: %s", code, stderr)
	}

	funcOut, funcErr, code, _ := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+coverPath)
	if code != 0 {
		t.Fatalf("C434_009: go tool cover -func exit=%d: %s", code, funcErr)
	}
	funcPath := filepath.Join(tmp, "coverage434.func.txt")
	if err := os.WriteFile(funcPath, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("C434_009: write func profile: %v", err)
	}

	dirOut, dirErr, code, _ := acsassert.SubprocessOutput(
		"go", "list", "-C", goDir, "-f", "{{.Dir}}",
		"./internal/bridge", "./internal/bridge/panestream",
	)
	if code != 0 {
		t.Fatalf("C434_009: go list package dirs exit=%d: %s", code, dirErr)
	}
	dirs := strings.Fields(dirOut)
	if len(dirs) != 2 {
		t.Fatalf("C434_009: expected 2 package dirs, got %v", dirs)
	}

	args := append([]string{"-cover", funcPath, "-enforce"}, dirs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(binPath, args...)
	if code != 0 {
		t.Errorf("C434_009: apicover -enforce exit=%d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
}
