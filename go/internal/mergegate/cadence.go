package mergegate

import "fmt"

type Cadence string

const (
	CadenceDefer           Cadence = "defer"
	CadencePerWave         Cadence = "per-wave"
	CadenceBatched         Cadence = "batched"
	CadenceFeatureComplete Cadence = "feature-complete"
)

type DecisionInput struct {
	WavesDone       int
	WavesTotal      int
	PendingWaves    int
	ChurnLOC        int
	CarryoverAgeMax int
	AuditPassed     bool
	CIGreen         bool
	LedgerVerified  bool
	SeverityBlocks  bool
	Conflicts       int
}

type Thresholds struct {
	BatchWaveCount       int
	BatchChurnLOC        int
	CarryoverStallCycles int
}

type Decision struct {
	Fire    bool
	Cadence Cadence
	Reason  string
}

func DecideCadence(in DecisionInput, th Thresholds) Decision {
	if d, blocked := safetyDefer(in); blocked {
		return d
	}
	if in.WavesTotal > 0 && in.WavesDone >= in.WavesTotal {
		return Decision{Fire: true, Cadence: CadenceFeatureComplete, Reason: "all waves complete"}
	}
	if th.CarryoverStallCycles > 0 && in.CarryoverAgeMax >= th.CarryoverStallCycles {
		return Decision{Fire: true, Cadence: CadenceBatched,
			Reason: fmt.Sprintf("carryover stall (%d >= %d cycles) forces a promotion", in.CarryoverAgeMax, th.CarryoverStallCycles)}
	}
	if th.BatchWaveCount <= 1 {
		return Decision{Fire: true, Cadence: CadencePerWave, Reason: "per-wave cadence"}
	}
	if in.PendingWaves >= th.BatchWaveCount {
		return Decision{Fire: true, Cadence: CadenceBatched,
			Reason: fmt.Sprintf("batch of %d waves reached", th.BatchWaveCount)}
	}
	if th.BatchChurnLOC > 0 && in.ChurnLOC > th.BatchChurnLOC {
		return Decision{Fire: true, Cadence: CadenceBatched,
			Reason: fmt.Sprintf("batch churn ceiling exceeded (%d > %d LOC)", in.ChurnLOC, th.BatchChurnLOC)}
	}
	return Decision{Fire: false, Cadence: CadenceDefer,
		Reason: fmt.Sprintf("accumulating batch (%d/%d waves)", in.PendingWaves, th.BatchWaveCount)}
}

func safetyDefer(in DecisionInput) (Decision, bool) {
	switch {
	case !in.AuditPassed:
		return deferFor("audit did not pass"), true
	case !in.CIGreen:
		return deferFor("CI is not green"), true
	case !in.LedgerVerified:
		return deferFor("ledger verification failed"), true
	case in.Conflicts > 0:
		return deferFor(fmt.Sprintf("%d unresolved merge conflict(s)", in.Conflicts)), true
	case in.SeverityBlocks:
		return deferFor("build severity at/above the block threshold"), true
	}
	return Decision{}, false
}

func deferFor(reason string) Decision {
	return Decision{Fire: false, Cadence: CadenceDefer, Reason: reason}
}
