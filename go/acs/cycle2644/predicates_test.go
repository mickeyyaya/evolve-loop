//go:build acs

package cycle2644

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const handoffTests = "cmd/evolve/cmd_loop_boot_refresh_handoff_test.go"

func runNamedTests(t *testing.T, names ...string) (output string, code int) {
	t.Helper()
	mod := acsassert.GoModuleDir(t)
	requireTrackable(t, mod)
	exactNames := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, _ := acsassert.SubprocessOutputIn(mod, "go", "test", "-count=1", "-v", "-run", exactNames, "./cmd/evolve")
	return stdout + stderr, code
}

func requireTrackable(t *testing.T, mod string) {
	t.Helper()
	path := filepath.Join(mod, handoffTests)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk", handoffTests)
	}
	if _, _, code, _ := acsassert.SubprocessOutputIn(mod, "git", "check-ignore", "-q", handoffTests); code == 0 {
		t.Fatalf("RED: %s is gitignored and would be dropped at ship", handoffTests)
	}
}

func requirePassed(t *testing.T, output string, code int, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(output, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass", name)
		}
	}
	if code != 0 {
		t.Fatalf("RED: go test exited %d:\n%s", code, acsassert.OutputTail(output, 25))
	}
}

func TestC2644_001_BootRefreshReExecArmsTheWaveHandoffTheNextImageTakes(t *testing.T) {
	names := []string{
		"TestBootBinaryRefresh_ReExecArmsTheWaveHandoff",
		"TestRunLoop_BootRefreshReExecCarriesTheResumePointTheLaunchTook",
	}
	out, code := runNamedTests(t, names...)
	requirePassed(t, out, code, names...)
}

func TestC2644_002_UnshippedPlaneHeadIsNeverBuiltIntoTheLoopBinaryAndShippedHeadStillIs(t *testing.T) {
	names := []string{
		"TestBootBinaryRefresh_UnshippedPlaneHeadDoesNotRebuild",
		"TestBootBinaryRefresh_ShippedPlaneHeadStillRebuilds",
	}
	out, code := runNamedTests(t, names...)
	requirePassed(t, out, code, names...)
}

func TestC2644_003_NothingToHandOffArmsNothingAndAFailedArmBootsAsIs(t *testing.T) {
	names := []string{
		"TestBootBinaryRefresh_ZeroResumeWavesArmsNothing",
		"TestBootBinaryRefresh_FailedArmBootsAsIsWithoutReExec",
	}
	out, code := runNamedTests(t, names...)
	requirePassed(t, out, code, names...)
}
