//go:build acs

package cycle566

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	profilesPkg = "github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC566_001_EffortManifestParityPerCLI(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEffortRealize_Matrix")
	if !ok {
		t.Errorf("effort not realized per-manifest (claude/codex must translate, agy/ollama must noop):\n%s", out)
	}
}

func TestC566_002_EffortDefaultMatrix(t *testing.T) {
	ok, out := runGoTest(t, profilesPkg, "TestEffortDefaults_Matrix")
	if !ok {
		t.Errorf("per-phase effort default matrix not pinned in config (scout/triage=low, tdd/audit/adversarial=medium, builder=medium):\n%s", out)
	}
}

func TestC566_003_EffortAbsentByteIdentical(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg, "TestEffortRealize_AbsentByteIdentical")
	if !ok {
		t.Errorf("unset effort is not byte-identical to a pre-effort manifest (additive-only regression guard):\n%s", out)
	}
}

func TestC566_004_TouchedPackagesVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", bridgePkg, profilesPkg)
	if code < 0 {
		t.Fatalf("go vet failed to launch: code=%d err=%v\nstderr:\n%s", code, err, stderr)
	}
	if code != 0 {
		t.Errorf("go vet not clean on touched packages (bridge, profiles):\n%s\n%s", stdout, stderr)
	}
}
