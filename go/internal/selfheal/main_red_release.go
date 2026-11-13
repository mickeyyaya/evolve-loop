package selfheal

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var errNoStopHolds = errors.New("no main-red stop holds")

func ReadStop(evolveDir string, warn io.Writer) (mainstop.Stop, bool) {
	stop, _, err := mainstop.Read(evolveDir)
	if err != nil {
		fmt.Fprintf(warn, "WARN main-red stop: the record could not be read (%v), so the stop does not hold\n", err)
		return mainstop.Stop{}, false
	}
	return stop, stop.Holds()
}

type StopRelease struct {
	EvolveDir string
	Now       time.Time
	Event     signalcenter.Event
	Emit      func(signalcenter.Event)
	Consume   func(fixItem string) error
}

func (r StopRelease) Keys(green []baseattr.Key, by string) error {
	if len(green) == 0 {
		return nil
	}
	next, err := mainstop.Update(r.EvolveDir, func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		if !s.Holds() {
			return s, errNoStopHolds
		}
		return s.Released(green, by, r.Now), nil
	})
	if errors.Is(err, errNoStopHolds) {
		return nil
	}
	if err != nil || next.Holds() {
		return err
	}
	itemErr := r.Consume(next.FixItem)
	r.Emit(r.released(next, itemErr))
	return itemErr
}

func (r StopRelease) released(stop mainstop.Stop, itemErr error) signalcenter.Event {
	e := r.Event
	e.Module, e.Kind, e.Code = signalcenter.ModuleGateRepo, signalcenter.KindGateCorrected, CodeMainRedStopReleased
	e.Severity = signalcenter.SeverityInfo
	e.Reason = "main-red stop released by " + stop.ReleasedBy + ": each key is green, and the parked lanes resume in the next pool fill"
	e.Fields = map[string]string{"main": stop.MainSHA, "fix_item": stop.FixItem, "released_by": stop.ReleasedBy, "parked": stop.ParkedCycles(), "fix_item_error": ""}
	if itemErr != nil {
		e.Severity = signalcenter.SeverityWarn
		e.Fields["fix_item_error"] = itemErr.Error()
	}
	return e
}
