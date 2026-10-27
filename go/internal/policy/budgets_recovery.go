package policy

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

type ReportBudgetPolicy struct {
	HandoffTokens int `json:"handoff_tokens,omitempty"`
}

type ReportBudgetConfig struct {
	HandoffTokens int
}

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

type RetroAutofilePolicy struct {
	DefaultWeight float64 `json:"default_weight,omitempty"`
}

func (p Policy) RetroAutofileDefaultWeight() float64 {
	const safeDefault = 0.75
	if p.RetroAutofile == nil || p.RetroAutofile.DefaultWeight <= 0 {
		return safeDefault
	}
	return p.RetroAutofile.DefaultWeight
}

type GoalStallPolicy struct {
	Threshold            int     `json:"threshold,omitempty"`
	NonprogressThreshold int     `json:"nonprogress_threshold,omitempty"`
	Weight               float64 `json:"weight,omitempty"`
}

func (p Policy) GoalStallThreshold() int {
	const safeDefault = 3
	if p.GoalStall == nil || p.GoalStall.Threshold <= 0 {
		return safeDefault
	}
	return p.GoalStall.Threshold
}

func (p Policy) GoalStallNonprogressThreshold() int {
	const safeDefault = 5
	if p.GoalStall == nil || p.GoalStall.NonprogressThreshold <= 0 {
		return safeDefault
	}
	return p.GoalStall.NonprogressThreshold
}

func (p Policy) GoalStallWeight() float64 {
	const safeDefault = 0.9
	if p.GoalStall == nil || p.GoalStall.Weight <= 0 {
		return safeDefault
	}
	return p.GoalStall.Weight
}

type SandboxPolicy struct {
	NestedFallback string `json:"nested_fallback,omitempty"`
	Mode           string `json:"mode,omitempty"`
}

func (s *SandboxPolicy) UnmarshalJSON(raw []byte) error {
	type block SandboxPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("sandbox: %w", err)
	}
	if decoded.Mode != "" && decoded.Mode != config.SandboxModeAuto && decoded.Mode != config.SandboxModeOn {
		return fmt.Errorf("sandbox: mode %q is not auto or on; the sandbox turns off only through the audited --sandbox-off=<reason> parameter", decoded.Mode)
	}
	*s = SandboxPolicy(decoded)
	return nil
}

func (s SandboxPolicy) SandboxMode() string {
	if s.Mode == config.SandboxModeOn {
		return config.SandboxModeOn
	}
	return config.SandboxModeAuto
}

func (p Policy) SandboxConfig() SandboxPolicy {
	c := SandboxPolicy{NestedFallback: "off"}
	if p.Sandbox == nil {
		return c
	}
	if p.Sandbox.NestedFallback != "" {
		c.NestedFallback = p.Sandbox.NestedFallback
	}
	c.Mode = p.Sandbox.Mode
	return c
}

type RecoveryPolicy struct {
	PhaseRecovery string `json:"phase_recovery,omitempty"`
	SpineFloor    string `json:"spine_floor,omitempty"`
	FatalPane     string `json:"fatal_pane,omitempty"`
}

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

func (p Policy) BridgeRecoveryStages() (recovery, fatalPane string) {
	rc := p.RecoveryConfig()
	r, _ := config.GateStage(rc.PhaseRecovery)
	f, _ := config.GateStage(rc.FatalPane)
	return r.String(), f.String()
}

type DocsFloorPolicy struct {
	Stage    string `json:"stage,omitempty"`
	SteStage string `json:"ste_stage,omitempty"`
}

func (d *DocsFloorPolicy) UnmarshalJSON(raw []byte) error {
	type block DocsFloorPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("docs_floor: %w", err)
	}
	for _, dial := range [][2]string{{"stage", decoded.Stage}, {"ste_stage", decoded.SteStage}} {
		if _, ok := config.GateStage(dial[1]); dial[1] != "" && !ok {
			return fmt.Errorf("docs_floor: %s %q is not off, shadow or enforce", dial[0], dial[1])
		}
	}
	*d = DocsFloorPolicy(decoded)
	return nil
}

func (p Policy) DocsFloorConfig() DocsFloorPolicy {
	c := DocsFloorPolicy{Stage: "enforce", SteStage: "shadow"}
	if p.DocsFloor == nil {
		return c
	}
	if p.DocsFloor.Stage != "" {
		c.Stage = p.DocsFloor.Stage
	}
	if p.DocsFloor.SteStage != "" {
		c.SteStage = p.DocsFloor.SteStage
	}
	return c
}

type CommentFloorPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) CommentFloorConfig() CommentFloorPolicy {
	c := CommentFloorPolicy{Stage: "shadow"}
	if p.CommentFloor != nil && p.CommentFloor.Stage != "" {
		c.Stage = p.CommentFloor.Stage
	}
	return c
}

type MergeGatePolicy struct {
	Stage                string `json:"stage,omitempty"`
	BatchWaveCount       int    `json:"batch_wave_count,omitempty"`
	BatchChurnLOC        int    `json:"batch_churn_loc,omitempty"`
	BlockSeverity        string `json:"block_severity,omitempty"`
	CarryoverStallCycles int    `json:"carryover_stall_cycles,omitempty"`
}

type MergeGateConfig struct {
	Stage                string
	BatchWaveCount       int
	BatchChurnLOC        int
	BlockSeverity        string
	CarryoverStallCycles int
}

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

type CoverChangedPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) CoverChangedConfig() CoverChangedPolicy {
	c := CoverChangedPolicy{Stage: "enforce"}
	if p.CoverChanged != nil && p.CoverChanged.Stage != "" {
		c.Stage = p.CoverChanged.Stage
	}
	return c
}

type RetrospectivesPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) RetrospectivesConfig() RetrospectivesPolicy {
	c := RetrospectivesPolicy{Stage: "shadow"}
	if p.Retrospectives != nil && p.Retrospectives.Stage != "" {
		c.Stage = p.Retrospectives.Stage
	}
	return c
}
