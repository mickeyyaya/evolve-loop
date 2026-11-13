package selfheal

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const stopEvidencePrefix = "main-red-stop:"

var errStopInBound = errors.New("the main-red stop is inside its bound")

type StopBound struct {
	EvolveDir string
	Bound     time.Duration
	Now       time.Time
	Emit      func(signalcenter.Event)
}

func (b StopBound) Check() error {
	incident := ""
	stop, err := mainstop.Update(b.EvolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		if !b.due(s) {
			return s, errStopInBound
		}
		id, err := b.record(s)
		incident = id
		return s.Escalate(b.Now), err
	})
	if errors.Is(err, errStopInBound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("the escalation of the stop past its bound: %w", err)
	}
	b.Emit(b.event(stop, incident))
	return nil
}

func (b StopBound) due(s mainstop.Stop) bool {
	return s.Escalated == nil && s.Overdue(b.Now, b.Bound)
}

func (b StopBound) record(s mainstop.Stop) (string, error) {
	o := b.observation(s)
	id := ""
	err := UpdateIncidents(Dir(b.EvolveDir), func(current []Incident) ([]Incident, error) {
		next, _ := DetectLive(current, o, b.Now)
		id = next[slices.IndexFunc(next, func(inc Incident) bool { return inc.Fingerprint == o.Fingerprint() })].ID
		return next, nil
	})
	return id, err
}

func (b StopBound) observation(s mainstop.Stop) Observation {
	return Observation{
		Class: ClassMainRed, Subject: mainBranch, Severity: signalcenter.SeverityIncident,
		Evidence: []Evidence{{ID: stopEvidencePrefix + s.MainSHA, At: b.Now}},
		Summary:  causeAndResume(s),
	}
}

func causeAndResume(s mainstop.Stop) string {
	return s.Cause() + "; to resume: " + s.ResumeCondition()
}

func (b StopBound) event(s mainstop.Stop, incident string) signalcenter.Event {
	return signalcenter.Event{
		Module: signalcenter.ModuleGateRepo, Origin: "selfheal.StopBound", Kind: signalcenter.KindLoopEscalation,
		Severity: signalcenter.SeverityIncident, Code: CodeMainRedStopEscalated,
		Reason: "main-red stop past its bound of " + b.Bound.String() + ": " + causeAndResume(s),
		Fields: map[string]string{
			"keys": baseattr.Names(s.Keys), "main": s.MainSHA, "fix_item": s.FixItem, "parked": s.ParkedCycles(),
			"opened_at": s.OpenedAt.UTC().Format(time.RFC3339), "bound": b.Bound.String(), "incident": incident,
		},
	}
}
