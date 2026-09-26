package policy

import "github.com/mickeyyaya/evolve-loop/go/internal/config"

// ReportBudgetPolicy is the "report_budget" block, dialed separately from the report-size gate's stage.
type ReportBudgetPolicy struct {
	HandoffTokens int `json:"handoff_tokens,omitempty"`
}

// ReportBudgetConfig holds the resolved token budget for the never-evicted "## Handoff Summary" section.
type ReportBudgetConfig struct {
	HandoffTokens int
}

// ReportBudgetConfig returns the report budgets, defaulting HandoffTokens to 2000.
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

// RetroAutofilePolicy is the "retro_autofile" block; DefaultWeight applies to preventive actions without a weight_hint.
type RetroAutofilePolicy struct {
	DefaultWeight float64 `json:"default_weight,omitempty"`
}

// RetroAutofileDefaultWeight returns the weight for auto-filed retro preventive actions, defaulting to 0.75.
func (p Policy) RetroAutofileDefaultWeight() float64 {
	const safeDefault = 0.75
	if p.RetroAutofile == nil || p.RetroAutofile.DefaultWeight <= 0 {
		return safeDefault
	}
	return p.RetroAutofile.DefaultWeight
}

// GoalStallPolicy is the "goal_stall" block for the goal-stall escalation.
type GoalStallPolicy struct {
	Threshold            int     `json:"threshold,omitempty"`
	NonprogressThreshold int     `json:"nonprogress_threshold,omitempty"`
	Weight               float64 `json:"weight,omitempty"`
}

// GoalStallThreshold returns how many consecutive empty or blocked cycles on one goal trigger escalation, defaulting to 3.
func (p Policy) GoalStallThreshold() int {
	const safeDefault = 3
	if p.GoalStall == nil || p.GoalStall.Threshold <= 0 {
		return safeDefault
	}
	return p.GoalStall.Threshold
}

// GoalStallNonprogressThreshold returns how many consecutive non-shipping cycles of any outcome trigger escalation.
func (p Policy) GoalStallNonprogressThreshold() int {
	// Above the empty-only default of 3: a mixed FAIL/EMPTY streak is noisier evidence.
	const safeDefault = 5
	if p.GoalStall == nil || p.GoalStall.NonprogressThreshold <= 0 {
		return safeDefault
	}
	return p.GoalStall.NonprogressThreshold
}

// GoalStallWeight returns the goal-stall todo's weight, defaulting to 0.9; the item builder re-floors lower values to 0.9.
func (p Policy) GoalStallWeight() float64 {
	const safeDefault = 0.9
	if p.GoalStall == nil || p.GoalStall.Weight <= 0 {
		return safeDefault
	}
	return p.GoalStall.Weight
}

// SandboxPolicy is the "sandbox" block.
type SandboxPolicy struct {
	// NestedFallback stages the nested-run write-canary: "off" (default), "shadow" (WARN) or "enforce" (HALT).
	NestedFallback string `json:"nested_fallback,omitempty"`
}

// SandboxConfig returns the sandbox block, defaulting NestedFallback to "off".
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

// RecoveryPolicy is the "recovery" block of phase-recovery rollout stages.
// See ADR-0044.
type RecoveryPolicy struct {
	// PhaseRecovery also arms the bidirectional channel and adviser promotion, so the two floors below have their own dials.
	PhaseRecovery string `json:"phase_recovery,omitempty"`
	// SpineFloor gates only the clean-absence handoff-gap abort; below "enforce" it WARNs and proceeds.
	SpineFloor string `json:"spine_floor,omitempty"`
	// FatalPane gates only the stop-review fatal-pane fast-fail: "enforce" ends the wait,
	// "shadow" records would_fast_fail, "off" skips the detector.
	FatalPane string `json:"fatal_pane,omitempty"`
}

// RecoveryConfig returns the recovery block, defaulting to shadow phase recovery and enforced SpineFloor and FatalPane.
func (p Policy) RecoveryConfig() RecoveryPolicy {
	c := RecoveryPolicy{PhaseRecovery: "shadow", SpineFloor: "enforce", FatalPane: "enforce"}
	if p.Recovery == nil {
		return c
	}
	if p.Recovery.PhaseRecovery != "" {
		c.PhaseRecovery = p.Recovery.PhaseRecovery
	}
	if p.Recovery.SpineFloor != "" {
		c.SpineFloor = p.Recovery.SpineFloor
	}
	if p.Recovery.FatalPane != "" {
		c.FatalPane = p.Recovery.FatalPane
	}
	return c
}

// BridgeRecoveryStages returns the phase-recovery and fatal-pane stage words for bridge roots that bypass the Loader.
func (p Policy) BridgeRecoveryStages() (recovery, fatalPane string) {
	rc := p.RecoveryConfig()
	// The Loader's own parser, so every root resolves a word identically and an unknown word is off.
	r, _ := config.GateStage(rc.PhaseRecovery)
	f, _ := config.GateStage(rc.FatalPane)
	return r.String(), f.String()
}

// DocsFloorPolicy is the "docs_floor" block for the documentation floor.
// See ADR-0077.
type DocsFloorPolicy struct {
	// Stage is "off", "shadow" or "enforce" (default); even "enforce" only WARNs.
	Stage string `json:"stage,omitempty"`
}

// DocsFloorConfig returns the docs_floor block, defaulting Stage to "enforce".
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

// MergeGatePolicy is the "merge_gate" block: the merge-to-main gate's stage and cadence thresholds.
// See ADR-0057.
type MergeGatePolicy struct {
	// Stage is "off", "shadow" (default), "advisory" or "enforce"; the composition
	// root maps an unknown word to off, so a typo can never arm auto-merge.
	Stage string `json:"stage,omitempty"`
	// BatchWaveCount is how many completed waves accumulate before the gate fires; non-positive means 1.
	BatchWaveCount int `json:"batch_wave_count,omitempty"`
	// BatchChurnLOC is the changed-LOC ceiling above which batching is preferred; non-positive means 800.
	BatchChurnLOC int `json:"batch_churn_loc,omitempty"`
	// BlockSeverity is the build severity at or above which promotion is deferred; empty means "HIGH".
	BlockSeverity string `json:"block_severity,omitempty"`
	// CarryoverStallCycles forces a promotion attempt once the oldest P0/P1 carryover is this old; non-positive means 8.
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

// MergeGateConfig returns merge-gate configuration; only positive numbers and non-empty strings override.
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
