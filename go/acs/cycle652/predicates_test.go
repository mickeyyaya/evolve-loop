//go:build acs

package cycle652

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	full := append([]string{"test"}, args...)
	cmd := exec.Command("go", full...)
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("go test failed to run: %v\n%s", err, out)
	return string(out), -1
}

func TestC652_001_OutOfLaneBuildAdvisory(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "-run", "TestTopNBindingGate|TestNewReviewer_Enforce", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-1 (advisory contract): the label-drift suite must PASS; got exit=%d\n%s", code, out)
	}
}

func TestC652_002_BuilderPromptNamesTopNSoleAuthority(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestBuilderPromptNamesTopNAsSoleTaskAuthority", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-2: builder-prompt task-authority suite must PASS; got exit=%d\n%s", code, out)
	}
}

func TestC652_003_ReplayCycle640ShapeAdvisory(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestReplayCycle640Shape", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-3: cycle-640-replay (advisory contract) must PASS; got exit=%d\n%s", code, out)
	}
}

func TestC652_004_TouchedPackageRaceClean(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "./internal/topngate/...")
	if code != 0 {
		t.Errorf("AC-4: `go test -race ./internal/topngate/...` must PASS; got exit=%d\n%s", code, out)
	}
	if strings.Contains(out, "DATA RACE") {
		t.Errorf("AC-4: race detector flagged a data race:\n%s", out)
	}
}
