//go:build acs

package cycle1568

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const retroPkg = "./internal/phases/retro"

func runRetroTest(t *testing.T, name string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", retroPkg, "-run", "^"+name+"$", "-count=1", "-v")
	return stdout + stderr, code
}

func assertNamedPass(t *testing.T, name string) {
	t.Helper()
	out, code := runRetroTest(t, name)
	if !strings.Contains(out, "=== RUN   "+name) {
		t.Fatalf("%s never ran (deleted, renamed, or filtered out)\n%s", name, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) || code != 0 {
		t.Errorf("%s did not PASS (exit=%d)\n%s", name, code, out)
	}
}

func TestC1568_001_submit_wedged_relaunches_retro_once(t *testing.T) {
	assertNamedPass(t, "TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce")
}

func TestC1568_002_generic_artifact_timeout_does_not_relaunch(t *testing.T) {
	assertNamedPass(t, "TestRun_GenericArtifactTimeout_DoesNotRelaunch")
}

func TestC1568_003_relaunch_test_drives_production_retro_route(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "phases", "retro", "retro_test.go")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s untracked — may be gitignored and dropped at ship", rel)
	}
	for _, marker := range []string{"phase.Run(", "core.BridgeRequest", "core.ErrArtifactTimeout"} {
		if !acsassert.FileContains(t, abs, marker) {
			t.Errorf("RED: %s does not reference %q — not the production route", rel, marker)
		}
	}
	assertNamedPass(t, "TestRun_SubmitWedgedDeliveryFailure_RelaunchesOnce")
}

func TestC1568_004_retro_never_dispatches_auto_model(t *testing.T) {
	assertNamedPass(t, "TestRun_AutoModel_ResolvedBeforeDispatch")
}

func TestC1568_005_explicit_model_tier_unchanged(t *testing.T) {
	assertNamedPass(t, "TestRun_ExplicitModel_PassesThroughUnchanged")
}

func TestC1568_006_sentinel_never_survives_degraded_resolution(t *testing.T) {
	assertNamedPass(t, "TestRun_AutoModel_ProfileWithoutTier_ResolvesToDefaultNotAuto")
	assertNamedPass(t, "TestRun_AutoModel_NoProfile_NeverDispatchesSentinel")
}

func TestC1568_007_retro_package_suite_green(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", retroPkg, "-count=1")
	if code != 0 {
		t.Errorf("go test %s exited %d — retro package regression\nstdout:\n%s\nstderr:\n%s",
			retroPkg, code, stdout, stderr)
	}
}
