package policy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// RetryPolicy is the .evolve/policy.json "retry" block.
type RetryPolicy struct {
	PhaseMaxAttempts          int `json:"phase_max_attempts,omitempty"`
	RetryBackoffBaseS         int `json:"retry_backoff_base_s,omitempty"`
	PhaseLatencyCeilingS      int `json:"phase_latency_ceiling_s,omitempty"`
	ContractCorrectionRetries int `json:"contract_correction_retries,omitempty"`

	retryBackoffBaseSSet         bool
	contractCorrectionRetriesSet bool
}

// UnmarshalJSON records which of the two zero-disables settings were explicitly present.
func (r *RetryPolicy) UnmarshalJSON(data []byte) error {
	type retryPolicy RetryPolicy
	var decoded retryPolicy
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*r = RetryPolicy(decoded)
	_, r.retryBackoffBaseSSet = fields["retry_backoff_base_s"]
	_, r.contractCorrectionRetriesSet = fields["contract_correction_retries"]
	return nil
}

// RetryConfig is the resolved retry configuration with defaults applied.
type RetryConfig struct {
	PhaseMaxAttempts          int
	RetryBackoffBaseS         int
	PhaseLatencyCeilingS      int
	ContractCorrectionRetries int
}

const (
	defaultPhaseMaxAttempts          = 2
	maxPhaseMaxAttempts              = 5
	defaultRetryBackoffBaseS         = 5
	defaultPhaseLatencyCeilingS      = 900
	defaultContractCorrectionRetries = 2
	maxContractCorrectionRetries     = 5
)

// SwarmPolicy is the .evolve/policy.json "swarm" block.
type SwarmPolicy struct {
	// Stage is "off", "shadow" (default, delegates to the inner runner), "advisory" or "enforce".
	Stage string `json:"stage,omitempty"`
	// PortBase overrides the writer dev-server port base; 0 means swarm.DefaultPortBase.
	PortBase int `json:"port_base,omitempty"`
}

// SwarmConfig is the resolved swarm configuration with defaults applied.
type SwarmConfig struct {
	Stage    string
	PortBase int
}

// BootPolicy is the .evolve/policy.json "boot" block.
type BootPolicy struct {
	// BinaryRefresh is "auto" (default: rebuild and re-exec a stale binary) or "off".
	BinaryRefresh string `json:"binary_refresh,omitempty"`
}

// BootBinaryRefresh returns "off" only for an exact "off", otherwise "auto".
func (p Policy) BootBinaryRefresh() string {
	// The self-heal is integrity posture, so a typo must not disable it.
	if p.Boot != nil && p.Boot.BinaryRefresh == "off" {
		return "off"
	}
	return "auto"
}

// WorktreePolicy is the .evolve/policy.json "worktree" block.
type WorktreePolicy struct {
	// Base overrides the per-cycle worktree base directory; empty means the caller's default.
	Base string `json:"base,omitempty"`
}

// WorktreeBase returns the worktree.base override, or "" when absent (the readers own the default).
func (p Policy) WorktreeBase() string {
	if p.Worktree == nil {
		return ""
	}
	return p.Worktree.Base
}

// WorktreeBaseFor loads projectRoot's policy and returns worktree.base, or "" if the file is missing or malformed.
func WorktreeBaseFor(projectRoot string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return ""
	}
	return pol.WorktreeBase()
}

// SwarmConfig returns swarm configuration, defaulting Stage to "shadow" and PortBase to 0.
func (p Policy) SwarmConfig() SwarmConfig {
	c := SwarmConfig{Stage: "shadow"}
	if p.Swarm == nil {
		return c
	}
	if p.Swarm.Stage != "" {
		c.Stage = p.Swarm.Stage
	}
	c.PortBase = p.Swarm.PortBase
	return c
}

// FleetPolicy is the .evolve/policy.json "fleet" block.
type FleetPolicy struct {
	// Count is the number of lanes; non-positive means 1.
	Count int `json:"count,omitempty"`
	// Concurrency is the number of lanes run in parallel; non-positive follows the resolved Count.
	Concurrency int `json:"concurrency,omitempty"`
	// MinLanes is the floor the quota-aware wave shrink never goes below; non-positive means 1, clamped to Count.
	MinLanes int `json:"min_lanes,omitempty"`
	// PlanSource is "triage" (default) or "manual"; an unknown value fails safe to "manual".
	PlanSource string `json:"plan_source,omitempty"`
	// Scheduling is "wave" (default) or "pool"; an unknown value fails safe to "wave".
	Scheduling string `json:"scheduling,omitempty"`
	// Landing is "per-lane" (default) or "prefix-queue"; an unknown value fails safe to "per-lane".
	Landing string `json:"landing,omitempty"`
	// Budget opts in to quota-driven lane sizing; absent means the wave never probes quota.
	Budget *FleetBudgetPolicy `json:"budget,omitempty"`
	// StarvationK is how many consecutive work-starved waves trigger a self-filed todo; non-positive means 3.
	StarvationK int `json:"starvation_k,omitempty"`
	// StarvationWeight is that todo's weight; non-positive means 0.9, and lower values clamp up to 0.9.
	StarvationWeight float64 `json:"starvation_weight,omitempty"`
}

