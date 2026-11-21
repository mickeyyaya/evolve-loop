package orchestrator

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type IdleCause string

const (
	CauseFamilyHeld       IdleCause = "family_held"
	CauseRunCap           IdleCause = "run_cap"
	CauseSupplyEmpty      IdleCause = "supply_empty"
	CauseAdmissionRefused IdleCause = "admission_refused"
	CauseBench            IdleCause = "bench"
	CauseOperatorStop     IdleCause = "operator_stop"
	CauseHostPressure     IdleCause = "host_pressure"
	CauseSystemHalt       IdleCause = "system_halt"
)

var IdleCauses = []IdleCause{CauseFamilyHeld, CauseRunCap, CauseSupplyEmpty, CauseAdmissionRefused, CauseBench, CauseOperatorStop, CauseHostPressure, CauseSystemHalt}

var idleCauseList = func() string {
	names := make([]string, len(IdleCauses))
	for i, c := range IdleCauses {
		names[i] = string(c)
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}()

func (c IdleCause) Valid() bool { return slices.Contains(IdleCauses, c) }

func idleSeverity(code signalcenter.Code) signalcenter.Severity {
	if code == CodePoolUnderfilled {
		return signalcenter.SeverityWarn
	}
	return signalcenter.SeverityInfo
}

func (p Publish) emitIdle(code signalcenter.Code, reason string, fields map[string]string, cause IdleCause) {
	stamped := make(map[string]string, len(fields)+1)
	for k, v := range fields {
		stamped[k] = v
	}
	stamped["cause"] = string(cause)
	p.emit("Pool.Run", code, idleSeverity(code), reason, stamped)
}

func (p Pool) idleCause(t fleet.PoolTransition) IdleCause {
	switch {
	case p.systemHalted():
		return CauseSystemHalt
	case t.Stopped:
		return CauseOperatorStop
	case t.Capped:
		return CauseRunCap
	case p.FamilyHolds.Summary() != "":
		return CauseFamilyHeld
	case t.Pending == 0:
		return CauseSupplyEmpty
	case p.hostCapped():
		return CauseHostPressure
	case p.Benched != nil && p.Benched():
		return CauseBench
	}
	return CauseAdmissionRefused
}

func (p Pool) hostCapped() bool {
	return p.Window != nil && p.Stage == StageEnforce && p.Window.HostCapped()
}
