package policy

import (
	"errors"
	"fmt"
)

type FleetOrchestratorPolicy struct {
	Stage                string             `json:"stage,omitempty"`
	Admission            string             `json:"admission,omitempty"`
	ScanK                int                `json:"scan_k,omitempty"`
	HubMinImporters      int                `json:"hub_min_importers,omitempty"`
	DeadlineFloorMinutes int                `json:"deadline_floor_minutes,omitempty"`
	StallMinutes         int                `json:"stall_minutes,omitempty"`
	RunMaxLanes          int                `json:"run_max_lanes,omitempty"`
	Window               *FleetWindowPolicy `json:"window,omitempty"`
}

type FleetWindowPolicy struct {
	Start *int `json:"start,omitempty"`
	Min   *int `json:"min,omitempty"`
}

type OrchestratorConfig struct {
	Stage                string
	Admission            string
	ScanK                int
	HubMinImporters      int
	DeadlineFloorMinutes int
	StallMinutes         int
	RunMaxLanes          int
	WindowStart          int
	WindowMin            int
}

const (
	defaultOrchestratorScanK           = 32
	defaultOrchestratorHubMinImporters = 120
	defaultOrchestratorDeadlineFloor   = 60
	defaultOrchestratorStallMinutes    = 30
	defaultOrchestratorWindowStart     = 4
	defaultOrchestratorWindowMin       = 4
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
	case o.Admission != "" && o.Admission != "shadow" && o.Admission != "enforce":
		return fmt.Errorf("admission %q is not \"shadow\" or \"enforce\"", o.Admission)
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
	return o.Window.validate()
}

func orchestratorConfig(o *FleetOrchestratorPolicy, count int) OrchestratorConfig {
	c := OrchestratorConfig{Stage: "shadow", Admission: "shadow", ScanK: defaultOrchestratorScanK, HubMinImporters: defaultOrchestratorHubMinImporters,
		DeadlineFloorMinutes: defaultOrchestratorDeadlineFloor, StallMinutes: defaultOrchestratorStallMinutes,
		RunMaxLanes: orchestratorRunLanesPerSlot * count,
		WindowStart: defaultOrchestratorWindowStart, WindowMin: defaultOrchestratorWindowMin}
	if o == nil {
		return c
	}
	if o.Stage != "" {
		c.Stage = o.Stage
	}
	if o.Admission != "" {
		c.Admission = o.Admission
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
	c.WindowStart, c.WindowMin = o.Window.resolve()
	return c
}

func (w *FleetWindowPolicy) validate() error {
	if w == nil {
		return nil
	}
	switch {
	case w.Start != nil && *w.Start < 1:
		return fmt.Errorf("window.start must be at least 1, got %d", *w.Start)
	case w.Min != nil && *w.Min < 1:
		return fmt.Errorf("window.min must be at least 1, got %d", *w.Min)
	}
	if start, floor := w.resolve(); floor > start {
		return fmt.Errorf("window.min %d is above window.start %d; set window.start to at least %d", floor, start, floor)
	}
	return nil
}

func (w *FleetWindowPolicy) resolve() (start, floor int) {
	start, floor = defaultOrchestratorWindowStart, defaultOrchestratorWindowMin
	if w == nil {
		return start, floor
	}
	if w.Start != nil {
		start = *w.Start
	}
	floor = min(floor, start)
	if w.Min != nil {
		floor = *w.Min
	}
	return start, floor
}

func (f *FleetPolicy) validateWindowStart() error {
	if f == nil || f.Orchestrator == nil || f.Orchestrator.Window == nil || f.Orchestrator.Window.Start == nil {
		return nil
	}
	if start, count := *f.Orchestrator.Window.Start, max(f.Count, 1); start > count {
		return fmt.Errorf("window.start %d is above fleet.count %d", start, count)
	}
	return nil
}

func (c OrchestratorConfig) OptimisticAdmission() bool {
	return c.Admission == "enforce"
}
