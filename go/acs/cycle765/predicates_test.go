//go:build acs

package cycle765

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func runGoTest(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			pkg, name, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("go test reported no PASS for %s (renamed or not run?)\nstdout:\n%s", name, stdout)
	}
}

func TestC765_001_ContentionBudgetScalesWithFleetWidth(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_ContentionBudgetScalesWithFleetWidth")
}

func TestC765_002_JitteredBackoffBetweenReaudits(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_JitteredBackoffBetweenReaudits")
}

func TestC765_003_NonContentionTransientKeepsConstantBudget(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_NonContentionTransientKeepsConstantBudget")
}

func TestC765_004_BudgetClassifierScalesOnlyContentionCodes(t *testing.T) {
	runGoTest(t, corePkg, "TestShipRecovery_BudgetClassifierScalesOnlyContentionCodes")
}
