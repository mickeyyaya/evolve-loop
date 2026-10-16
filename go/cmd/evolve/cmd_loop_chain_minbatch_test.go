package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestChainStartDecision_MinOneBatchOnDrainedInbox(t *testing.T) {
	t.Parallel()
	reason, stop := chainStartDecision(0, 20, 0, false)
	if stop || reason != "" {
		t.Fatalf("chainStartDecision(0,20,0,false) = (%q,%v), want (\"\",false): "+
			"chain mode must run at least one batch, not exit having done nothing", reason, stop)
	}
}

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
