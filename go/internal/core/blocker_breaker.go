package core

// blocker_breaker.go — mid-batch pipeline-blocker breaker (ADR-0072 extension,
// operator directive 2026-07-22: a pipeline blocker must be fixed directly,
// never passed to following cycles). The ADR-0072 floor halts on FORGED
// verdicts; this breaker halts on the other blocker signature — the same
// failure identity recurring across a batch's cycles, which honest-looking
// per-cycle FAILs never surface on their own (batch-5 burned six cycles on one
// class; the 862–899 storm burned 37 on byte-identical defect strings).
//
// Two deterministic rules over the S1 failure digests (failure_digest.go),
// evaluated batch-scoped by the loop after every iteration:
//
//	Rule A "guard-class"           — guard-abort digests ≥ GuardClassCeiling.
//	                                 A guard abort is pipeline machinery
//	                                 failing by construction, never task-legit.
//	Rule B "identical-fingerprint" — one exact fingerprint ≥
//	                                 IdenticalFingerprintCeiling. Identical
//	                                 failure identities cannot be distinct
//	                                 honest defects.
//
// Same-task repeats are S5 quarantine's job (task_retry_ceiling) — the breaker
// is task-agnostic so a healthy batch of many DIFFERENT honest rejections
// (batch-2's shape) never trips it. A zero ceiling disables its rule (the
// policy escape hatch, mirroring the positive-overrides-win threshold merge).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BlockerBreakerConfig carries the policy ceilings (policy.FailureThresholds
// GuardClassHaltCeiling / IdenticalFingerprintHaltCeiling at the composition
// root). Zero disables the rule.
type BlockerBreakerConfig struct {
	GuardClassCeiling           int
	IdenticalFingerprintCeiling int
	// UnexplainedCeiling halts when this many digests carry NO machine-
	// readable failure reason (the degenerate empty-evidence bucket) — a
	// diagnosability breakdown, deliberately named apart from the identical-
	// fingerprint rule (batch-6: three DIFFERENT failures shared one empty
	// fingerprint).
	UnexplainedCeiling int
}

// BlockerVerdict is the breaker's decision. Halt=true means the batch must
// stop and escalate (P0 pipeline-repair) instead of dispatching another cycle
// into the same wall.
type BlockerVerdict struct {
	Halt        bool
	Rule        string // "guard-class" | "identical-fingerprint"
	Fingerprint string // Rule B: the recurring identity; Rule A: representative
	Count       int
	Reason      string
}

// guardAbortClass is the failure_digest pre-class bucket that is never
// task-legit (statemap severing, tree-guard aborts).
const guardAbortClass = "guard-abort"

// isUnexplainedDigest reports the degenerate empty-evidence digest: no reason
// artifact existed, so phase and pre-class degraded to their unknown defaults.
// These MUST NOT count as "identical" defects — distinct failures collapse
// into this bucket by construction.
func isUnexplainedDigest(d FailureDigest) bool {
	return d.PreClass == "unknown" && strings.HasPrefix(d.Fingerprint, "|unknown|")
}

// EvaluateBlockerBreaker applies the two rules over a batch's failure digests.
// Pure and deterministic: same digests + config in, same verdict out.
func EvaluateBlockerBreaker(digests []FailureDigest, cfg BlockerBreakerConfig) BlockerVerdict {
	if cfg.GuardClassCeiling > 0 {
		var guard []FailureDigest
		for _, d := range digests {
			if d.PreClass == guardAbortClass {
				guard = append(guard, d)
			}
		}
		if len(guard) >= cfg.GuardClassCeiling {
			return BlockerVerdict{
				Halt: true, Rule: "guard-class", Fingerprint: guard[0].Fingerprint, Count: len(guard),
				Reason: fmt.Sprintf("%d %s-class failures in one batch (ceiling %d) — guard aborts are pipeline machinery failing, never task defects", len(guard), guardAbortClass, cfg.GuardClassCeiling),
			}
		}
	}
	if cfg.UnexplainedCeiling > 0 {
		var unexplained int
		for _, d := range digests {
			if isUnexplainedDigest(d) {
				unexplained++
			}
		}
		if unexplained >= cfg.UnexplainedCeiling {
			return BlockerVerdict{
				Halt: true, Rule: "unexplained-failures", Count: unexplained,
				Reason: fmt.Sprintf("%d failures in one batch produced no machine-readable failure reason (ceiling %d) — a diagnosability breakdown: fix the missing reason-writers, then diagnose the underlying failures individually", unexplained, cfg.UnexplainedCeiling),
			}
		}
	}
	if cfg.IdenticalFingerprintCeiling > 0 {
		counts := map[string]int{}
		for _, d := range digests {
			if d.Fingerprint == "" || isUnexplainedDigest(d) {
				continue
			}
			counts[d.Fingerprint]++
			if counts[d.Fingerprint] >= cfg.IdenticalFingerprintCeiling {
				return BlockerVerdict{
					Halt: true, Rule: "identical-fingerprint", Fingerprint: d.Fingerprint, Count: counts[d.Fingerprint],
					Reason: fmt.Sprintf("failure fingerprint %q recurred %d× in one batch (ceiling %d) — identical failure identities cannot be distinct honest defects", d.Fingerprint, counts[d.Fingerprint], cfg.IdenticalFingerprintCeiling),
				}
			}
		}
	}
	return BlockerVerdict{}
}

// CollectBatchFailureDigests reads every <evolveDir>/runs/cycle-N/
// failure-digest.json with N >= fromCycle. Missing or malformed digests are
// skipped silently — a PASS cycle writes none, and one corrupt artifact must
// not disable the breaker for the rest of the batch.
func CollectBatchFailureDigests(evolveDir string, fromCycle int) []FailureDigest {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var out []FailureDigest
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !strings.HasPrefix(name, "cycle-") {
			continue
		}
		n, cerr := strconv.Atoi(strings.TrimPrefix(name, "cycle-"))
		if cerr != nil || n < fromCycle {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(evolveDir, "runs", name, "failure-digest.json"))
		if rerr != nil {
			continue
		}
		var d FailureDigest
		if json.Unmarshal(raw, &d) != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
