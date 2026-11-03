//go:build acs

package cycle2142

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const adapterFile = "audit_repair_convergence.go"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamed(t *testing.T, pkg, pattern string, extra ...string) string {
	t.Helper()
	args := append([]string{"-C", goDir(t), "test", "-count=1", "-v", "-run", pattern}, extra...)
	args = append(args, pkg)
	out, errOut, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil || code != 0 {
		t.Fatalf("go test %s -run %s exit=%d: %v\n%s", pkg, pattern, code, err, acsassert.OutputTail(out+errOut, 40))
	}
	return out
}

func requirePasses(t *testing.T, out string, atLeast int) {
	t.Helper()
	if n := strings.Count(out, "--- PASS: TestAuditRepairConvergence_"); n < atLeast {
		t.Fatalf("only %d AuditRepairConvergence tests passed, want at least %d\n%s", n, atLeast, acsassert.OutputTail(out, 30))
	}
	if strings.Contains(out, "--- FAIL") || strings.Contains(out, "--- SKIP") {
		t.Fatalf("a test failed or skipped\n%s", acsassert.OutputTail(out, 30))
	}
}

func TestC2142_001_ReplayOfTwoRoundsRaisesTheJudgeAndFreshensTheFixer(t *testing.T) {
	out := runNamed(t, "./internal/core", "TestAuditRepairConvergence_(Replay|Judgments|Input)")
	requirePasses(t, out, 5)
}

func TestC2142_002_ShadowRecordsTheRungAndLeavesTheRepairFlowAlone(t *testing.T) {
	out := runNamed(t, "./internal/core", "TestAuditRepairConvergence_(Shadow|AnUnresolved|ARepairWithNo)")
	requirePasses(t, out, 4)
}

func TestC2142_003_EnforceFollowsTheDecisionAndNeverLiftsAnEnvelopeDecline(t *testing.T) {
	out := runNamed(t, "./internal/core", "TestAuditRepairConvergence_Enforce")
	requirePasses(t, out, 3)
}

func TestC2142_004_PolicyCarriesTheConvergenceStageIntoTheWorkflowConfig(t *testing.T) {
	out := runNamed(t, "./internal/core", "TestAuditRepairConvergence_PolicyCarries")
	requirePasses(t, out, 1)
}

var coverLine = regexp.MustCompile(`audit_repair_convergence\.go:\d+:\s+(\w+)\s+(\d+\.\d)%`)

func TestC2142_005_EveryFunctionOfTheAdapterIsFullyCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	runNamed(t, "./internal/core", "TestAuditRepairConvergence_", "-coverprofile="+profile)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir(t), "tool", "cover", "-func="+profile)
	if err != nil || code != 0 {
		t.Fatalf("go tool cover exit=%d: %v\n%s", code, err, errOut)
	}
	rows := coverLine.FindAllStringSubmatch(out, -1)
	if len(rows) < 3 {
		t.Fatalf("%s lists %d functions in the cover profile, want at least 3 (judgments, input, mapping)\n%s", adapterFile, len(rows), acsassert.OutputTail(out, 20))
	}
	for _, r := range rows {
		if r[2] != "100.0" {
			t.Errorf("%s is covered %s%%, want 100.0%%", r[1], r[2])
		}
	}
}
