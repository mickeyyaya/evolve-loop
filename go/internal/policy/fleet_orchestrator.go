package policy

import (
	"errors"
	"fmt"
)

type FleetOrchestratorPolicy struct {
	Stage                string `json:"stage,omitempty"`
	ScanK                int    `json:"scan_k,omitempty"`
	HubMinImporters      int    `json:"hub_min_importers,omitempty"`
	DeadlineFloorMinutes int    `json:"deadline_floor_minutes,omitempty"`
	StallMinutes         int    `json:"stall_minutes,omitempty"`
	RunMaxLanes          int    `json:"run_max_lanes,omitempty"`
}

type OrchestratorConfig struct {
	Stage                string
	ScanK                int
	HubMinImporters      int
	DeadlineFloorMinutes int
	StallMinutes         int
	RunMaxLanes          int
}

const (
	defaultOrchestratorScanK           = 32
	defaultOrchestratorHubMinImporters = 120
	defaultOrchestratorDeadlineFloor   = 60
	defaultOrchestratorStallMinutes    = 30
	orchestratorRunLanesPerSlot        = 4
)

func (o *FleetOrchestratorPolicy) UnmarshalJSON(raw []byte) error {
	type block FleetOrchestratorPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("fleet.orchestrator: %w", err)
	}
	parsed := FleetOrchestratorPolicy(decoded)
	if err := parsed.validate(); err != nil {
		return fmt.Errorf("fleet.orchestrator: %w", err)
	}
	*o = parsed
	return nil
}

func (o FleetOrchestratorPolicy) validate() error {
	switch {
	case o.Stage != "" && o.Stage != "shadow" && o.Stage != "enforce":
		return fmt.Errorf("stage %q is not \"shadow\" or \"enforce\"", o.Stage)
	case o.ScanK < 0:
		return errors.New("scan_k must not be negative")
	case o.HubMinImporters < 0:
		return errors.New("hub_min_importers must not be negative")
	case o.DeadlineFloorMinutes < 0:
		return errors.New("deadline_floor_minutes must not be negative")
	case o.StallMinutes < 0:
		return errors.New("stall_minutes must not be negative")
	case o.RunMaxLanes < 0:
		return errors.New("run_max_lanes must not be negative")
	}
	return nil
}

func orchestratorConfig(o *FleetOrchestratorPolicy, count int) OrchestratorConfig {
	c := OrchestratorConfig{Stage: "shadow", ScanK: defaultOrchestratorScanK, HubMinImporters: defaultOrchestratorHubMinImporters,
		DeadlineFloorMinutes: defaultOrchestratorDeadlineFloor, StallMinutes: defaultOrchestratorStallMinutes,
		RunMaxLanes: orchestratorRunLanesPerSlot * count}
	if o == nil {
		return c
	}
	if o.Stage != "" {
		c.Stage = o.Stage
	}
	if o.ScanK > 0 {
		c.ScanK = o.ScanK
	}
	if o.HubMinImporters > 0 {
		c.HubMinImporters = o.HubMinImporters
	}
	if o.DeadlineFloorMinutes > 0 {
		c.DeadlineFloorMinutes = o.DeadlineFloorMinutes
	}
	if o.StallMinutes > 0 {
		c.StallMinutes = o.StallMinutes
	}
	if o.RunMaxLanes > 0 {
		c.RunMaxLanes = o.RunMaxLanes
	}
	return c
}
