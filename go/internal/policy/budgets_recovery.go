package policy

type ReportBudgetPolicy struct {
	HandoffTokens int `json:"handoff_tokens,omitempty"`
}

// ReportBudgetConfig is the resolved report-budget configuration with defaults
// applied. HandoffTokens is the ~2K default token budget for the never-evict
// "## Handoff Summary" section (policy-sourced per phase_settings_from_config_not_code).
type ReportBudgetConfig struct {
	HandoffTokens int
}

// ReportBudgetConfig returns the report-size token budgets with built-in
// defaults resolved. An absent or empty report_budget block resolves the 2000
// default; an explicit positive value overrides it.
func (p Policy) ReportBudgetConfig() ReportBudgetConfig {
	c := ReportBudgetConfig{HandoffTokens: 2000}
	if p.ReportBudget == nil {
		return c
	}
	if p.ReportBudget.HandoffTokens > 0 {
		c.HandoffTokens = p.ReportBudget.HandoffTokens
	}
	return c
}

// RetroAutofilePolicy is the .evolve/policy.json "retro_autofile" block. It
// tunes the retro→inbox preventive-actions autofiler (internal/retrofile):
// DefaultWeight is the weight applied to a filed preventive-action item that
// carries no per-action weight_hint. Absent ⇒ the compiled-in safe default.
type RetroAutofilePolicy struct {
	DefaultWeight float64 `json:"default_weight,omitempty"`
}

// RetroAutofileDefaultWeight returns the default weight for auto-filed retro
// preventive-action inbox items. Absent/non-positive block ⇒ 0.75 (the
// compiled-in safe default); a present positive DefaultWeight overrides it.
// Sourced from policy, never a Go literal at the call site
// (feedback_phase_settings_from_config_not_code).
func (p Policy) RetroAutofileDefaultWeight() float64 {
	const safeDefault = 0.75
	if p.RetroAutofile == nil || p.RetroAutofile.DefaultWeight <= 0 {
		return safeDefault
	}
	return p.RetroAutofile.DefaultWeight
}

// GoalStallPolicy is the .evolve/policy.json "goal_stall" block. It tunes the
// goal-stall escalation (cmd/evolve/cmd_loop_goalstall.go): Threshold is the
// number of CONSECUTIVE empty/blocked cycles on one goal after which the loop
// stops blind re-dispatch and self-files an inbox todo; NonprogressThreshold is
// the same ceiling for the WIDER union class (any cycle that shipped nothing,
// FAIL included); Weight is that todo's weight. Absent ⇒ the compiled-in safe
// defaults.
type GoalStallPolicy struct {
	Threshold            int     `json:"threshold,omitempty"`
	NonprogressThreshold int     `json:"nonprogress_threshold,omitempty"`
	Weight               float64 `json:"weight,omitempty"`
}

// GoalStallThreshold returns the consecutive non-shipping-cycle count that
// triggers goal-stall escalation. Absent/non-positive block ⇒ 3 (the compiled-in
// safe default); a present positive Threshold overrides it. Sourced from policy,
// never a Go literal at the call site (feedback_phase_settings_from_config_not_code).
func (p Policy) GoalStallThreshold() int {
	const safeDefault = 3
	if p.GoalStall == nil || p.GoalStall.Threshold <= 0 {
		return safeDefault
	}
	return p.GoalStall.Threshold
}

// GoalStallNonprogressThreshold returns the consecutive NON-SHIPPING-cycle count
// (any outcome that is not PASS / SHIPPED_VIA_BUILD — FAIL, WARN, empty and
// blocked alike) that triggers the union non-progress escalation. It exists
// because the empty-only goal-stall counter and the consecutive-FAIL breaker each
// RESET on the other's outcome, so an interleaved FAIL,EMPTY,FAIL,EMPTY goal
// crossed neither ceiling and ground on forever. Absent/non-positive block ⇒ 5,
// deliberately ABOVE the empty-only default of 3: a mixed streak is noisier
// evidence (a FAIL at least produced a signal), so it gets more rope. Sourced
// from policy, never a Go literal at the call site
// (feedback_phase_settings_from_config_not_code).
func (p Policy) GoalStallNonprogressThreshold() int {
	const safeDefault = 5
	if p.GoalStall == nil || p.GoalStall.NonprogressThreshold <= 0 {
		return safeDefault
	}
	return p.GoalStall.NonprogressThreshold
}

