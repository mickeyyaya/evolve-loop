package selfheal

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeIncidentOpened   signalcenter.Code = "HEALING_INCIDENT_OPENED"
	CodeIncidentReopened signalcenter.Code = "HEALING_INCIDENT_REOPENED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleHealing, CodeIncidentOpened, "the incident detector opened an incident record under .evolve/incidents; fields incident, class, fingerprint, tier, state, transition, recurrence, stage (llm-isolation.md §18, §24)")
	signalcenter.RegisterCode(signalcenter.ModuleHealing, CodeIncidentReopened, "the incident detector reopened a closed incident, because its signal fired again after the close; the same fields as HEALING_INCIDENT_OPENED (llm-isolation.md §18, §22)")
}

var transitionCodes = map[string]signalcenter.Code{TransitionOpened: CodeIncidentOpened, TransitionReopened: CodeIncidentReopened}

func (t Transition) Event(stage string) signalcenter.Event {
	inc := t.Incident
	severity := inc.Severity
	if t.Kind == TransitionClosed {
		severity = signalcenter.SeverityInfo
	}
	return signalcenter.Event{
		Module: signalcenter.ModuleHealing, Kind: signalcenter.KindSelfhealIncident, Origin: "selfheal.Detect",
		Severity: severity, Code: transitionCodes[t.Kind],
		Reason: "incident " + inc.ID + " " + t.Kind + ": " + inc.Summary,
		Fields: map[string]string{
			"incident": inc.ID, "class": string(inc.Class), "fingerprint": inc.Fingerprint, "tier": inc.Tier,
			"state": string(inc.State), "transition": t.Kind, "recurrence": strconv.Itoa(inc.Recurrence), "stage": stage,
		},
	}
}
