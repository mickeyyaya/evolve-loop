package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// blockerBreakerHalt evaluates the batch's failure digests (cycles after
// batchStartCycle) against the policy ceilings. halted=false means continue
// dispatching; on halt it writes the escalation + P0 item and returns the
// loop exit code. Policy reloads fresh on every check; a missing or
// malformed policy falls back to compiled defaults rather than disabling
// the breaker.
func blockerBreakerHalt(evolveDir, projectRoot string, batchStartCycle int, stderr io.Writer, signals *signalcenter.Center) (rc int, halted bool) {
	pol, _ := policy.Load(filepath.Join(evolveDir, "policy.json"))
	fp, err := pol.FailurePolicyConfig()
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: blocker-breaker: failure policy unreadable (%v) — using compiled defaults\n", err)
		fp = policy.DefaultSystemFailurePolicy()
	}
	digests := core.CollectBatchFailureDigests(evolveDir, batchStartCycle+1)
	reconcileConsumedFingerprints(evolveDir, stderr)
	reconcileConsumedBindings(projectRoot, evolveDir, stderr)
	acked, lerr := core.LoadResolvedFingerprints(evolveDir)
	if lerr != nil {
		fmt.Fprintf(stderr, "[loop] WARN: blocker-breaker: resolved-fingerprints ledger unreadable (%v) — proceeding without exclusions\n", lerr)
		acked = nil
	}
	v := core.EvaluateBlockerBreaker(digests, core.BlockerBreakerConfig{
		GuardClassCeiling:           fp.Thresholds.GuardClassHaltCeiling,
		IdenticalFingerprintCeiling: fp.Thresholds.IdenticalFingerprintHaltCeiling,
		UnexplainedCeiling:          fp.Thresholds.UnexplainedFailuresHaltCeiling,
		ConsecutiveFailuresCeiling:  fp.Thresholds.ConsecutiveFailuresHaltCeiling,
		AckedFingerprints:           acked,
	})
	if !v.Halt {
		return 0, false
	}
	latest := 0
	for _, d := range digests {
		if d.Cycle > latest {
			latest = d.Cycle
		}
	}
	workspace := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", latest))
	sf := &cyclestate.SystemFailureSignal{
		Category: "pipeline-blocker",
		Level:    "system",
		Evidence: v.Reason + " (rule=" + v.Rule + " fingerprint=" + v.Fingerprint + ")",
		Halt:     true,
	}
	// The rule's fields flow into the one loop.halt INCIDENT the shared halt
	// action emits, so nothing is signalled twice.
	// See ADR-0101.
	rule := loopHaltRule{code: CodeLoopPipelineBlockerHalt, fields: map[string]string{"rule": v.Rule, "fingerprint": v.Fingerprint}}
	return haltOnSystemFailure(evolveDir, projectRoot, latest, workspace, sf, stderr, signals, rule), true
}
