package policy

type WaveLatencyPolicy struct {
	MinLiveShare float64 `json:"min_live_share,omitempty"`
	MaxGateShare float64 `json:"max_gate_share,omitempty"`
}

func (p Policy) WaveLatencyMinLiveShare() float64 {
	const safeDefault = 0.8
	if p.WaveLatency == nil || p.WaveLatency.MinLiveShare <= 0 {
		return safeDefault
	}
	return p.WaveLatency.MinLiveShare
}

func (p Policy) WaveLatencyMaxGateShare() float64 {
	const safeDefault = 0.25
	if p.WaveLatency == nil || p.WaveLatency.MaxGateShare <= 0 {
		return safeDefault
	}
	return p.WaveLatency.MaxGateShare
}
