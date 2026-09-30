//go:build acs

package cycle976

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
const routerPkg = "github.com/mickeyyaya/evolve-loop/go/internal/router"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC976_001_EnvelopeCeilingClampsThroughRealSeam(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestModelTierEnvelope_CeilingClampsThroughRealProfileLookup")
	if !ok {
		t.Errorf("model-tier-envelope ceiling does not clamp through the real profileForModelRouting seam — the nil-stub is still disabling the guard in production:\n%s", out)
	}
}

func TestC976_002_UniversalFloorFiresInRealDispatch(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestModelTierEnvelope_UniversalFloorClampsThroughRealDispatch")
	if !ok {
		t.Errorf("the documented universal envelope floor does not fire in the real dispatch path — it is still gated behind the nil-profile stub:\n%s", out)
	}
}

func TestC976_003_ClampIsPreciseAndNilSafe(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestModelTierEnvelope_WithinEnvelopeTierPassesThrough|TestModelTierEnvelope_AbsentProfileDegradesNilSafe|TestModelTierEnvelope_ClampRecordedInPhasePlan")
	if !ok {
		t.Errorf("envelope wiring is imprecise (over-clamps a legal tier), nil-unsafe (panics/errors on a profile-less phase), or does not record the clamp to phase-plan.json:\n%s", out)
	}
}

func TestC976_004_RouterUnitGuardUnchanged(t *testing.T) {
	ok, out := runGoTest(t, routerPkg, "TestClampPlanModelRouting.*|.*ModelRouting.*Envelope.*|.*UniversalFloor.*")
	if !ok {
		t.Errorf("the router-level model-tier-envelope guard suite regressed — the core wiring must not alter router guard logic:\n%s", out)
	}
}

func TestC976_005_TreeBuildsNoImportCycle(t *testing.T) {
	_, stderr, code, err := acsassert.SubprocessOutput("go", "build", "./...")
	if code < 0 {
		t.Fatalf("go build failed to launch: code=%d err=%v\n%s", code, err, stderr)
	}
	if code != 0 {
		t.Errorf("go build ./... failed (exit=%d) — the profile-lookup wiring must not introduce an import cycle:\n%s", code, stderr)
	}
}