// GoalStallWeight returns the weight applied to a self-filed goal-stall inbox
// todo. Absent/non-positive block ⇒ 0.9 (a stalled goal is a high-priority
// self-prioritization signal, never a low-weight afterthought). A present
// positive Weight raises it above 0.9; the item-build layer independently
// re-floors any value below 0.9 back UP to 0.9 (goalStallWeightFloor), so a
// config value under 0.9 is accepted here but does not take effect — the
// effective floor is always 0.9 regardless of config.
func (p Policy) GoalStallWeight() float64 {
	const safeDefault = 0.9
	if p.GoalStall == nil || p.GoalStall.Weight <= 0 {
		return safeDefault
	}
	return p.GoalStall.Weight
}

// SandboxPolicy is the .evolve/policy.json "sandbox" block. NestedFallback
// selects the verified-fallback rollout stage for nested runs where the inner
// OS sandbox can't apply: "off" (default — no canary), "shadow" (run the
// write-canary and WARN if the outer environment is unverified), or "enforce"
// (HALT the batch if unverified). Resolved to a config.Stage via parseGateStage
// at the composition root; unknown values map to off (canary disabled).
type SandboxPolicy struct {
	NestedFallback string `json:"nested_fallback,omitempty"`
}

// SandboxConfig returns sandbox configuration with built-in defaults resolved.
// Empty/absent NestedFallback ⇒ "off" (canary opt-in; a fresh policy.json never
// runs the write-canary nor halts a nested run).
func (p Policy) SandboxConfig() SandboxPolicy {
	c := SandboxPolicy{NestedFallback: "off"}
	if p.Sandbox == nil {
		return c
	}
	if p.Sandbox.NestedFallback != "" {
		c.NestedFallback = p.Sandbox.NestedFallback
	}
	return c
}

// RecoveryPolicy is the .evolve/policy.json "recovery" block.
// It surfaces the ADR-0044 Unified Phase Recovery rollout stage so operators
// can set phase_recovery = "enforce" in policy.json without an env var, and
// (R8.5, 2026-07-16) the artifact-backed spine floor's OWN dial — split from
// phase_recovery because that stage ALSO arms the bidirectional channel
// (ADR-0045 I6) and the failure-adviser promotion path (see
// config.RolloutStages.SpineFloor for the full decoupling rationale).
type RecoveryPolicy struct {
	PhaseRecovery string `json:"phase_recovery,omitempty"`
	// SpineFloor gates ONLY the clean-absence handoff-gap abort:
	// "enforce" (default) aborts the cycle; "shadow" WARN-and-proceeds (the
	// no-recompile escape hatch); anything else parses to off ≡ shadow (the
	// gate never acts below enforce).
	SpineFloor string `json:"spine_floor,omitempty"`
}

// RecoveryConfig returns recovery configuration with built-in defaults resolved.
// Empty/absent PhaseRecovery ⇒ "shadow" (behavior-neutral first-ship default);
// empty/absent SpineFloor ⇒ "enforce" (the R8.5 flip — replay-evidenced; see
// config.defaults()).
func (p Policy) RecoveryConfig() RecoveryPolicy {
	c := RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce"}
	if p.Recovery == nil {
		return c
	}
	if p.Recovery.PhaseRecovery != "" {
		c.PhaseRecovery = p.Recovery.PhaseRecovery
	}
	if p.Recovery.SpineFloor != "" {
		c.SpineFloor = p.Recovery.SpineFloor
	}
	return c
}

// DocsFloorPolicy is the .evolve/policy.json "docs_floor" block — the
// config-as-code dial (no flag) for the ADR-0077 documentation floor, shaped
// exactly like the SpineFloor dial it is modeled on.
type DocsFloorPolicy struct {
	// Stage selects the rollout stage: "off" / "shadow" / "enforce".
	// Empty/absent ⇒ "enforce". Unlike the spine floor, "enforce" here still
	// only WARNs (see internal/docsfloor): the mechanical half of the rule is
	// "is there a doc at all", and judging adequacy stays with the auditor.
	// "off" is the no-recompile escape hatch for a lane that legitimately
	// churns architecture surfaces without a doc delta.
	Stage string `json:"stage,omitempty"`
}

