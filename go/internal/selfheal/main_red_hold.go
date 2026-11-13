package selfheal

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var ErrMainRedHold = errors.New("main is red and a stop holds")

type KeyRun func(ctx context.Context, rev string, reds baseattr.Reds) (red []baseattr.Key, err error)

type StopHold struct {
	EvolveDir string
	Enforced  bool
	Event     signalcenter.Event
	Emit      func(signalcenter.Event)
	Warn      io.Writer
	Run       KeyRun

	recorded bool
}

type stopLanding struct {
	name  string
	main  string
	proof mainstop.Proof
}

func (h *StopHold) Red(landing string) error {
	stop, holds := ReadStop(h.EvolveDir, h.Warn)
	if !holds {
		return nil
	}
	_, err := h.decide(stop, stopLanding{name: landing, main: stop.MainSHA, proof: mainstop.Proof{Base: stop.MainSHA}})
	return err
}

func (h *StopHold) Admit(ctx context.Context, main, tip string) ([]baseattr.Key, error) {
	stop, holds := ReadStop(h.EvolveDir, h.Warn)
	if !holds {
		return nil, nil
	}
	proof := mainstop.Proof{Base: main}
	red, err := h.Run(ctx, tip, mainstop.ProofReds(stop.Keys))
	if err == nil {
		proof = stop.Proven(main, red)
	} else {
		fmt.Fprintf(h.Warn, "WARN main-red stop: the keys of the stop could not be run on %s (%v), so the batch is not a proven fix\n", tip, err)
	}
	return h.decide(stop, stopLanding{name: "batch " + tip, main: main, proof: proof})
}

func (h *StopHold) decide(stop mainstop.Stop, l stopLanding) ([]baseattr.Key, error) {
	admission := stop.Admit(l.main, l.proof)
	if admission == mainstop.AdmitFix {
		return l.proof.Green, nil
	}
	if h.Enforced {
		return nil, fmt.Errorf("%w: %s is held (%s) while %s is red on main %s; land one batch with each fix and no other change, or ship the fix (fix item %s)",
			ErrMainRedHold, l.name, admission, baseattr.Names(stop.Keys), stop.MainSHA, stop.FixItem)
	}
	if !h.recorded {
		h.recorded = true
		h.Emit(StopRefusal{Landing: l.name, Head: l.main, Base: l.proof.Base, Admission: admission}.WouldRefuse(h.Event, stop))
	}
	return nil, nil
}

type StopRefusal struct {
	Landing   string
	Head      string
	Base      string
	Admission mainstop.Admission
}

func (r StopRefusal) Main(stop mainstop.Stop) string {
	if r.Admission == mainstop.RefuseRebase {
		return r.Head
	}
	return stop.MainSHA
}

func (r StopRefusal) WouldRefuse(base signalcenter.Event, stop mainstop.Stop) signalcenter.Event {
	keys := baseattr.Names(stop.Keys)
	e := base
	e.Module, e.Kind, e.Code = signalcenter.ModuleGateRepo, signalcenter.KindShipWarning, CodeMainRedStopWouldRefuse
	e.Severity = signalcenter.SeverityWarn
	e.Reason = "main-red stop (shadow): " + r.Landing + " lands, and enforce refuses it (" + r.Admission.String() + ") while " + keys + " is red on main"
	e.Fields = map[string]string{"keys": keys, "base": r.Base, "main": r.Main(stop), "admission": r.Admission.String(), "fix_item": stop.FixItem, "landing": r.Landing}
	return e
}
