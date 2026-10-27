package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	liveReactionWouldBrake = "would_brake"
	liveReactionBraked     = "braked"
	liveReactionBrakeHeld  = "brake_already_engaged"
	liveReactionBrakeFail  = "brake_failed"
)

type liveReaction struct {
	evolveDir string
	stage     string
	now       func() time.Time
	emit      func(signalcenter.Event)
	warn      io.Writer
}

func newLiveReaction(evolveDir string, cfg policy.HealingLiveReactionPolicy, warn io.Writer) liveReaction {
	return liveReaction{evolveDir: evolveDir, stage: cfg.Stage, now: time.Now, warn: warn}
}

func (r liveReaction) arm(signals *signalcenter.Center) {
	if r.stage == policy.HealingStageOff {
		return
	}
	r.emit = signals.Emit
	signals.Subscribe(r.onEvent)
}

func (r liveReaction) onEvent(e signalcenter.Event) {
	if e.Kind != signalcenter.KindCycleSealed || e.Fields[cyclestate.SealedVerdictField] != cyclestate.VerdictFAIL {
		return
	}
	if err := r.react(e.Cycle); err != nil {
		fmt.Fprintf(r.warn, "[selfheal] WARN live incident of cycle %d: %v\n", e.Cycle, err)
	}
}

func (r liveReaction) react(cycle int) error {
	fc, found, err := selfheal.LoadFailureClass(r.evolveDir, cycle)
	if !found {
		return err
	}
	o, recurring := selfheal.FromFailureClass(fc, r.now())
	if !recurring {
		return nil
	}
	transitions, err := r.record(o)
	for _, t := range transitions {
		event := t.Event(r.stage)
		event.Origin, event.Fields["reaction"] = "liveReaction.react", r.brake(t.Incident)
		r.emit(event)
	}
	return err
}

func (r liveReaction) record(o selfheal.Observation) ([]selfheal.Transition, error) {
	var transitions []selfheal.Transition
	err := selfheal.UpdateIncidents(selfheal.Dir(r.evolveDir), func(current []selfheal.Incident) ([]selfheal.Incident, error) {
		var next []selfheal.Incident
		next, transitions = selfheal.DetectLive(current, o, r.now())
		return next, nil
	})
	if err != nil {
		return nil, err
	}
	return transitions, nil
}

func (r liveReaction) brake(inc selfheal.Incident) string {
	if r.stage != policy.HealingStageEnforce {
		return liveReactionWouldBrake
	}
	f, err := os.OpenFile(paths.LoopStopPath(r.evolveDir), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return liveReactionBrakeHeld
	}
	if err == nil {
		_, err = fmt.Fprintf(f, "%s incident %s\n", r.now().UTC().Format(time.RFC3339), inc.ID)
		err = errors.Join(err, f.Close())
	}
	if err != nil {
		fmt.Fprintf(r.warn, "[selfheal] WARN incident %s: engage the loop brake: %v\n", inc.ID, err)
		return liveReactionBrakeFail
	}
	return liveReactionBraked
}
