//go:build acs

package cycle1335

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	windowEvalSlug  = "batch-window-allocation-lease-anchor"
	consumeEvalSlug = "consumed-inbox-fingerprint-ack-projection"
	cmdPkg          = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func requireNamedPass(t *testing.T, pkg, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if code != 0 || err != nil {
		t.Logf("go test -run ^%s$ %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, pkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("test %s did not report PASS in %s (missing, unimplemented, or regressed)", name, pkg)
	}
}

func requireEvalQuality(t *testing.T, slug string) {
	t.Helper()
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", slug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s has %d classifiable command(s), want >=2 (vacuous-empty-eval guard)", evalPath, len(res.Commands))
	}
}

func TestC1335_001_batch_window_floor_prefers_allocation_lease(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestReadBatchWindowFloor_PrefersAllocationLease")
}

func TestC1335_002_batch_window_floor_legacy_state_fallback(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestReadBatchWindowFloor_LegacyStateFallsBackToCompletionCounter")
}

func TestC1335_003_read_last_cycle_number_unchanged(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestReadLastCycleNumber_StillReportsCompletionCounter")
}

func TestC1335_004_run_loop_aborted_digests_outside_window(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestRunLoop_AbortedCycleDigestsFallOutsideBatchWindow")
}

func TestC1335_005_run_loop_in_batch_digests_still_halt(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestRunLoop_InBatchDigestsStillHalt")
}

func TestC1335_006_breaker_reconciles_already_consumed_item(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestBlockerBreakerHalt_ReconcilesAlreadyConsumedItem")
}

func TestC1335_007_breaker_reconciles_item_with_no_kind(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestBlockerBreakerHalt_ReconcilesItemWithNoKindFromNotes")
}

func TestC1335_008_reconcile_does_not_weaken_rule_b(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestBlockerBreakerHalt_ReconcileDoesNotWeakenRuleB")
}

func TestC1335_009_reconcile_survives_unreadable_item(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestBlockerBreakerHalt_ReconcileSurvivesUnreadableItem")
}

func TestC1335_010_reconcile_no_consumed_dir_is_quiet(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestBlockerBreakerHalt_NoConsumedDirIsQuiet")
}

func TestC1335_011_inbox_consume_moves_and_acks(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestRunInbox_Consume_MovesItemAndAcksFingerprint")
}

func TestC1335_012_inbox_consume_without_fingerprint_still_moves(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestRunInbox_Consume_ItemWithoutFingerprintStillMoves")
}

func TestC1335_013_inbox_consume_missing_item(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestRunInbox_Consume_MissingItemReturnsNonZero")
}

func TestC1335_014_inbox_consume_no_arg_usage(t *testing.T) {
	requireNamedPass(t, cmdPkg, "TestRunInbox_Consume_NoArgReturnsUsage")
}

func TestC1335_015_window_eval_passes_quality_check(t *testing.T) {
	requireEvalQuality(t, windowEvalSlug)
}

func TestC1335_016_consume_eval_passes_quality_check(t *testing.T) {
	requireEvalQuality(t, consumeEvalSlug)
}
