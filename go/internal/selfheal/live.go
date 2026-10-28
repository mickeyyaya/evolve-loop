package selfheal

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type FailureClass struct {
	Cycle       int    `json:"cycle"`
	Fingerprint string `json:"class_fingerprint"`
	Layer       string `json:"class_layer"`
	Recurrence  int    `json:"class_recurrence"`
}

func LoadFailureClass(evolveDir string, cycle int) (FailureClass, bool, error) {
	var fc FailureClass
	found, err := readOptionalJSON(paths.FailureDigestRecordPath(evolveDir, cycle), &fc)
	return fc, found && err == nil, err
}

func FromFailureClass(fc FailureClass, at time.Time) (Observation, bool) {
	if fc.Layer != dispositionrouter.LayerPipelineCode || fc.Recurrence < dispositionrouter.RecurrenceConsoleFloor {
		return Observation{}, false
	}
	return Observation{
		Class: ClassPipelineDefect, Subject: fc.Fingerprint, Severity: signalcenter.SeverityIncident,
		Evidence: []Evidence{{ID: "cycle:" + strconv.Itoa(fc.Cycle), At: at}},
		Summary:  fmt.Sprintf("the pipeline-code failure class %s failed %d times; the last is cycle %d", fc.Fingerprint, fc.Recurrence, fc.Cycle),
	}, true
}

func DetectLive(existing []Incident, o Observation, now time.Time) ([]Incident, []Transition) {
	at := slices.IndexFunc(existing, func(inc Incident) bool { return inc.Fingerprint == o.Fingerprint() })
	var prev Incident
	if at >= 0 {
		prev = existing[at]
	}
	next, kind := observe(prev, at >= 0, o, now)
	out := slices.Clone(existing)
	if at >= 0 {
		out[at] = next
	} else {
		out = append(out, next)
	}
	if kind == "" {
		return out, nil
	}
	return out, []Transition{{Kind: kind, Incident: next}}
}
