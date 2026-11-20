package hostpressure

import (
	"math"
	"time"
)

const (
	CausePressureHigh = "pressure_high"
	CausePressureLow  = "pressure_low"
	CauseBlind        = "blind"
	CauseHold         = "hold"
)

type Config struct {
	LoadHigh         float64
	LoadLow          float64
	MemMinPct        float64
	SwapMaxMB        float64
	DecreaseFactor   float64
	Step             int
	SmoothSamples    int
	Floor            int
	Ceiling          int
	Cooldown         time.Duration
	IncreaseCooldown time.Duration
}

type State struct {
	Cap     int
	Changed time.Time
	Scores  []float64
	Blind   int
}

type Decision struct {
	Old   int
	New   int
	Cause string
	Score float64
	Blind bool
}

func NewState(ceiling int) State { return State{Cap: ceiling} }

func Score(r Reading, cfg Config) float64 {
	return math.Max(r.LoadPerCore()/cfg.LoadHigh, math.Max(cfg.MemMinPct/r.MemAvailablePct, r.SwapUsedMB/cfg.SwapMaxMB))
}

func Decide(s State, r Reading, ok bool, cfg Config, now time.Time) (State, Decision) {
	if !ok {
		s.Blind++
		return s, Decision{Old: s.Cap, New: s.Cap, Cause: CauseBlind, Blind: s.Blind == cfg.SmoothSamples}
	}
	s.Blind = 0
	score := Score(r, cfg)
	s.Scores = window(s.Scores, score, cfg.SmoothSamples)
	mean := average(s.Scores)
	next, cause := s.Cap, CauseHold
	since := now.Sub(s.Changed)
	switch {
	case mean > 1 && since >= cfg.Cooldown && s.Cap > cfg.Floor:
		next, cause = max(cfg.Floor, int(float64(s.Cap)*cfg.DecreaseFactor)), CausePressureHigh
	case mean < cfg.LoadLow/cfg.LoadHigh && since >= cfg.IncreaseCooldown && s.Cap < cfg.Ceiling:
		next, cause = min(cfg.Ceiling, s.Cap+cfg.Step), CausePressureLow
	}
	d := Decision{Old: s.Cap, New: next, Cause: cause, Score: score}
	if next != s.Cap {
		s.Cap, s.Changed = next, now
	}
	return s, d
}

func window(scores []float64, score float64, keep int) []float64 {
	all := append(append([]float64(nil), scores...), score)
	return all[max(len(all)-max(keep, 1), 0):]
}

func average(scores []float64) float64 {
	sum := 0.0
	for _, v := range scores {
		sum += v
	}
	return sum / float64(len(scores))
}
