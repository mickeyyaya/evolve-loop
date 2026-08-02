//go:build acs

// Package cycle1176 materialises the cycle-1176 acceptance criteria for the
// three fleet-scoped ids pinned to this lane:
//
//   - wave-lane-task-quarantine-dead          → predicates 001-004
//   - workspace-hygiene-s5-wiring-shadow-default → predicate 005
//   - wave-planner-pass-scope-prune           → predicate 006
//
// STATE NOTE (read this before treating a GREEN here as a rubber stamp). All
// three implementations are already resident in this worktree's base: the
// lifecycle seam (inboxmover.ApplyCycleOutcome + ClaimLaneScope, wired at the
// production FAIL site cmd_loop.go:728 and the PASS site postship.go:188)
// landed in 4e523438 (#372), and the gc workspace sweep + wave-plan scope prune
// landed in the salvage snapshot f2d87339. Those cycles' own predicates
// (acs/cycle1156, acs/cycle1172) are CYCLE-SCOPED — they do not run again — so
// this package re-pins the same behaviour under this cycle's scope rather than
// asserting a fresh RED. See test-report.md for the pre-existing-GREEN
// disposition and the "consumed scope was re-picked" finding it raises.
//
// Predicate strategy — every predicate exercises the system under test, never
// greps production source (the cycle-85 degenerate-predicate ban):
//
//   - 001-004 drive inboxmover.ApplyCycleOutcome over a real temp-dir inbox in
//     the exact WAVE-LANE shape the defect described (committed ids that were
//     never claimed into processing/) and assert on the resulting filesystem
//     state: durable failure_count, release destination, quarantine parking.
//   - 005/006 shell the landed unit suites for the CLI-surface and planner
//     halves. `go test -run` exits 0 when its pattern matches NOTHING, so these
//     run with -v and require an explicit `--- PASS: <name>` line per expected
//     test — a renamed or deleted test reads as FAIL here, never a silent pass.
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
	// waveCycle is the fixture cycle number; any value works, it only names
	// the processing/cycle-<N>/ subdir the drain walks.
	waveCycle = 1176
)

// laneFixture builds the wave-lane shape: an inbox ROOT holding one committed
// item and one uncommitted menu item, with NOTHING in processing/cycle-N/ —
// precisely the state in which the pre-fix drain found nothing to bump. seed is
// the committed item's starting failure_count. Returns the project root.
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

// failLane applies a FAILED cycle outcome for the committed id only.
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

// failureCountOf reads the durable counter off an item, searching the inbox
// root, then quarantine/, then processing/cycle-N/. Returns (count, location).
// location is "" when the id is nowhere — a stranded item, itself a failure.
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

// TestC1176_001_WaveLaneFailBumpsUnclaimedCommittedID is the crux predicate for
// wave-lane-task-quarantine-dead: a lane that never claimed its scope into
// processing/ must STILL accrue a task-level failure on its committed id. This
// is the exact batch-14 shape in which failure_count stayed at 0 across four
// FAILs and the ADR-0072 S5 ceiling was structurally unreachable.
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

// TestC1176_002_UncommittedMenuIDNeitherBumpsNorMoves is the NEGATIVE half —
// menu semantics (PR #366). No phase worked the uncommitted id, so it must not
// accrue a task-level failure. Without this, N failures of an unrelated task
// walk the whole menu to the quarantine ceiling and a healthy backlog is parked.
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

// TestC1176_003_CommittedIDQuarantinesAtCeiling proves the ceiling is now
// REACHABLE for a wave lane: an id already carrying ceiling-1 failures bumps to
// the ceiling and is parked in quarantine/ instead of returning to the root to
// be re-picked forever. This is the behaviour the dead code path was supposed
// to deliver and never did.
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

// TestC1176_004_SystemLevelFailureNeverBumps is the second NEGATIVE (ADR-0072
// S3 precedence, AC4). A quota/infra storm is not the task's fault: it must
// neither bump nor quarantine, or one later task-level FAIL parks a backlog
// that never failed on its own merits.
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

// requirePassing shells `go test -v -run '^(<pattern>)$' -count=1 <pkg>` and
// requires an explicit `--- PASS: <name>` line for every name in want.
//
// The -v + per-name check is load-bearing, not belt-and-braces: `go test -run`
// exits 0 when its pattern matches no test at all, so an exit-code-only
// predicate would report GREEN for a suite that was renamed away. -count=1
// defeats the test cache so current source is always exercised. code < 0 is a
// launch failure (toolchain missing/killed), never a verdict, so it is fatal.
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

// TestC1176_005_GCWorkspaceSweepHasOperatorSurface pins
// workspace-hygiene-s5-wiring-shadow-default's remaining scope: `evolve gc`
// gained the workspace (worktree/branch) sweep with --dry-run parity, the
// documented enforce/shadow asymmetry (an explicit operator run APPLIES), and
// the refusal that makes it safe — an UNMERGED cycle-* branch is flagged, never
// deleted. All three run through runGC itself over a real git fixture.
func TestC1176_005_GCWorkspaceSweepHasOperatorSurface(t *testing.T) {
	requirePassing(t, cmdEvolvePkg, []string{
		"TestRunGC_DryRunPrintsWorkspacePlanAndMutatesNothing",
		"TestRunGC_ExplicitRunAppliesWorkspaceSweep",
		"TestRunGC_ExplicitRunPreservesUnmergedBranch",
	})
}

// TestC1176_006_WavePlanSeedDropsConsumedScope pins
// wave-planner-pass-scope-prune: the consumed-state filter runs at wave-PLAN
// seed time, so lane-scope.json never records an id already resolved to
// processed/ (cycle-1116), while pending/unknown scopes survive (the anti-no-op
// negative: a planner that pruned everything would pass a prune-only check) and
// an all-consumed prior decision still plans live work rather than an empty wave.
func TestC1176_006_WavePlanSeedDropsConsumedScope(t *testing.T) {
	requirePassing(t, cmdEvolvePkg, []string{
		"TestProductionWavePlanFn_PrunesConsumedScopeFromPriorDecision",
		"TestProductionWavePlanFn_KeepsPendingAndUnknownScopes",
		"TestProductionWavePlanFn_AllConsumedStillPlansLiveWork",
	})
}

// TestC1176_007_CommittedIDsReaderFeedsTheFailSite covers the hinge between the
// production FAIL site and the seam: failedCycleCommittedIDs reads the failed
// cycle's triage-decision.json into CycleOutcome.CommittedIDs. Return the wrong
// set and 001-004 above are decided on the wrong ids — an empty set bumps
// nothing (the original defect), the whole menu bumps everything. New coverage
// this cycle: the reader had none.
func TestC1176_007_CommittedIDsReaderFeedsTheFailSite(t *testing.T) {
	requirePassing(t, cmdEvolvePkg, []string{
		"TestFailedCycleCommittedIDs_ReadsTopNAndSkipShipped",
		"TestFailedCycleCommittedIDs_ExcludesDeferredAndDropped",
		"TestFailedCycleCommittedIDs_AbsentOrCorruptDecisionIsNil",
	})
}
