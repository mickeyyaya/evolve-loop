//go:build acs

package cycle561

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	runleasePkg = "github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	swarmPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	auditPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	if err != nil {
		t.Fatalf("go test failed to launch for %s (%s): %v\nstderr:\n%s", pkg, pattern, err, stderr)
	}
	return code == 0, stdout + stderr
}

func TestC561_001_S1_PidAwareLeaseStaleness(t *testing.T) {
	ok, out := runGoTest(t, runleasePkg,
		"TestOwnerLive_DeadPidFreshHeartbeatNotLive|TestOwnerLive_FreshAliveOwnerIsLive|TestOwnerLive_StaleAliveOwnerNotLive|TestOwnerLive_NilAlive|TestOwnerLive_Pid0")
	if !ok {
		t.Errorf("runlease.OwnerLive pid-aware-staleness suite is not green — S1 lease liveness regressed:\n%s", out)
	}
}

func TestC561_002_S1_SealFenceConsumesPidAlive(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestSealCycle_DeadOwnerFreshLease.*|TestSealCycle_LiveOwnerFreshLease.*")
	if !ok {
		t.Errorf("SealCycle pid-fence suite is not green — S1 seal fence regressed:\n%s", out)
	}
}

func TestC561_003_AuditorEGPS_NormalCompletionShipEligibleGate(t *testing.T) {
	ok, out := runGoTest(t, auditPkg,
		"TestRun_NormalCompletion_PassReport_ShipEligibleFalse_RejectsAsUnreconciled|TestRun_NormalCompletion_PassReport_ShipEligibleTrue_StaysPass|TestRun_NormalCompletion_PassReport_ShipEligibleAbsent_StaysPass|TestRun_NormalCompletion_PassReport_RedCountPositive_StaysFail")
	if !ok {
		t.Errorf("audit normal-completion ship_eligible reconciliation gate is not green — a narrative PASS with ship_eligible:false still ships (or the fix broke genuine-agreement/back-compat):\n%s", out)
	}
}

func TestC561_004_S3_InlineBranchDeleteNonForce(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestCleanup_DeletesMergedCycleBranch|TestCleanup_UnmergedBranchSurvives_WarnsOnly")
	if !ok {
		t.Errorf("core Cleanup branch-delete suite is not green — S3 in-cycle branch deletion regressed:\n%s", out)
	}
	ok2, out2 := runGoTest(t, swarmPkg,
		"TestCleanup_RoutesGitBranchDeleteThroughSeam|TestCleanup_UnmergedBranch_NeverForceDeletes")
	if !ok2 {
		t.Errorf("swarm Cleanup branch-delete mirror is not green — S3 swarm branch deletion regressed:\n%s", out2)
	}
}
