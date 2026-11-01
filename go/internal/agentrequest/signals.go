package agentrequest

import (
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const origin = "agentrequest.Channel"

func (c Channel) emitRequest(cell Cell, rec Record, req Request) {
	c.Emit(signalcenter.Event{
		Cycle: cell.Cycle, Phase: cell.Phase, Module: signalcenter.ModuleIsolation, Origin: origin, Kind: signalcenter.KindIsolationRequest,
		Severity: signalcenter.SeverityInfo, Reason: fmt.Sprintf("the %s agent wrote the %s request %s", cell.Phase, rec.Kind, rec.Request),
		Fields: map[string]string{"request": rec.Request, "kind": rec.Kind, "verb": req.What.Verb, "role": rec.Role, "cell": cell.Phase, "depth": strconv.Itoa(rec.Depth)},
	})
}

func (c Channel) emitDecision(cell Cell, rec Record) {
	severity, code := signalcenter.SeverityInfo, signalcenter.Code("")
	switch rec.Decision {
	case DecisionRefused:
		severity, code = signalcenter.SeverityWarn, isolation.CodeRequestRefused
	case DecisionFailed:
		severity, code = signalcenter.SeverityWarn, isolation.CodeRequestFailed
	}
	c.Emit(signalcenter.Event{
		Cycle: cell.Cycle, Phase: cell.Phase, Module: signalcenter.ModuleIsolation, Origin: origin, Kind: signalcenter.KindIsolationRequestDecision,
		Severity: severity, Code: code, Reason: rec.Summary,
		Fields: map[string]string{"request": rec.Request, "kind": rec.Kind, "role": rec.Role, "decision": rec.Decision, "precondition": rec.Precondition,
			"reason": rec.Reason, "result": rec.RunLabel, "stage": rec.Stage},
	})
}
