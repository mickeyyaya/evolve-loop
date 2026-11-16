//go:build acs

package cycle2444

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2444_001_AnUnlocatedLandingOrSealEmitsAWarnSignalNamingItsCycle(t *testing.T) {
	pkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "orchestrator")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v",
		"-run", "^TestOversight_AnUnlocatedLandingOrSealEmitsOneWarnSignalNamingItsCycleAndEvent$", pkg)
	if err != nil || code != 0 {
		t.Fatalf("a landing or a seal for a cycle that no live lane holds must emit one WARN signal naming the cycle and the event (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	if !strings.Contains(stdout, "--- PASS: TestOversight_AnUnlocatedLandingOrSealEmitsOneWarnSignalNamingItsCycleAndEvent") {
		t.Errorf("the oversight test did not run and pass:\n%s", acsassert.OutputTail(stdout, 20))
	}
}

func TestC2444_002_ALocatedLaneAndTheLandingCleanupStayUnchanged(t *testing.T) {
	pkg := filepath.Join(acsassert.RepoRoot(t), "go", "internal", "orchestrator")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v",
		"-run", "^TestOversight_AnUnlocatedLandingOrSealEmitsOneWarnSignalNamingItsCycleAndEvent$|^TestOversight_AnUnlocatedLandingKeepsTheLandingStateCleanup$|^TestOversight_AnUnlocatedLandingSignalIsAbsentForALocatedLane$", pkg)
	if err != nil || code != 0 {
		t.Fatalf("the signal for an unlocated landing must not change the cleanup or fire for a located lane (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, acsassert.OutputTail(stdout, 20), acsassert.OutputTail(stderr, 20))
	}
	for _, name := range []string{"KeepsTheLandingStateCleanup", "SignalIsAbsentForALocatedLane"} {
		if !strings.Contains(stdout, "--- PASS: TestOversight_AnUnlocatedLanding"+name) {
			t.Errorf("test %s did not run and pass:\n%s", name, acsassert.OutputTail(stdout, 20))
		}
	}
}
