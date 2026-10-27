package selfheal

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Class string

const (
	ClassMainRed        Class = "main-red"
	ClassCIHang         Class = "ci-hang"
	ClassFlakyTest      Class = "flaky-test"
	ClassQuality        Class = "quality"
	ClassIsolation      Class = "isolation"
	ClassStall          Class = "stall"
	ClassSystem         Class = "system"
	ClassZeroShip       Class = "zero-ship"
	ClassLaneStarvation Class = "lane-starvation"
	ClassRepairLoop     Class = "repair-loop"
	ClassClaimChurn     Class = "claim-churn"
	ClassUsageBlind     Class = "usage-blind"
)

const (
	TierKernel   = "T0"
	TierPipeline = "T1"
	TierItem     = "T2"
)

var classTiers = map[Class]string{
	ClassMainRed: TierPipeline, ClassCIHang: TierPipeline, ClassFlakyTest: TierItem, ClassQuality: TierPipeline,
	ClassIsolation: TierPipeline, ClassStall: TierPipeline, ClassSystem: TierPipeline, ClassZeroShip: TierPipeline,
	ClassLaneStarvation: TierPipeline, ClassRepairLoop: TierPipeline, ClassClaimChurn: TierItem, ClassUsageBlind: TierPipeline,
}

func Classes() []Class {
	return []Class{ClassMainRed, ClassCIHang, ClassFlakyTest, ClassQuality, ClassIsolation, ClassStall, ClassSystem, ClassZeroShip, ClassLaneStarvation, ClassRepairLoop, ClassClaimChurn, ClassUsageBlind}
}

func tierOf(c Class, protected []string) string {
	if len(protected) > 0 {
		return TierKernel
	}
	return classTiers[c]
}

type State string

const (
	StateOpen   State = "open"
	StateClosed State = "closed"
)

const RecordSchema = "evolve.incident/1"

type Incident struct {
	Schema      string                `json:"schema"`
	ID          string                `json:"id"`
	Fingerprint string                `json:"fingerprint"`
	Class       Class                 `json:"class"`
	Subject     string                `json:"subject"`
	Severity    signalcenter.Severity `json:"severity"`
	Tier        string                `json:"tier"`
	State       State                 `json:"state"`
	Sources     []string              `json:"sources"`
	Recurrence  int                   `json:"recurrence"`
	Reopened    int                   `json:"reopened"`
	Attempts    int                   `json:"attempts"`
	ReproTest   string                `json:"repro_test"`
	Summary     string                `json:"summary"`
	OpenedAt    time.Time             `json:"opened_at"`
	LastSeenAt  time.Time             `json:"last_seen_at"`
	ClosedAt    *time.Time            `json:"closed_at"`
}

const (
	CondReproGreen    = "repro-green"
	CondSignalClear   = "signal-clear"
	CondQualityHeld   = "quality-held"
	CondRetrospective = "retrospective"
)

type Conditions struct {
	ReproGreen    bool
	SignalClear   bool
	QualityHeld   bool
	Retrospective bool
}

func (c Conditions) missing() []string {
	var out []string
	for _, cond := range []struct {
		held bool
		name string
	}{{c.ReproGreen, CondReproGreen}, {c.SignalClear, CondSignalClear}, {c.QualityHeld, CondQualityHeld}, {c.Retrospective, CondRetrospective}} {
		if !cond.held {
			out = append(out, cond.name)
		}
	}
	return out
}

func (i Incident) Close(c Conditions, at time.Time) (Incident, []string) {
	if missing := c.missing(); len(missing) > 0 {
		return i, missing
	}
	i.State, i.ClosedAt = StateClosed, &at
	return i, nil
}

func incidentID(fingerprint string, at time.Time) string {
	return "inc-" + at.UTC().Format(time.DateOnly) + "-" + fingerprintHash(fingerprint)
}

func fingerprintHash(fingerprint string) string {
	sum := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(sum[:4])
}
