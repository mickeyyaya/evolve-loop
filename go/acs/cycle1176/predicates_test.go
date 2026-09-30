//go:build acs

package cycle1176

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	waveCycle    = 1176
)

func laneFixture(t *testing.T, seed int) string {
	t.Helper()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	committed := map[string]any{"id": "committed-task", "title": "committed", "failure_count": seed}
	menu := map[string]any{"id": "menu-task", "title": "menu"}
	for name, body := range map[string]map[string]any{"committed-task.json": committed, "menu-task.json": menu} {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(inbox, name), raw, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func failLane(t *testing.T, root string, ceiling int, systemLevel bool) inboxmover.OutcomeResult {
	t.Helper()
	res, err := inboxmover.ApplyCycleOutcome(
		inboxmover.Options{ProjectRoot: root, Stderr: io.Discard},
		inboxmover.CycleOutcome{
			Cycle:        waveCycle,
			Passed:       false,
			CommittedIDs: []string{"committed-task"},
			Reason:       "cycle-failure-release",
			Ceiling:      ceiling,
			SystemLevel:  systemLevel,
		})
	if err != nil {
		t.Fatalf("ApplyCycleOutcome(FAIL): %v", err)
	}
	return res
}

func failureCountOf(t *testing.T, root, id string) (int, string) {
	t.Helper()
	inbox := filepath.Join(root, ".evolve", "inbox")
	for _, loc := range []string{"", "quarantine", filepath.Join("processing", "cycle-1176")} {
		path := filepath.Join(inbox, loc, id+".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m struct {
			FailureCount int `json:"failure_count"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		if loc == "" {
			loc = "root"
		}
		return m.FailureCount, loc
	}
	return 0, ""
}

func TestC1176_001_WaveLaneFailBumpsUnclaimedCommittedID(t *testing.T) {
	root := laneFixture(t, 0)

	failLane(t, root, 3, false)

	count, loc := failureCountOf(t, root, "committed-task")
	if loc == "" {
		t.Fatalf("committed-task vanished from the inbox after a FAIL — a lane's scope must never be stranded")
	}
	if count != 1 {
		t.Errorf("committed-task failure_count = %d after one FAIL, want 1 (loc=%s) — the drain found no claim to bump, so the S5 ceiling stays unreachable for wave lanes", count, loc)
	}
	if loc != "root" {
		t.Errorf("committed-task landed in %s below the ceiling, want the inbox root so the next triage can re-pick it", loc)
	}
}

func TestC1176_002_UncommittedMenuIDNeitherBumpsNorMoves(t *testing.T) {
	root := laneFixture(t, 0)

	failLane(t, root, 3, false)

	count, loc := failureCountOf(t, root, "menu-task")
	if loc != "root" {
		t.Errorf("uncommitted menu-task moved to %q on a FAIL it had no part in, want it left at the inbox root", loc)
	}
	if count != 0 {
		t.Errorf("uncommitted menu-task failure_count = %d, want 0 — only ids triage COMMITTED may accrue task-level failures", count)
	}
}

func TestC1176_003_CommittedIDQuarantinesAtCeiling(t *testing.T) {
	root := laneFixture(t, 2)

	res := failLane(t, root, 3, false)

	count, loc := failureCountOf(t, root, "committed-task")
	if loc != "quarantine" {
		t.Errorf("committed-task is at %q with failure_count=%d after reaching ceiling 3, want \"quarantine\" — a poison todo must stop being re-picked (result=%+v)", loc, count, res)
	}
	if count < 3 {
		t.Errorf("committed-task failure_count = %d at quarantine time, want >= ceiling 3", count)
	}
	if len(res.Quarantined) == 0 {
		t.Errorf("OutcomeResult.Quarantined is empty — the seam must REPORT the parking, not just perform it")
	}
	if len(res.Promoted) != 0 {
		t.Errorf("OutcomeResult.Promoted = %v on a FAIL, want empty — nothing is ever promoted to processed/ on a failure", res.Promoted)
	}
}

func TestC1176_004_SystemLevelFailureNeverBumps(t *testing.T) {
	root := laneFixture(t, 2)

	failLane(t, root, 3, true)

	count, loc := failureCountOf(t, root, "committed-task")
	if loc == "quarantine" {
		t.Errorf("a SYSTEM-level failure quarantined committed-task — the S3 halt path takes precedence, S5 must not park it")
	}
	if count != 2 {
		t.Errorf("committed-task failure_count = %d after a system-level FAIL, want the seeded 2 (a storm must not walk a task toward the ceiling)", count)
	}
}

func requirePassing(t *testing.T, pkg string, want []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-v", "-count=1", "-run", "^("+strings.Join(want, "|")+")$", pkg)
	out := stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", pkg, code, err, out)
	}
	for _, name := range want {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("%s: no `--- PASS: %s` line — the test is failing, renamed, or gone:\n%s", pkg, name, out)
		}
	}
	if code != 0 {
		t.Errorf("go test %s exited %d:\n%s", pkg, code, out)
	}
}

func TestC1176_005_GCWorkspaceSweepHasOperatorSurface(t *testing.T) {
	requirePassing(t, cmdEvolvePkg, []string{
		"TestRunGC_DryRunPrintsWorkspacePlanAndMutatesNothing",
		"TestRunGC_ExplicitRunAppliesWorkspaceSweep",
		"TestRunGC_ExplicitRunPreservesUnmergedBranch",
	})
}

func TestC1176_006_WavePlanSeedDropsConsumedScope(t *testing.T) {
	requirePassing(t, cmdEvolvePkg, []string{
		"TestProductionWavePlanFn_PrunesConsumedScopeFromPriorDecision",
		"TestProductionWavePlanFn_KeepsPendingAndUnknownScopes",
		"TestProductionWavePlanFn_AllConsumedStillPlansLiveWork",
	})
}

func TestC1176_007_CommittedIDsReaderFeedsTheFailSite(t *testing.T) {
	requirePassing(t, cmdEvolvePkg, []string{
		"TestFailedCycleCommittedIDs_ReadsTopNAndSkipShipped",
		"TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped",
		"TestFailedCycleCommittedIDs_AbsentOrCorruptDecisionIsNil",
	})
}
