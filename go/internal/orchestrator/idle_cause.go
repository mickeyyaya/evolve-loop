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
	CauseDiskFloor        IdleCause = "disk_floor"
)

var IdleCauses = []IdleCause{CauseFamilyHeld, CauseRunCap, CauseSupplyEmpty, CauseAdmissionRefused, CauseBench, CauseOperatorStop, CauseHostPressure, CauseSystemHalt, CauseDiskFloor}

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
	case t.Held:
		return CauseDiskFloor
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
	case p.FamilyHolds.refusedCount() > 0:
		return CauseAdmissionRefused
	case p.benched():
		return CauseBench
	}
	return CauseAdmissionRefused
}

func (p Pool) benched() bool { return p.Benched != nil && p.Benched() }

const refusedFieldLimit = 5

func (p Pool) idleFields(cause IdleCause) map[string]string {
	fields := map[string]string{}
	refused := p.FamilyHolds.Refused(refusedFieldLimit)
	if len(refused) == 0 {
		return fields
	}
	ids := make([]string, len(refused))
	by := make([]string, len(refused))
	for i, d := range refused {
		ids[i] = d.ID
		by[i] = d.ID + " (footprint " + strings.Join(d.Cells, ",") + ", held by " + d.BlockedBy + ")"
	}
	fields["refused"] = strings.Join(ids, ",")
	fields["refused_by"] = strings.Join(by, "; ")
	if cause == CauseAdmissionRefused && p.benched() {
		fields["also"] = string(CauseBench)
	}
	return fields
}

func (p Pool) hostCapped() bool {
	return p.Window != nil && p.Stage == StageEnforce && p.Window.HostCapped()
}