// DocsFloorConfig returns docs-floor configuration with the built-in default
// resolved: empty/absent Stage ⇒ "enforce".
func (p Policy) DocsFloorConfig() DocsFloorPolicy {
	c := DocsFloorPolicy{Stage: "enforce"}
	if p.DocsFloor == nil {
		return c
	}
	if p.DocsFloor.Stage != "" {
		c.Stage = p.DocsFloor.Stage
	}
	return c
}

// MergeGatePolicy is the .evolve/policy.json "merge_gate" block — the config-as-code
// surface for the merge-to-main gate (no flags). Stage drives the
// shadow→advisory→enforce rollout; the remaining fields are the cadence-scaling
// thresholds the advisor reads to decide when accumulated milestone work is
// promoted to main.
type MergeGatePolicy struct {
	// Stage selects the rollout stage: "off" / "shadow" / "advisory" / "enforce".
	// Empty/absent ⇒ "shadow" (gate runs and records its would-be verdict but
	// promotes nothing — byte-neutral first deploy over the riskiest action). The
	// composition root translates this string to a config.Stage via parseStage,
	// whose closed vocabulary maps any UNKNOWN value (e.g. a "enforced" typo) to
	// StageOff — a fail-safe that disables the gate rather than guessing, so a
	// misspelling can never silently arm auto-merge.
	Stage string `json:"stage,omitempty"`
	// BatchWaveCount is how many completed campaign waves accumulate before the
	// advisor fires the gate (cadence scaling). Zero/absent ⇒ 1 (gate per wave).
	BatchWaveCount int `json:"batch_wave_count,omitempty"`
	// BatchChurnLOC is the diff-size ceiling (changed LOC) above which the advisor
	// prefers batching over per-wave promotion. Zero/absent ⇒ 800.
	BatchChurnLOC int `json:"batch_churn_loc,omitempty"`
	// BlockSeverity is the build severity at or above which the gate hard-defers
	// promotion. Empty/absent ⇒ "HIGH".
	BlockSeverity string `json:"block_severity,omitempty"`
	// CarryoverStallCycles is the anti-starvation bound: when a feature's oldest
	// unpicked P0/P1 carryover has aged this many cycles, force a feature-complete
	// promotion attempt. Zero/absent ⇒ 8.
	CarryoverStallCycles int `json:"carryover_stall_cycles,omitempty"`
}

// MergeGateConfig is the resolved merge-gate configuration with defaults applied.
type MergeGateConfig struct {
	Stage                string
	BatchWaveCount       int
	BatchChurnLOC        int
	BlockSeverity        string
	CarryoverStallCycles int
}

// MergeGateConfig returns merge-gate configuration with built-in defaults
// resolved. The zero-value Policy{} yields the safe defaults (stage="shadow",
// so an absent block is provably behavior-neutral). Each numeric threshold
// overrides only when > 0 and each string only when non-empty, so a partial
// block can never silently produce an unsafe zero threshold. Pure.
func (p Policy) MergeGateConfig() MergeGateConfig {
	c := MergeGateConfig{
		Stage:                "shadow",
		BatchWaveCount:       1,
		BatchChurnLOC:        800,
		BlockSeverity:        "HIGH",
		CarryoverStallCycles: 8,
	}
	if p.MergeGate == nil {
		return c
	}
	if p.MergeGate.Stage != "" {
		c.Stage = p.MergeGate.Stage
	}
	if p.MergeGate.BatchWaveCount > 0 {
		c.BatchWaveCount = p.MergeGate.BatchWaveCount
	}
	if p.MergeGate.BatchChurnLOC > 0 {
		c.BatchChurnLOC = p.MergeGate.BatchChurnLOC
	}
	if p.MergeGate.BlockSeverity != "" {
		c.BlockSeverity = p.MergeGate.BlockSeverity
	}
	if p.MergeGate.CarryoverStallCycles > 0 {
		c.CarryoverStallCycles = p.MergeGate.CarryoverStallCycles
	}
	return c
}

// ParallelEvaluatePolicy is the .evolve/policy.json "parallel_evaluate" block —
// the config-as-code surface for post-build evaluate-phase parallelization (no
// flags). Stage drives the off→shadow→enforce rollout; Concurrency bounds the
// parallel runner pool. See [[phase_settings_from_config_not_code]].