// FleetBudgetPolicy is the "fleet.budget" block for quota-driven lane sizing in native quota units, never dollars.
type FleetBudgetPolicy struct {
	// Stage is "shadow" (default, compute and log only) or "enforce" (resize); unknown fails safe to "shadow".
	Stage string `json:"stage,omitempty"`
	// CapacityCycles is how many cycles a full quota window affords; sizing runs only when it and SafetyFraction are set.
	CapacityCycles float64 `json:"capacity_cycles,omitempty"`
	// SafetyFraction is the reserved headroom in (0,1]; fleetbudget owns its validation.
	SafetyFraction float64 `json:"safety_fraction,omitempty"`
	// HistoryWindow is how many recent cycles feed the pace estimate; non-positive means the default.
	HistoryWindow int `json:"history_window,omitempty"`
}

const defaultFleetBudgetHistoryWindow = 10

// FleetBudgetConfig is the resolved fleet.budget block; a nil FleetConfig.Budget means budgeting is off.
type FleetBudgetConfig struct {
	Stage          string
	CapacityCycles float64
	Safety         float64
	HistoryWindow  int
}

// FleetConfig is the resolved fleet configuration; the caller reports Warnings, since the getter does no I/O.
type FleetConfig struct {
	Count            int
	Concurrency      int
	MinLanes         int
	PlanSource       string
	Scheduling       string
	Landing          string
	Budget           *FleetBudgetConfig
	StarvationK      int
	StarvationWeight float64
	Warnings         []string
}

const (
	defaultStarvationK      = 3
	defaultStarvationWeight = 0.9
)

// FleetConfig resolves the fleet block; an unknown word falls back to the safe choice and adds a warning.
func (p Policy) FleetConfig() FleetConfig {
	c := FleetConfig{Count: 1, Concurrency: 1, MinLanes: 1, PlanSource: "triage",
		Scheduling: "wave", Landing: "per-lane", StarvationK: defaultStarvationK, StarvationWeight: defaultStarvationWeight}
	if p.Fleet == nil {
		return c
	}
	if p.Fleet.Count > 0 {
		c.Count = p.Fleet.Count
	}
	c.Concurrency = c.Count
	if p.Fleet.Concurrency > 0 {
		c.Concurrency = p.Fleet.Concurrency
	}
	if p.Fleet.MinLanes > 1 {
		c.MinLanes = p.Fleet.MinLanes
		if c.MinLanes > c.Count {
			c.MinLanes = c.Count
		}
	}
	switch p.Fleet.PlanSource {
	case "", "triage":
		c.PlanSource = "triage"
	case "manual":
		c.PlanSource = "manual"
	default:
		c.PlanSource = "manual"
		c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.plan_source: unknown value %q, falling back to \"manual\"", p.Fleet.PlanSource))
	}
	switch p.Fleet.Scheduling {
	case "", "wave":
		c.Scheduling = "wave"
	case "pool":
		c.Scheduling = "pool"
	default:
		c.Scheduling = "wave"
		c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.scheduling: unknown value %q, falling back to \"wave\"", p.Fleet.Scheduling))
	}
	switch p.Fleet.Landing {
	case "", "per-lane":
		c.Landing = "per-lane"
	case "prefix-queue":
		c.Landing = "prefix-queue"
	default:
		c.Landing = "per-lane"
		c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.landing: unknown value %q, falling back to \"per-lane\"", p.Fleet.Landing))
	}
	if p.Fleet.Budget != nil {
		b := &FleetBudgetConfig{
			Stage:          "shadow",
			CapacityCycles: p.Fleet.Budget.CapacityCycles,
			Safety:         p.Fleet.Budget.SafetyFraction,
			HistoryWindow:  defaultFleetBudgetHistoryWindow,
		}
		switch p.Fleet.Budget.Stage {
		case "enforce":
			b.Stage = "enforce"
		case "", "shadow":
			// Already "shadow"; listed so it raises no warning.
		default:
			c.Warnings = append(c.Warnings, fmt.Sprintf("fleet.budget.stage: unknown value %q, falling back to \"shadow\"", p.Fleet.Budget.Stage))
		}
		if p.Fleet.Budget.HistoryWindow > 0 {
			b.HistoryWindow = p.Fleet.Budget.HistoryWindow
		}
		c.Budget = b
	}
	if p.Fleet.StarvationK > 0 {
		c.StarvationK = p.Fleet.StarvationK
	}
	if p.Fleet.StarvationWeight > 0 {
		c.StarvationWeight = p.Fleet.StarvationWeight
		if c.StarvationWeight < defaultStarvationWeight {
			c.StarvationWeight = defaultStarvationWeight
		}
	}
	return c
}
