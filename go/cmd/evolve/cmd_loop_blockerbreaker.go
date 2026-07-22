package main

// cmd_loop_blockerbreaker.go — loop wiring of the mid-batch pipeline-blocker
// breaker (core.EvaluateBlockerBreaker; ADR-0072 extension, operator directive
// 2026-07-22). Checked at the TOP of every batch iteration, before another
// cycle is dispatched: a pipeline blocker must be fixed directly, never passed
// to the following cycles. On a trip it reuses the ADR-0072 halt machinery
// verbatim — escalation dossier + P0 pipeline-repair inbox item + the
// system-failure exit code — so operators have ONE halt vocabulary.

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// blockerBreakerHalt evaluates the batch's failure digests (cycles strictly
// after batchStartCycle) against the policy ceilings. halted=false means
// continue dispatching. On halt it writes the escalation + P0 item and returns
// the loop exit code. Policy is loaded fresh per check (the wave-boundary
// hot-reload idiom); a missing/malformed policy falls back to compiled
// defaults via FailurePolicyConfig's own resolution — the breaker never
// fail-opens to disabled silently.
func blockerBreakerHalt(evolveDir, projectRoot string, batchStartCycle int, stderr io.Writer) (rc int, halted bool) {
	pol, _ := policy.Load(filepath.Join(evolveDir, "policy.json"))
	fp, err := pol.FailurePolicyConfig()
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: blocker-breaker: failure policy unreadable (%v) — using compiled defaults\n", err)
		fp = policy.DefaultSystemFailurePolicy()
	}
	digests := core.CollectBatchFailureDigests(evolveDir, batchStartCycle+1)
	v := core.EvaluateBlockerBreaker(digests, core.BlockerBreakerConfig{
		GuardClassCeiling:           fp.Thresholds.GuardClassHaltCeiling,
		IdenticalFingerprintCeiling: fp.Thresholds.IdenticalFingerprintHaltCeiling,
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
	fmt.Fprintf(stderr, "[loop] PIPELINE-BLOCKER HALT: %s — stopping the batch instead of passing the failure to the next cycle; fix the pipeline directly, then resume with evolve loop --resume\n", sf.Evidence)
	return haltOnSystemFailure(evolveDir, projectRoot, latest, workspace, sf, stderr), true
}
