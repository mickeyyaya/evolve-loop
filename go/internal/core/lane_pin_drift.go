package core

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodeLanePinForged signalcenter.Code = "ISOLATION_LANE_PIN_FORGED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeLanePinForged,
		"the lane-scope.json of a workspace lists other items than the host record .evolve/lineage/cycle-N.json; the host kept the record and no reader used the file; one event per cycle, at the host seam that follows a cell; fields.record, fields.file, fields.path")
}

func (cr *cycleRun) reportLanePinDrift(phase Phase) {
	if cr.lanePinDriftSeen {
		return
	}
	d, drifted := committedset.LaneDrift(cr.cs.WorkspacePath)
	if !drifted {
		return
	}
	cr.lanePinDriftSeen = true
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.o.signalRunID(), Phase: string(phase),
		Module: signalcenter.ModuleIsolation, Origin: "cycleRun.reportLanePinDrift",
		Kind: signalcenter.KindIsolationViolation, Severity: signalcenter.SeverityWarn, Code: CodeLanePinForged,
		Reason: "the pin file " + d.Path + " lists [" + strings.Join(d.File, ", ") + "], but the host record lists [" + strings.Join(d.Record, ", ") +
			"]; the host kept the host record and ignored the file. A cell may have written the file. Next step: an agent must not edit lane-scope.json; an operator reads the output of the " +
			string(phase) + " cell to find the writer",
		Fields: map[string]string{"record": strings.Join(d.Record, ","), "file": strings.Join(d.File, ","), "path": d.Path},
	})
}
