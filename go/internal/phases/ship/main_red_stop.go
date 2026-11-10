package ship

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	admissionRebase  = "rebase"
	admissionNotAFix = "not-a-fix"
)

var errNoStopHolds = errors.New("no main-red stop holds")

type stopCheck struct {
	opts     *Options
	warn     io.Writer
	attr     baseattr.Attributor
	stop     mainstop.Stop
	main     string
	enforced bool
}

func checkMainRedStop(ctx context.Context, opts *Options, warn io.Writer) ([]baseattr.Key, error) {
	stop, _, err := mainstop.Read(opts.evolveDir())
	if err != nil {
		fmt.Fprintf(warn, "[ship] WARN main-red stop: the record could not be read (%v), so the stop does not hold the lane\n", err)
		return nil, nil
	}
	if !stop.Holds() {
		return nil, nil
	}
	tree, baseRef, err := repoContractGateRoot(opts)
	if err != nil {
		return nil, err
	}
	gates := shipGates(opts, warn)
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
	red, err := c.attr.RunIn(ctx, tree, baseattr.Reds{Keys: c.stop.Keys, Flags: covchanged.TagFlags(covchanged.DefaultTags)})
	if err != nil {
		fmt.Fprintf(c.warn, "[ship] WARN main-red stop: the keys of the stop could not be run in the lane (%v), so the lane is not a proven fix\n", err)
		return mainstop.Proof{Base: base}
	}
	return mainstop.Proof{Base: base, Green: keysNotIn(c.stop.Keys, red)}
}

func keysNotIn(keys, drop []baseattr.Key) []baseattr.Key {
	return slices.DeleteFunc(slices.Clone(keys), func(k baseattr.Key) bool { return slices.Contains(drop, k) })
}

func (c stopCheck) admit(proof mainstop.Proof) ([]baseattr.Key, error) {
	admission := c.stop.Admit(c.main, proof)
	if admission == mainstop.AdmitFix {
		return proof.Green, nil
	}
	refusal := c.refusal(proof, admission)
	if !c.enforced {
		c.wouldRefuse(refusal)
		return nil, nil
	}
	return nil, baseRedShipError(refusal.code, refusal.class, refusal.verdict, refusal.message)
}

type stopRefusal struct {
	admission string
	code      shiperr.ShipErrorCode
	class     shiperr.ShipErrorClass
	verdict   baseattr.Verdict
	message   string
}

func (c stopCheck) refusal(proof mainstop.Proof, admission mainstop.Admission) stopRefusal {
	if admission == mainstop.RefuseRebase {
		return stopRefusal{
			admission: admissionRebase, code: shiperr.CodeGitFleetRebaseNeeded, class: shiperr.ShipClassTransient,
			verdict: baseattr.Verdict{BaseRed: c.stop.Keys, Base: proof.Base, Main: c.main},
			message: "main is red and a stop holds, and the lane is not on the main tip; rebase the lane onto main and verify again",
		}
	}
	return stopRefusal{
		admission: admissionNotAFix, code: shiperr.CodeMainRedPark, class: shiperr.ShipClassPrecondition,
		verdict: baseattr.Verdict{BaseRed: c.stop.Keys, MainRed: c.stop.Keys, Base: proof.Base, Main: c.stop.MainSHA},
		message: "main is red and a stop holds, and the lane turns no key of the stop green; the lane waits for a green main",
	}
}

func (c stopCheck) wouldRefuse(r stopRefusal) {
	keys := baseattr.Names(c.stop.Keys)
	c.opts.Signals.Emit(signalcenter.Event{
		Cycle: c.opts.CycleID, RunID: c.opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleGateRepo, Origin: "checkMainRedStop", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: core.CodeMainRedStopWouldRefuse,
		Reason: "main-red stop (shadow): the lane lands, and enforce refuses it (" + r.admission + ") while " + keys + " is red on main",
		Fields: map[string]string{"keys": keys, "base": r.verdict.Base, "main": r.verdict.Main, "admission": r.admission, "fix_item": c.stop.FixItem},
	})
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
	if err := r.release(keysNotIn(v.BaseRed, v.MainRed), "main "+v.Main); err != nil {
		fmt.Fprintf(r.warn, "[ship] WARN main-red stop release: %v\n", err)
	}
}

func (r stopRelease) release(green []baseattr.Key, by string) error {
	if len(green) == 0 {
		return nil
	}
	next, err := mainstop.Update(r.opts.evolveDir(), releaseInStop(green, by, r.opts.clock()))
	if errors.Is(err, errNoStopHolds) {
		return nil
	}
	if err != nil || next.Holds() {
		return err
	}
	itemErr := r.consumeFixItem(next.FixItem)
	r.emitReleased(next, itemErr)
	return itemErr
}

func releaseInStop(green []baseattr.Key, by string, now time.Time) func(mainstop.Stop, bool) (mainstop.Stop, error) {
	return func(s mainstop.Stop, _ bool) (mainstop.Stop, error) {
		if !s.Holds() {
			return s, errNoStopHolds
		}
		return s.Released(green, by, now), nil
	}
}

func (r stopRelease) consumeFixItem(id string) error {
	_, err := inboxmover.Locate(filepath.Join(r.opts.evolveDir(), "inbox"), id)
	if errors.Is(err, inboxmover.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("fix item %s: %w", id, err)
	}
	mover := inboxmover.Options{ProjectRoot: r.opts.ProjectRoot, Stderr: r.warn, Signals: r.opts.Signals}
	if _, err := inboxmover.ReconcileSuperseded(mover, []string{id}, inboxmover.StateProcessed, inboxmover.PromoteOpts{Cycle: fmt.Sprint(r.opts.CycleID)}); err != nil {
		return fmt.Errorf("fix item %s: %w", id, err)
	}
	return nil
}

func (r stopRelease) emitReleased(stop mainstop.Stop, itemErr error) {
	e := signalcenter.Event{
		Cycle: r.opts.CycleID, RunID: r.opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleGateRepo, Origin: "stopRelease.release", Kind: signalcenter.KindGateCorrected,
		Severity: signalcenter.SeverityInfo, Code: core.CodeMainRedStopReleased,
		Reason: "main-red stop released by " + stop.ReleasedBy + ": each key is green, and the parked lanes resume in the next pool fill",
		Fields: map[string]string{"main": stop.MainSHA, "fix_item": stop.FixItem, "released_by": stop.ReleasedBy, "parked": stop.ParkedCycles(), "fix_item_error": ""},
	}
	if itemErr != nil {
		e.Severity = signalcenter.SeverityWarn
		e.Fields["fix_item_error"] = itemErr.Error()
	}
	r.opts.Signals.Emit(e)
}
