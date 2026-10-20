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

func blockerBreakerHalt(evolveDir, projectRoot string, batchStartCycle int, stderr io.Writer, signals *signalcenter.Center) (rc int, halted bool) {
	fp := breakerFailurePolicy(evolveDir, stderr)
	digests := core.CollectBatchFailureDigests(evolveDir, batchStartCycle+1)
	deferrals := core.CollectBatchLaneDeferrals(evolveDir, batchStartCycle+1)
	reconcileConsumedFingerprints(evolveDir, stderr)
	reconcileConsumedBindings(projectRoot, evolveDir, stderr)
	acked := breakerAckedFingerprints(evolveDir, stderr)
	v := core.EvaluateBlockerBreaker(digests, core.BlockerBreakerConfig{
		GuardClassCeiling:           fp.Thresholds.GuardClassHaltCeiling,
		IdenticalFingerprintCeiling: fp.Thresholds.IdenticalFingerprintHaltCeiling,
		UnexplainedCeiling:          fp.Thresholds.UnexplainedFailuresHaltCeiling,
		ConsecutiveFailuresCeiling:  fp.Thresholds.ConsecutiveFailuresHaltCeiling,
		AckedFingerprints:           acked,
		LaneDeferredCycles:          core.LaneDeferredCycleSet(deferrals),
	})
	if v.Halt {
		latest := 0
		for _, d := range digests {
			latest = max(latest, d.Cycle)
		}
		return haltPipelineBlocker(evolveDir, projectRoot, latest, "pipeline-blocker", v, stderr, signals), true
	}
	if v := core.EvaluateLaneDeferrals(deferrals, fp.Thresholds.LaneDeferralHaltCeiling); v.Halt {
		latest := 0
		for _, d := range deferrals {
			latest = max(latest, d.Cycle)
		}
		return haltPipelineBlocker(evolveDir, projectRoot, latest, "lane-provisioning", v, stderr, signals), true
	}
	return noWorkHandoffHalt(evolveDir, projectRoot, fp.Thresholds, acked, stderr, signals)
}

func breakerFailurePolicy(evolveDir string, stderr io.Writer) policy.SystemFailurePolicy {
	pol, _ := policy.Load(filepath.Join(evolveDir, "policy.json"))
	fp, err := pol.FailurePolicyConfig()
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: blocker-breaker: failure policy unreadable (%v) — using compiled defaults\n", err)
		return policy.DefaultSystemFailurePolicy()
	}
	return fp
}

func breakerAckedFingerprints(evolveDir string, stderr io.Writer) map[string]bool {
	acked, err := core.LoadResolvedFingerprints(evolveDir)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: blocker-breaker: resolved-fingerprints ledger unreadable (%v) — proceeding without exclusions\n", err)
		return nil
	}
	return acked
}

func noWorkHandoffHalt(evolveDir, projectRoot string, th policy.FailureThresholds, acked map[string]bool, stderr io.Writer, signals *signalcenter.Center) (rc int, halted bool) {
	through := core.NoWorkHandoffsResolvedThrough(acked)
	if through > 0 {
		fmt.Fprintf(stderr, "[loop] no-work-handoffs: hand-offs through cycle %d are acknowledged and not counted\n", through)
	}
	handoffs := core.CollectNoWorkHandoffs(evolveDir, through, th.NoWorkHandoffWindow)
	v := core.EvaluateNoWorkHandoffs(handoffs, th.NoWorkHandoffHaltCeiling)
	if !v.Halt {
		return 0, false
	}
	return haltPipelineBlocker(evolveDir, projectRoot, handoffs[len(handoffs)-1], "no-work-handoff", v, stderr, signals), true
}

func noWorkHandoffHaltAfterWave(evolveDir, projectRoot string, stderr io.Writer, signals *signalcenter.Center) (rc int, halted bool) {
	reconcileConsumedFingerprints(evolveDir, stderr)
	return noWorkHandoffHalt(evolveDir, projectRoot, breakerFailurePolicy(evolveDir, stderr).Thresholds, breakerAckedFingerprints(evolveDir, stderr), stderr, signals)
}

func haltPipelineBlocker(evolveDir, projectRoot string, cycle int, category string, v core.BlockerVerdict, stderr io.Writer, signals *signalcenter.Center) int {
	workspace := filepath.Join(evolveDir, "runs", fmt.Sprintf("cycle-%d", cycle))
	sf := &cyclestate.SystemFailureSignal{
		Category: category,
		Level:    "system",
		Evidence: v.Reason + " (rule=" + v.Rule + " fingerprint=" + v.Fingerprint + ")",
		Halt:     true,
	}
	rule := loopHaltRule{code: CodeLoopPipelineBlockerHalt, fields: map[string]string{"rule": v.Rule, "fingerprint": v.Fingerprint}}
	return haltOnSystemFailure(evolveDir, projectRoot, cycle, workspace, sf, stderr, signals, rule)
}
