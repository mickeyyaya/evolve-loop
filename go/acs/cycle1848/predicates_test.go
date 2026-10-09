//go:build acs

package cycle1848

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runAuditTests(t *testing.T, names ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", "./internal/phases/audit")
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the audit skills-drift tests fail: %v\n%s", err, out)
	}
	for _, name := range names {
		if !strings.Contains(string(out), "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass\n%s", name, out)
		}
	}
	return string(out)
}

func TestC1848_001_Exit2DriftReportAtTheDeadlineIsAFail(t *testing.T) {
	runAuditTests(t,
		"TestWorktreeSkillsDrift_Exit2AtDeadlineWithDriftIsOffender",
		"TestWorktreeSkillsDrift_DeadlineWithoutADriftReportStaysUngraded",
		"TestWorktreeSkillsDrift_ATimeoutCannotGradeAndSaysSo",
	)
}

func TestC1848_002_CancellingThePhaseContextStopsTheWorktreeRun(t *testing.T) {
	runAuditTests(t,
		"TestWorktreeSkillsDrift_ContextCancellationStopsRun",
		"TestWorktreeSkillsDrift_CancelledPhaseIsNotADeadline",
	)
}

func TestC1848_003_MissingGoOnPathIsANamedSkipWarning(t *testing.T) {
	runAuditTests(t, "TestWorktreeSkillsDrift_MissingGoBinaryWarns")
}

func TestC1848_004_ExistingSkillsDriftOutcomesAreUnchanged(t *testing.T) {
	runAuditTests(t,
		"TestWorktreeSkillsDrift_Outcomes",
		"TestWorktreeSkillsDrift_RunsTheWorktreeGeneratorCheck",
		"TestWorktreeSkillsDrift_BoundsTheRunWithTheGateLimit",
		"TestSkillsCheckOffenders",
	)
}
