//go:build acs

package cycle1939

import (
	"os/exec"
	"strings"
	"testing"
)

const (
	sharedFunc = "coverage.func.txt"
	scopedFunc = "coverage.apicover.func.txt"
)

func dryRun(t *testing.T, target string) string {
	t.Helper()
	cmd := exec.Command("make", "-n", "-f", "Makefile", target)
	cmd.Dir = "../.."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n %s: %v\n%s", target, err, out)
	}
	return string(out)
}

func assertScoped(t *testing.T, target string) {
	t.Helper()
	plan := dryRun(t, target)
	if strings.Contains(plan, sharedFunc) {
		t.Errorf("RED: make %s still touches the shared %s:\n%s", target, sharedFunc, plan)
	}
	if !strings.Contains(plan, scopedFunc) {
		t.Errorf("RED: make %s does not use the scoped %s:\n%s", target, scopedFunc, plan)
	}
}

func TestC1939_001_ApicoverWritesScopedFuncReport(t *testing.T) {
	assertScoped(t, "apicover")
}

func TestC1939_002_ApicoverCheckReadsScopedFuncReport(t *testing.T) {
	assertScoped(t, "apicover-check")
}

func TestC1939_003_ApicoverKeepsScopedProfile(t *testing.T) {
	plan := dryRun(t, "apicover")
	if !strings.Contains(plan, "-coverprofile=coverage.apicover.txt") {
		t.Errorf("apicover must keep its scoped profile coverage.apicover.txt:\n%s", plan)
	}
}

func TestC1939_004_CoverTargetKeepsSharedFuncReport(t *testing.T) {
	plan := dryRun(t, "cover")
	if !strings.Contains(plan, sharedFunc) {
		t.Errorf("make cover must keep writing the shared %s (scoping is apicover-only):\n%s", sharedFunc, plan)
	}
	if strings.Contains(plan, scopedFunc) {
		t.Errorf("make cover must not use the apicover-scoped %s:\n%s", scopedFunc, plan)
	}
}
