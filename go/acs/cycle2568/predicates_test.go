//go:build acs

// Package cycle2568 pins the idle-pane checkpoint: a busy pane keeps the full interval and a deliverable written inside the grace completes with no nudge.
package cycle2568

import (
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const noTestsMarker = "no tests to run"

func moduleDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	return dir
}

func requirePassingTests(t *testing.T, pkg string, names []string, want int) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir(t), "test", "-count=1", "-v", "-run", pattern, pkg)
	out := stdout + stderr
	if code != 0 || err != nil || strings.Contains(out, noTestsMarker) {
		t.Fatalf("go test -run %s %s exit=%d err=%v\n%s", pattern, pkg, code, err, acsassert.OutputTail(out, 40))
	}
	if got := strings.Count("\n"+out, "\n--- PASS: "); got != want {
		t.Errorf("go test -run %s %s passed %d test(s), want %d", pattern, pkg, got, want)
	}
}

func TestC2568_002_ABusyPaneStillWaitsTheFullInterval(t *testing.T) {
	requirePassingTests(t, "./internal/bridge", []string{
		"TestIdleCheckpoint_BusyPaneWaitsTheFullInterval",
		"TestIdleCheckpoint_IdleClockRestartsAfterBusyWork",
	}, 2)
}

func TestC2568_003_ADeliverableInsideTheGraceCompletesWithoutANudge(t *testing.T) {
	requirePassingTests(t, "./internal/bridge", []string{
		"TestIdleCheckpoint_ArtifactInsideTheGraceCompletesWithoutANudge",
	}, 1)
}
