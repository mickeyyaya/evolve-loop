package lanetime

import (
	"fmt"
	"time"
)

const (
	StatusOK      = "ok"
	StatusBreach  = "breach"
	StatusLeftOut = "left_out"

	CodeQuotaPaused  = "ORCHESTRATOR_QUOTA_PAUSED"
	CodeSupplyShort  = "LOOP_ORCH_SUPPLY_SHORT"
	BrakeStopReason  = "loop_operator_brake"
	kindLoopHalt     = "loop.halt"
	BreachRunToAlarm = 2
)

type Mark struct {
	At         time.Time
	PID        int
	Code       string
	Kind       string
	StopReason string
}

type Bounds struct {
	MinLiveShare float64
	MaxGateShare float64
}

type WaveInput struct {
	Number  int
	PID     int
	Start   time.Time
	End     time.Time
	Cycles  []Cycle
	Samples []Sample
	Marks   []Mark
}

type Verdict struct {
	Wave      int      `json:"wave"`
	Status    string   `json:"status"`
	Reasons   []string `json:"reasons,omitempty"`
	LiveShare float64  `json:"live_share"`
	GateShare float64  `json:"gate_share"`
}

func (w WaveInput) owns(m Mark) bool {
	if w.PID > 0 && m.PID == w.PID {
		return true
	}
	return !m.At.Before(w.Start) && !m.At.After(w.End)
}

func (w WaveInput) marks() []Mark {
	var own []Mark
	for _, m := range w.Marks {
		if w.owns(m) {
			own = append(own, m)
		}
	}
	return own
}

func (w WaveInput) pauseReasons(marks []Mark) []string {
	var reasons []string
	paused := false
	for _, c := range w.Cycles {
		paused = paused || c.Paused
	}
	for _, m := range marks {
		paused = paused || m.Code == CodeQuotaPaused
	}
	if paused {
		reasons = append(reasons, "quota pause")
	}
	for _, m := range marks {
		switch {
		case m.Kind != kindLoopHalt:
		case m.StopReason == BrakeStopReason:
			reasons = append(reasons, "operator brake ("+BrakeStopReason+")")
		case m.StopReason != "":
			reasons = append(reasons, "loop stop_reason "+m.StopReason)
		default:
			reasons = append(reasons, "loop stop (halt signal without a stop_reason)")
		}
	}
	return reasons
}

func supplyShort(marks []Mark) bool {
	for _, m := range marks {
		if m.Code == CodeSupplyShort {
			return true
		}
	}
	return false
}

func Check(w WaveInput, b Bounds) Verdict {
	v := Verdict{Wave: w.Number, Status: StatusOK}
	leftOut := func(reasons ...string) Verdict {
		v.Status, v.Reasons = StatusLeftOut, reasons
		return v
	}
	if w.End.IsZero() {
		return leftOut("wave still open")
	}
	marks := w.marks()
	if reasons := w.pauseReasons(marks); len(reasons) > 0 {
		return leftOut(reasons...)
	}
	samples, from, to := WaveSamples(w.Samples, w.PID, w.Start, w.End)
	slots := SlotUse(samples, from, to)
	phases := Phases(w.Cycles)
	var span, host time.Duration
	for _, p := range phases.Phases {
		span += p.Time
		host += p.Host
	}
	if slots.Window == 0 && span == 0 {
		return leftOut("no orchestration sample and no phase span lie in the wave")
	}
	var breaches []string
	if slots.Window > 0 {
		v.LiveShare = slots.Utilization
		if !supplyShort(marks) && v.LiveShare < b.MinLiveShare {
			breaches = append(breaches, fmt.Sprintf("live lanes %.1f%% of the target, below the floor of %.1f%%", v.LiveShare*100, b.MinLiveShare*100))
		}
	}
	if span > 0 {
		v.GateShare = ratio(float64(host), float64(span))
		if v.GateShare > b.MaxGateShare {
			breaches = append(breaches, fmt.Sprintf("host gate share %.1f%%, above the ceiling of %.1f%%", v.GateShare*100, b.MaxGateShare*100))
		}
	}
	if len(breaches) > 0 {
		v.Status, v.Reasons = StatusBreach, breaches
	}
	return v
}

func BreachRun(verdicts []Verdict) []Verdict {
	var run []Verdict
	for i := len(verdicts) - 1; i >= 0; i-- {
		switch verdicts[i].Status {
		case StatusLeftOut:
		case StatusBreach:
			run = append([]Verdict{verdicts[i]}, run...)
		default:
			return run
		}
	}
	return run
}
