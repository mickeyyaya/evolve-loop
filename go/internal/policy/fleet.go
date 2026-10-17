package policy

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

type RetryPolicy struct {
	PhaseMaxAttempts          int `json:"phase_max_attempts,omitempty"`
	RetryBackoffBaseS         int `json:"retry_backoff_base_s,omitempty"`
	PhaseLatencyCeilingS      int `json:"phase_latency_ceiling_s,omitempty"`
	ContractCorrectionRetries int `json:"contract_correction_retries,omitempty"`

	retryBackoffBaseSSet         bool
	contractCorrectionRetriesSet bool
}

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

type SwarmPolicy struct {
	Stage    string `json:"stage,omitempty"`
	PortBase int    `json:"port_base,omitempty"`
}

type SwarmConfig struct {
	Stage    string
	PortBase int
}

type BootPolicy struct {
	BinaryRefresh string `json:"binary_refresh,omitempty"`
	DetachWaitS   int    `json:"detach_wait_s,omitempty"`
}

const DefaultBootDetachWait = 10 * time.Minute

func (p Policy) BootDetachWait() time.Duration {
	if p.Boot == nil || p.Boot.DetachWaitS <= 0 {
		return DefaultBootDetachWait
	}
	return time.Duration(p.Boot.DetachWaitS) * time.Second
}

func (p Policy) BootBinaryRefresh() string {
	if p.Boot != nil && p.Boot.BinaryRefresh == "off" {
		return "off"
	}
	return "auto"
}

type WorktreePolicy struct {
	Base string `json:"base,omitempty"`
}

func (p Policy) WorktreeBase() string {
	if p.Worktree == nil {
		return ""
	}
	return p.Worktree.Base
}

func WorktreeBaseFor(projectRoot string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return ""
	}
	return pol.WorktreeBase()
}

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

type FleetPolicy struct {
	Count            int                      `json:"count,omitempty"`
	Concurrency      int                      `json:"concurrency,omitempty"`
	MinLanes         int                      `json:"min_lanes,omitempty"`
	PlanSource       string                   `json:"plan_source,omitempty"`
	Scheduling       string                   `json:"scheduling,omitempty"`
	Landing          string                   `json:"landing,omitempty"`
	Budget           *FleetBudgetPolicy       `json:"budget,omitempty"`
	StarvationK      int                      `json:"starvation_k,omitempty"`
	StarvationWeight float64                  `json:"starvation_weight,omitempty"`
	Orchestrator     *FleetOrchestratorPolicy `json:"orchestrator,omitempty"`
}

type FleetBudgetPolicy struct {
	Stage          string  `json:"stage,omitempty"`
	CapacityCycles float64 `json:"capacity_cycles,omitempty"`
	SafetyFraction float64 `json:"safety_fraction,omitempty"`
	HistoryWindow  int     `json:"history_window,omitempty"`
}

const defaultFleetBudgetHistoryWindow = 10

type FleetBudgetConfig struct {
	Stage          string
	CapacityCycles float64
	Safety         float64
	HistoryWindow  int
}

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
	Orchestrator     OrchestratorConfig
	Warnings         []string
}

const (
	defaultStarvationK      = 3
	defaultStarvationWeight = 0.9
)

func (p Policy) FleetConfig() FleetConfig {
	c := FleetConfig{Count: 1, Concurrency: 1, MinLanes: 1, PlanSource: "triage",
		Scheduling: "wave", Landing: "per-lane", StarvationK: defaultStarvationK, StarvationWeight: defaultStarvationWeight, Orchestrator: orchestratorConfig(nil)}
	if p.Fleet == nil {
		return c
	}
	c.Orchestrator = orchestratorConfig(p.Fleet.Orchestrator)
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
