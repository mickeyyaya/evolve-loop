package lanetime

import "time"

type Slots struct {
	Window            time.Duration `json:"window"`
	UsedSlotHours     float64       `json:"used_slot_hours"`
	CapacitySlotHours float64       `json:"capacity_slot_hours"`
	AvgLive           float64       `json:"avg_live"`
	Utilization       float64       `json:"utilization"`
}

func ratio(part, whole float64) float64 {
	if whole == 0 {
		return 0
	}
	return part / whole
}

func within(samples []Sample, from, to time.Time) []Sample {
	var in []Sample
	for _, s := range samples {
		if (from.IsZero() || !s.At.Before(from)) && (to.IsZero() || !s.At.After(to)) {
			in = append(in, s)
		}
	}
	return in
}

func SlotUse(samples []Sample, from, to time.Time) Slots {
	in := within(samples, from, to)
	if len(in) == 0 {
		return Slots{}
	}
	end := to
	if end.IsZero() {
		end = in[len(in)-1].At
	}
	var used, capacity float64
	for i, s := range in {
		next := end
		if i+1 < len(in) {
			next = in[i+1].At
		}
		hours := next.Sub(s.At).Hours()
		used += float64(s.Live) * hours
		capacity += float64(s.Target) * hours
	}
	window := end.Sub(in[0].At)
	return Slots{
		Window: window, UsedSlotHours: used, CapacitySlotHours: capacity,
		AvgLive: ratio(used, window.Hours()), Utilization: ratio(used, capacity),
	}
}

func ForPID(samples []Sample, pid int) []Sample {
	var own []Sample
	for _, s := range samples {
		if s.PID == pid {
			own = append(own, s)
		}
	}
	return own
}
