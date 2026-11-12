package ship

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type stopCheck struct {
	opts     *Options
	warn     io.Writer
	attr     baseattr.Attributor
	stop     mainstop.Stop
	main     string
	enforced bool
}

func checkMainRedStop(ctx context.Context, opts *Options, warn io.Writer) ([]baseattr.Key, error) {
	stop, holds := selfheal.ReadStop(opts.evolveDir(), warn)
	if !holds {
		return nil, nil
	}
	tree, baseRef, err := repoContractGateRoot(opts)
	if err != nil {
		return nil, err
	}
	gates := shipGates(opts, warn)
	bound := selfheal.StopBound{EvolveDir: opts.evolveDir(), Bound: gates.MainRedStopBound, Now: opts.clock(), Emit: opts.Signals.Emit}
	if err := bound.Check(); err != nil {
		fmt.Fprintf(warn, "[ship] WARN main-red stop: %v\n", err)
	}
	c := stopCheck{opts: opts, warn: warn, attr: attributorOf(opts, gates), stop: stop, enforced: gates.MainRedStopEnforced()}
	c.main = c.resolved(ctx, c.attr.MainRef)
	return c.admit(c.proof(ctx, tree, c.resolved(ctx, baseRef)))
}

func (c stopCheck) resolved(ctx context.Context, rev string) string {
	sha, err := c.attr.Resolve(ctx, rev)
	if err != nil {
		fmt.Fprintf(c.warn, "[ship] WARN main-red stop: %s could not be resolved (%v), so the lane is not a proven fix\n", rev, err)
		return ""
	}
	return sha
}

func (c stopCheck) proof(ctx context.Context, tree, base string) mainstop.Proof {
	if base == "" || base != c.main {
		return mainstop.Proof{Base: base}
	}
	red, err := c.attr.RunIn(ctx, tree, core.StopProofReds(c.stop.Keys))
	if err != nil {
		fmt.Fprintf(c.warn, "[ship] WARN main-red stop: the keys of the stop could not be run in the lane (%v), so the lane is not a proven fix\n", err)
		return mainstop.Proof{Base: base}
	}
	return c.stop.Proven(base, red)
}

func (c stopCheck) admit(proof mainstop.Proof) ([]baseattr.Key, error) {
	admission := c.stop.Admit(c.main, proof)
	if admission == mainstop.AdmitFix {
		return proof.Green, nil
	}
	refused := selfheal.StopRefusal{Landing: "the lane", Head: c.main, Base: proof.Base, Admission: admission}
	if !c.enforced {
		base := signalcenter.Event{Cycle: c.opts.CycleID, RunID: c.opts.RunID, Phase: phaseName, Origin: "checkMainRedStop"}
		c.opts.Signals.Emit(refused.WouldRefuse(base, c.stop))
		return nil, nil
	}
	refusal := c.refusal(refused)
	return nil, baseRedShipError(refusal.code, refusal.class, refusal.verdict, refusal.message)
}

type stopRefusal struct {
	code    shiperr.ShipErrorCode
	class   shiperr.ShipErrorClass
	verdict baseattr.Verdict
	message string
}

func (c stopCheck) refusal(r selfheal.StopRefusal) stopRefusal {
	if r.Admission == mainstop.RefuseRebase {
		return stopRefusal{
			code: shiperr.CodeGitFleetRebaseNeeded, class: shiperr.ShipClassTransient,
			verdict: baseattr.Verdict{BaseRed: c.stop.Keys, Base: r.Base, Main: r.Main(c.stop)},
			message: "main is red and a stop holds, and the lane is not on the main tip; rebase the lane onto main and verify again",
		}
	}
	return stopRefusal{
		code: shiperr.CodeMainRedPark, class: shiperr.ShipClassPrecondition,
		verdict: baseattr.Verdict{BaseRed: c.stop.Keys, MainRed: c.stop.Keys, Base: r.Base, Main: r.Main(c.stop)},
		message: "main is red and a stop holds, and the lane turns no key of the stop green; the lane waits for a green main",
	}
}

type stopRelease struct {
	opts *Options
	warn io.Writer
}

func (r stopRelease) landedFix(green []baseattr.Key, res RunResult, err error) RunResult {
	if err != nil || res.ExitCode != ExitOK || res.CommitSHA == "" {
		return res
	}
	by := fmt.Sprintf("cycle %d %s", r.opts.CycleID, res.CommitSHA)
	if rerr := r.release(green, by); rerr != nil {
		res.Logs = append(slices.Clone(res.Logs), "[ship] WARN: main-red stop release: "+rerr.Error())
	}
	return res
}

func (r stopRelease) greenOnMain(v baseattr.Verdict) {
	if v.Main == "" {
		return
	}
	if err := r.release(mainstop.Without(v.BaseRed, v.MainRed), "main "+v.Main); err != nil {
		fmt.Fprintf(r.warn, "[ship] WARN main-red stop release: %v\n", err)
	}
}

func (r stopRelease) release(green []baseattr.Key, by string) error {
	return selfheal.StopRelease{
		EvolveDir: r.opts.evolveDir(), Now: r.opts.clock(), Emit: r.opts.Signals.Emit,
		Event:   signalcenter.Event{Cycle: r.opts.CycleID, RunID: r.opts.RunID, Phase: phaseName, Origin: "stopRelease.release"},
		Consume: r.consumeFixItem,
	}.Keys(green, by)
}

func (r stopRelease) consumeFixItem(id string) error {
	mover := inboxmover.Options{ProjectRoot: r.opts.ProjectRoot, Stderr: r.warn, Signals: r.opts.Signals}
	return inboxmover.ConsumeFixItem(mover, id, inboxmover.PromoteOpts{Cycle: fmt.Sprint(r.opts.CycleID)})
}
