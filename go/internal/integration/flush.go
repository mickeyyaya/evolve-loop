package integration

import (
	"slices"
	"time"
)

type FlushTrigger string

const (
	TriggerNone               FlushTrigger = ""
	TriggerWaveEnd            FlushTrigger = "wave_end"
	TriggerCapReached         FlushTrigger = "cap_reached"
	TriggerMaxAge             FlushTrigger = "max_age"
	TriggerUnblocksQueuedWork FlushTrigger = "unblocks_queued_work"
	TriggerRiskyAlone         FlushTrigger = "risky_alone"
)

type Deliverable struct {
	ID                 string
	DeliveredAt        time.Time
	Risky              bool
	UnblocksQueuedWork bool
	InboxRank          int
	Deps               []string
	UnblocksCount      int
	VerificationCost   int
}

type FlushPolicy struct {
	Cap           int
	MaxAgeMinutes int
}

func DefaultFlushPolicy() FlushPolicy {
	return FlushPolicy{
		Cap:           DefaultCap,
		MaxAgeMinutes: DefaultMaxAgeMinutes,
	}
}

type FlushDecision struct {
	Trigger FlushTrigger
	Flush   []Deliverable
	Hold    []Deliverable
}

func DecideFlush(batch []Deliverable, waveEnded bool, policy FlushPolicy, now time.Time) FlushDecision {
	if len(batch) == 0 {
		return FlushDecision{Trigger: TriggerNone}
	}
	riskyIndex := slices.IndexFunc(batch, func(d Deliverable) bool {
		return d.Risky
	})
	if riskyIndex >= 0 {
		flush := []Deliverable{batch[riskyIndex]}
		hold := make([]Deliverable, 0, len(batch)-1)
		for i, d := range batch {
			if i != riskyIndex {
				hold = append(hold, d)
			}
		}
		return FlushDecision{
			Trigger: TriggerRiskyAlone,
			Flush:   flush,
			Hold:    hold,
		}
	}
	switch {
	case waveEnded:
		return FlushDecision{
			Trigger: TriggerWaveEnd,
			Flush:   slices.Clone(batch),
		}
	case policy.Cap > 0 && len(batch) >= policy.Cap:
		return FlushDecision{
			Trigger: TriggerCapReached,
			Flush:   slices.Clone(batch),
		}
	case hasMaxAge(batch, policy.MaxAgeMinutes, now):
		return FlushDecision{
			Trigger: TriggerMaxAge,
			Flush:   slices.Clone(batch),
		}
	case hasUnblocking(batch):
		return FlushDecision{
			Trigger: TriggerUnblocksQueuedWork,
			Flush:   slices.Clone(batch),
		}
	default:
		return FlushDecision{
			Trigger: TriggerNone,
			Hold:    slices.Clone(batch),
		}
	}
}

func hasMaxAge(batch []Deliverable, maxAgeMinutes int, now time.Time) bool {
	threshold := time.Duration(maxAgeMinutes) * time.Minute
	for _, d := range batch {
		if now.Sub(d.DeliveredAt) >= threshold {
			return true
		}
	}
	return false
}

func hasUnblocking(batch []Deliverable) bool {
	for _, d := range batch {
		if d.UnblocksQueuedWork {
			return true
		}
	}
	return false
}
