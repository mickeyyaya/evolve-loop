package selfheal

import (
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	codeStall       signalcenter.Code = "LIVENESS_PHASE_STALLED"
	codeZeroShip    signalcenter.Code = "LOOP_ZERO_SHIP_HALT"
	codeSupplyShort signalcenter.Code = "LOOP_ORCH_SUPPLY_SHORT"
	launcherSource                    = "launcher"
)

type signalRule func(signalcenter.Event) (Observation, bool)

var signalRules = []signalRule{qualityBreach, enforcedViolation, phaseStall, systemFailure, zeroShipHalt, supplyShort}

func FromSignals(events []signalcenter.Event) []Observation {
	set := newObservations()
	for _, e := range events {
		at, err := time.Parse(time.RFC3339Nano, e.TS)
		if err != nil {
			continue
		}
		for _, rule := range signalRules {
			if o, ok := rule(e); ok {
				set.add(o, Evidence{ID: signalEvidenceID(e), At: at})
			}
		}
	}
	return set.list()
}

func qualityBreach(e signalcenter.Event) (Observation, bool) {
	if e.Kind != signalcenter.KindQualityBreach || !strings.HasSuffix(string(e.Code), "_BREACH") {
		return Observation{}, false
	}
	return Observation{Class: ClassQuality, Subject: string(e.Code), Severity: signalcenter.SeverityWarn, Summary: "a quality index breached its threshold: " + string(e.Code)}, true
}

func enforcedViolation(e signalcenter.Event) (Observation, bool) {
	if e.Kind != signalcenter.KindIsolationViolation || e.Fields["stage"] != "enforce" || e.Fields["source"] == launcherSource {
		return Observation{}, false
	}
	return Observation{Class: ClassIsolation, Subject: string(e.Code) + ":" + e.Fields["rule"], Severity: signalcenter.SeverityWarn, Summary: "a cell tried an action that an invariant forbids, and the pipeline refused it at enforce: " + string(e.Code)}, true
}

func phaseStall(e signalcenter.Event) (Observation, bool) {
	if e.Code != codeStall {
		return Observation{}, false
	}
	phase := e.Phase
	if phase == "" {
		phase = "unknown"
	}
	return Observation{Class: ClassStall, Subject: string(e.Code) + ":" + phase, Severity: signalcenter.SeverityWarn, Summary: "a phase pane stalled in phase " + phase}, true
}

func systemFailure(e signalcenter.Event) (Observation, bool) {
	if e.Kind != signalcenter.KindSystemFailure {
		return Observation{}, false
	}
	return Observation{Class: ClassSystem, Subject: string(e.Code), Severity: e.Severity, Summary: "a system-level failure: " + string(e.Code)}, true
}

func zeroShipHalt(e signalcenter.Event) (Observation, bool) {
	if e.Code != codeZeroShip {
		return Observation{}, false
	}
	return Observation{Class: ClassZeroShip, Subject: "loop", Severity: signalcenter.SeverityIncident, Summary: "the zero-ship breaker halted the loop"}, true
}

func supplyShort(e signalcenter.Event) (Observation, bool) {
	if e.Code != codeSupplyShort {
		return Observation{}, false
	}
	return Observation{Class: ClassLaneStarvation, Subject: "supply", Severity: signalcenter.SeverityWarn, Summary: fmt.Sprintf("the lane supply is short: %s admissible for a window of %s", e.Fields["admissible"], e.Fields["window"])}, true
}

func signalEvidenceID(e signalcenter.Event) string {
	return fmt.Sprintf("signal:%d.%d", e.PID, e.Seq)
}
