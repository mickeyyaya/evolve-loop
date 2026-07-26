package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// cmd_loop_chain_minbatch_test.go — cycle-1098 RED contract tests for
// chain-min-one-batch: `--until-inbox-empty` against an ALREADY-drained inbox
// currently returns rc=0 having run ZERO cycles, silently weaker than the
// pre-chain contract where `evolve loop` always ran one batch. Chaining was
// specified as "keep going past the boundary", never as "may run nothing at
// all". The drained-inbox check is a CONTINUE condition mis-sited as a START
// condition.
//
// These tests are the TDD contract: Builder makes them pass by changing
// production code only. The helpers (chainTestEnv/stubBatches/runChain) are
// reused from cmd_loop_chain_test.go — same package, no duplicate fixtures.

// TestChainStartDecision_MinOneBatchOnDrainedInbox — AC1.1 (the crux). At n==0
// with nothing pending and no brake, the chain must START: a drained inbox is a
// CONTINUE condition ("we finished the work"), not a START condition ("there
// was never any work"), and the pre-chain single-batch contract must survive
// opting into chaining.
func TestChainStartDecision_MinOneBatchOnDrainedInbox(t *testing.T) {
	t.Parallel()
	reason, stop := chainStartDecision(0, 20, 0, false)
	if stop || reason != "" {
		t.Fatalf("chainStartDecision(0,20,0,false) = (%q,%v), want (\"\",false): "+
			"chain mode must run at least one batch, not exit having done nothing", reason, stop)
	}
}

// TestChainStartDecision_BrakeStillWinsAtZeroBatches — AC1.2 (negative /
// precedence). The min-one-batch allowance must NOT outrank the operator brake:
// `.evolve/loop-stop` present at n==0 still means zero batches. An explicit
// operator instruction is never overridden by a contract default.
func TestChainStartDecision_BrakeStillWinsAtZeroBatches(t *testing.T) {
	t.Parallel()
	for _, pending := range []int{0, 5} {
		reason, stop := chainStartDecision(0, 20, pending, true)
		if !stop || reason != "chain_operator_brake" {
			t.Fatalf("chainStartDecision(0,20,%d,true) = (%q,%v), want (chain_operator_brake,true): "+
				"the operator brake outranks the min-one-batch allowance", pending, reason, stop)
		}
	}
}

// TestChainStartDecision_DrainedInboxStopsAfterFirstBatch — AC1.4 (scope guard,
// anti-no-op). The allowance is scoped to n==0: once a batch HAS run, a drained
// inbox is still the clean success exit. A blanket "never stop on empty" would
// pass AC1.1 and fail here.
func TestChainStartDecision_DrainedInboxStopsAfterFirstBatch(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 7} {
		reason, stop := chainStartDecision(n, 20, 0, false)
		if !stop || reason != "chain_inbox_empty" {
			t.Fatalf("chainStartDecision(%d,20,0,false) = (%q,%v), want (chain_inbox_empty,true): "+
				"after >=1 batch a drained inbox is still the clean exit", n, reason, stop)
		}
	}
}

// TestChainStartDecision_ZeroCapNeverRunsABatch — AC1.3 (edge / OOD). The cap
// stays an EXACT ceiling: a non-positive cap must not be widened to one batch by
// the new allowance (no cap+1), and the existing exact-cap behaviour is
// unchanged for positive caps.
func TestChainStartDecision_ZeroCapNeverRunsABatch(t *testing.T) {
	t.Parallel()
	if reason, stop := chainStartDecision(0, 0, 0, false); !stop || reason != "chain_max_batches" {
		t.Fatalf("chainStartDecision(0,0,0,false) = (%q,%v), want (chain_max_batches,true): "+
			"the cap is an exact ceiling — min-one-batch must never produce cap+1", reason, stop)
	}
	if reason, stop := chainStartDecision(3, 3, 5, false); !stop || reason != "chain_max_batches" {
		t.Fatalf("chainStartDecision(3,3,5,false) = (%q,%v), want (chain_max_batches,true)", reason, stop)
	}
}

// TestRunLoopChain_DrainedInboxRunsExactlyOneBatch — AC1.1 end-to-end. The
// operator-visible contract: exactly ONE batch, rc=0, and the drained-inbox stop
// reason recorded in the chain summary.
func TestRunLoopChain_DrainedInboxRunsExactlyOneBatch(t *testing.T) {
	cfg := chainTestEnv(t, 0, "")
	seen := stubBatches(t, func(int, loopConfig) int { return 0 })

	rc, res, stderr := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(*seen) != 1 {
		t.Fatalf("chain ran %d batches against a drained inbox, want exactly 1 "+
			"(the pre-chain contract always ran one batch); stderr=%s", len(*seen), stderr)
	}
	if rc != 0 || res.StopReason != "chain_inbox_empty" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_inbox_empty", rc, res.StopReason)
	}
	if len(res.Batches) != 1 || res.Batches[0].Batch != 1 {
		t.Errorf("chain summary must record the one batch that ran, got %+v", res.Batches)
	}
}

// TestRunLoopChain_PreEngagedBrakeRunsZeroBatchesOnDrainedInbox — AC1.2
// end-to-end negative. A brake file already on disk at launch means ZERO
// batches, even though the min-one-batch allowance would otherwise fire.
func TestRunLoopChain_PreEngagedBrakeRunsZeroBatchesOnDrainedInbox(t *testing.T) {
	cfg := chainTestEnv(t, 0, "")
	if err := os.WriteFile(filepath.Join(cfg.EvolveDir, chainBrakeFile), nil, 0o644); err != nil {
		t.Fatalf("engage brake: %v", err)
	}
	seen := stubBatches(t, func(int, loopConfig) int { return 0 })

	rc, res, _ := runChain(t, cfg, policy.ChainConfig{Enabled: true, MaxBatches: 10})

	if len(*seen) != 0 {
		t.Fatalf("a pre-engaged operator brake must run ZERO batches, ran %d", len(*seen))
	}
	if rc != 0 || res.StopReason != "chain_operator_brake" {
		t.Errorf("got rc=%d reason=%q, want rc=0 chain_operator_brake", rc, res.StopReason)
	}
}
