//go:build acs

package cycle563

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	routerPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	if err != nil {
		t.Fatalf("go test failed to launch for %s (%s): %v\nstderr:\n%s", pkg, pattern, err, stderr)
	}
	return code == 0, stdout + stderr
}

func TestC563_001_MemoRunnerRegistrationRegression(t *testing.T) {
	ok, out := runGoTest(t, cmdEvolvePkg, "TestWireOrchestrator_MemoRunnerRegistered")
	if !ok {
		t.Errorf("wireOrchestratorDeps does not register a PhaseRunner for \"memo\" — the routing→dispatch handoff is still silently dropping it:\n%s", out)
	}
}

func TestC563_002_MemoDisabledClampsSafely(t *testing.T) {
	ok, out := runGoTest(t, routerPkg, "TestRoute_PostShip_MemoEnabled_RoutesToMemo|TestRoute_PostShip_MemoDisabled_ClampsSafely")
	if !ok {
		t.Errorf("post-ship memo enable/disable routing regressed (either memo never reaches Route() when enabled, or a disabled memo still gets routed):\n%s", out)
	}
}

func TestC563_003_DoctorBootNoMemoWarning(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "run", cmdEvolvePkg, "doctor", "probe", "claude")
	if code == -1 {
		t.Fatalf("go run %s doctor probe claude failed to launch: %v\nstderr:\n%s", cmdEvolvePkg, err, stderr)
	}
	combined := stdout + stderr
	for _, line := range strings.Split(combined, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "memo") {
			continue
		}
		if strings.Contains(lower, "invalid") || strings.Contains(lower, "clashes with a built-in") || strings.Contains(lower, "not routed") {
			t.Errorf("evolve doctor printed a memo invalid/clash/not-routed warning while memo is also never dispatched — the silent WARN-and-drop class this task fixes:\n%s", line)
		}
	}
}
