package wavehealth

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Outcome string

const (
	OutcomeContinue Outcome = "CONTINUE"
	OutcomeBrake    Outcome = "BRAKE"
	OutcomeFixLocal Outcome = "FIX-LOCAL"
)

type Result string

const (
	ResultOK       Result = "OK"
	ResultBrake    Result = "BRAKE"
	ResultFixLocal Result = "FIX-LOCAL"
	ResultHeld     Result = "HELD"
	ResultGap      Result = "GAP"
	ResultSkip     Result = "SKIP"
)

type Known[T any] struct {
	Value T
	Known bool
}

func Have[T any](v T) Known[T] { return Known[T]{Value: v, Known: true} }

type LoopState struct {
	Running   bool
	BrakeHeld bool
}

type WaveFacts struct {
	Start    time.Time
	Started  int
	Shipped  int
	PoolLive Known[int]
}

type DigestAt struct {
	Digest core.FailureDigest
	Class  string
	At     time.Time
}

type Failures struct {
	Digests []DigestAt
	Breaker core.BlockerBreakerConfig
}

type CycleEvent struct {
	Cycle int
	Event dispatchevents.Event
}

type StallLine struct {
	Lane      string
	At        time.Time
	LastEvent time.Time
}

func (l StallLine) Recovered() bool { return l.LastEvent.After(l.At) }

type CIState struct {
	Available  bool
	Status     string
	Conclusion string
	RunID      string
}

type LoadSample struct {
	At      time.Time
	PerCore float64
}

type Snapshot struct {
	Now         time.Time
	Settings    policy.WaveCheckinConfig
	Loop        Known[LoopState]
	Wave        Known[WaveFacts]
	Failures    Known[Failures]
	Abnormal    Known[[]CycleEvent]
	Stalls      Known[[]StallLine]
	CI          Known[CIState]
	DiskFree    Known[uint64]
	LoadSamples Known[[]LoadSample]
}

type Step struct {
	ID        string `json:"id"`
	Reading   string `json:"reading"`
	Threshold string `json:"threshold"`
	Result    Result `json:"result"`
	Action    string `json:"action,omitempty"`
}

type Report struct {
	Steps   []Step  `json:"steps"`
	Outcome Outcome `json:"outcome"`
}
