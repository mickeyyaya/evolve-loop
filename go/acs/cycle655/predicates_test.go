//go:build acs

package cycle655

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

func TestC655_001_OutOfLaneBuildBlocked(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "-run", "TestTopNBindingGate|TestNewReviewer_Enforce", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-1: the out-of-lane build->audit block suite must PASS; got exit=%d\n%s", code, out)
	}
}

func TestC655_002_BuilderPromptNamesTopNSoleAuthority(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestBuilderPromptNamesTopNAsSoleTaskAuthority", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-2: builder-prompt task-authority suite must PASS; got exit=%d\n%s", code, out)
	}
}

func TestC655_003_ReplayCycle640ShapeBlocksBeforeAudit(t *testing.T) {
	out, code := goTest(t, "-count=1", "-run", "TestReplayCycle640Shape", "./internal/topngate/")
	if code != 0 {
		t.Errorf("AC-3: cycle-640-replay regression must PASS (blocked before audit); got exit=%d\n%s", code, out)
	}
}

func TestC655_004_TouchedPackageRaceClean(t *testing.T) {
	out, code := goTest(t, "-race", "-count=1", "./internal/topngate/...")
	if code != 0 {
		t.Errorf("AC-4(race): `go test -race ./internal/topngate/...` must PASS; got exit=%d\n%s", code, out)
	}
	if strings.Contains(out, "DATA RACE") {
		t.Errorf("AC-4(race): race detector flagged a data race:\n%s", out)
	}
}

func TestC655_005_TopngateApicoverGraduated(t *testing.T) {
	root := acsassert.RepoRoot(t)

	// acs-predicate: config-check — membership in .apicover-enforce is an
	enforcePath := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.FileContains(t, enforcePath, "./internal/topngate") {
		t.Errorf("AC-4(apicover-a): go/.apicover-enforce must list `./internal/topngate` (graduate the new package into the enforce SSOT)")
	}

	namedTest := filepath.Join(root, "go", "internal", "topngate", "apicover_named_test.go")
	if !acsassert.FileExists(t, namedTest) {
		t.Errorf("AC-4(apicover-b): go/internal/topngate/apicover_named_test.go must exist (public-API DoD)")
	} else if !acsassert.FileContains(t, namedTest, "NewReviewer") {
		t.Errorf("AC-4(apicover-b): apicover_named_test.go must name NewReviewer (topngate's only exported symbol) by identifier")
	}

	out, code := goTest(t, "-tags", "acs", "-count=1", "-run", "TestApicoverEnforce_CoversEveryInternalPackage", "./acs/regression/apicover/")
	if code != 0 {
		t.Errorf("AC-4(apicover-c): repo-wide apicover completeness gate must PASS (topngate graduated, no stale entries); got exit=%d\n%s", code, out)
	}
}
