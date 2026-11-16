package ship

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeMainRedCIOpened     signalcenter.Code = "SHIP_MAIN_RED_CI_OPENED"
	CodeMainRedCIUnread     signalcenter.Code = "SHIP_MAIN_RED_CI_UNREAD"
	CodeMainRedCIUnprovable signalcenter.Code = "SHIP_MAIN_RED_CI_UNPROVABLE"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeMainRedCIUnprovable, "the required CI run of the main tip was red at a step that no runner can prove in a lane (ciparity.GateSteps has no row for it), so ship left that key out of the main-red stop; fields.keys (the left-out keys), main (the main tip), run (the URL of the run); next: add a GateSteps row for the step, because a stop on a key that no lane can prove would park every lane")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeMainRedCIOpened, "the required CI run of the main tip was red and no main-red stop held, so ship opened the stop with the failed tests and steps as its keys; fields.keys (package.Test of a test, step:<name> of a step), main (the main tip), run (the URL of the run), open_error (empty when the stop and its fix item are written)")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeMainRedCIUnread, "ship could not read the required CI run of the main tip, so the CI source judged nothing and the lane went on; fields.main (the main tip), error (the cause); next: repair the gh login or the network, because a red CI step on main opens no stop while the source is unreadable")
}

type StepRunner func(ctx context.Context, tree string, steps []baseattr.Key) (red []baseattr.Key, err error)

type mainCI struct {
	opts *Options
	warn io.Writer
	attr baseattr.Attributor
}

func (m mainCI) sync(ctx context.Context, stop mainstop.Stop, holds bool) (mainstop.Stop, bool) {
	sha, err := m.attr.Resolve(ctx, m.attr.MainRef)
	if err != nil {
		fmt.Fprintf(m.warn, "[ship] WARN main-red stop: %s could not be resolved (%v), so the CI source was not read\n", m.attr.MainRef, err)
		return stop, holds
	}
	run, err := m.opts.mainCI(ctx, sha)
	if err != nil {
		m.unread(sha, err)
		return stop, holds
	}
	if run.Status != ciwatch.StatusCompleted {
		return stop, holds
	}
	if run.Conclusion == ciwatch.ConclusionSuccess {
		return m.greened(stop, holds, sha)
	}
	if holds {
		return stop, holds
	}
	return m.opened(run, sha)
}

func (m mainCI) greened(stop mainstop.Stop, holds bool, sha string) (mainstop.Stop, bool) {
	steps := stop.StepKeys()
	if !holds || len(steps) == 0 {
		return stop, holds
	}
	stopRelease{opts: m.opts, warn: m.warn}.greenOnMain(baseattr.Verdict{BaseRed: steps, Main: sha})
	return selfheal.ReadStop(m.opts.evolveDir(), m.warn)
}

func (m mainCI) opened(run ciwatch.RunStatus, sha string) (mainstop.Stop, bool) {
	keys, unprovable := splitProvable(ciKeys(run))
	if len(unprovable) > 0 {
		m.opts.Signals.Emit(signalcenter.Event{
			Cycle: m.opts.CycleID, RunID: m.opts.RunID, Phase: phaseName,
			Module: signalcenter.ModuleShip, Origin: "mainCI.opened", Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeMainRedCIUnprovable,
			Reason: fmt.Sprintf("the required CI run of main %s is red at %s, which no runner can prove in a lane, so the main-red stop does not hold it", sha, baseattr.Names(unprovable)),
			Fields: map[string]string{"keys": baseattr.Names(unprovable), "main": sha, "run": run.RunURL},
		})
	}
	if len(keys) == 0 {
		return mainstop.Stop{}, false
	}
	_, err := core.OpenMainRedStop(m.opts.ProjectRoot, baseattr.Verdict{BaseRed: keys, MainRed: keys, Main: sha})
	m.opts.Signals.Emit(signalcenter.Event{
		Cycle: m.opts.CycleID, RunID: m.opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: "mainCI.opened", Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeMainRedCIOpened,
		Reason: fmt.Sprintf("the required CI run of main %s is red (%s), so the main-red stop opened; next: build the fix item %s", sha, baseattr.Names(keys), mainstop.FixItemID(sha)),
		Fields: map[string]string{"keys": baseattr.Names(keys), "main": sha, "run": run.RunURL, "open_error": errStr(err)},
	})
	return selfheal.ReadStop(m.opts.evolveDir(), m.warn)
}

func (m mainCI) unread(sha string, err error) {
	fmt.Fprintf(m.warn, "[ship] WARN main-red stop: the required CI run of main could not be read (%v), so the CI source judged nothing\n", err)
	m.opts.Signals.Emit(signalcenter.Event{
		Cycle: m.opts.CycleID, RunID: m.opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: "mainCI.unread", Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: CodeMainRedCIUnread,
		Reason: fmt.Sprintf("the required CI run of main %s could not be read (%v), so the lane went on", sha, err),
		Fields: map[string]string{"main": sha, "error": err.Error()},
	})
}

func ciKeys(run ciwatch.RunStatus) []baseattr.Key {
	var keys []baseattr.Key
	for _, f := range run.FailingTests {
		k := ciKey(f)
		if isKey(k) && !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	return keys
}

func isKey(k baseattr.Key) bool {
	return k != (baseattr.Key{})
}

func splitProvable(keys []baseattr.Key) (proven, unprovable []baseattr.Key) {
	for _, k := range keys {
		if provable(k) {
			proven = append(proven, k)
		} else {
			unprovable = append(unprovable, k)
		}
	}
	return proven, unprovable
}

func ciKey(f ciwatch.FailingTest) baseattr.Key {
	if f.Test != "" {
		return baseattr.Key{Package: f.Package, Test: f.Test}
	}
	return baseattr.StepKey(cmp.Or(f.Step, strings.Join(f.Jobs, ", ")))
}
