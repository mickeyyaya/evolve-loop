package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CIPackage = "origin-main-ci"

	CodeMainCIRed signalcenter.Code = "GATE_REPO_MAIN_CI_RED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleGateRepo, CodeMainCIRed, "the loop tick polled the CI of origin/main, found a completed failed check-run, and no stop held, so the main-red stop opened with one CI key for each failed job; ship refuses each landing until the poll sees a green main; fields.failed (each failed job as name=conclusion), main (the origin/main commit), fix_item, fix_item_error (empty when the item is in the inbox)")
}

type MainCIRed struct {
	Main   string
	Failed []string
}

func OpenMainCIRedStop(projectRoot string, r MainCIRed) (bool, error) {
	if len(r.Failed) == 0 {
		return false, nil
	}
	keys := make([]baseattr.Key, len(r.Failed))
	for i, job := range r.Failed {
		keys[i] = baseattr.Key{Package: CIPackage, Test: job}
	}
	return mainRedStopOpener{now: time.Now}.open(projectRoot, baseattr.Verdict{MainRed: keys, Main: r.Main})
}

func (r MainCIRed) Incident(origin string, fileErr error) signalcenter.Event {
	failed, fixItem := strings.Join(r.Failed, ", "), mainstop.FixItemID(r.Main)
	e := signalcenter.Event{
		Module: signalcenter.ModuleGateRepo, Origin: origin, Kind: signalcenter.KindLoopWarning,
		Severity: signalcenter.SeverityIncident, Code: CodeMainCIRed,
		Reason: fmt.Sprintf("the CI of origin/main is red at %s (%s), so the main-red stop is open and ship refuses each landing; next: build the inbox item %s, which leads the rank while the stop holds", r.Main, failed, fixItem),
		Fields: map[string]string{"failed": failed, "main": r.Main, "fix_item": fixItem, "fix_item_error": errText(fileErr)},
	}
	if fileErr != nil {
		e.Reason += fmt.Sprintf("; the fix item is not in the inbox (%v), and the next poll files it again", fileErr)
	}
	return e
}

func IsCIKey(k baseattr.Key) bool {
	return k.Package == CIPackage
}

func StopCIKeys(stop mainstop.Stop) []baseattr.Key {
	return slices.DeleteFunc(slices.Clone(stop.Keys), func(k baseattr.Key) bool { return !IsCIKey(k) })
}

func StopLaneKeys(stop mainstop.Stop) []baseattr.Key {
	return slices.DeleteFunc(slices.Clone(stop.Keys), IsCIKey)
}

func StopCIOnly(stop mainstop.Stop) bool {
	return stop.Holds() && len(StopCIKeys(stop)) == len(stop.Keys)
}

type KeyRunFn func(ctx context.Context, rev string, reds baseattr.Reds) ([]baseattr.Key, error)

func CIKeysStayRed(run KeyRunFn) KeyRunFn {
	return func(ctx context.Context, rev string, reds baseattr.Reds) ([]baseattr.Key, error) {
		lane := baseattr.Reds{Keys: slices.DeleteFunc(slices.Clone(reds.Keys), IsCIKey), Flags: reds.Flags}
		ci := slices.DeleteFunc(slices.Clone(reds.Keys), func(k baseattr.Key) bool { return !IsCIKey(k) })
		if len(lane.Keys) == 0 {
			return ci, nil
		}
		red, err := run(ctx, rev, lane)
		return append(red, ci...), err
	}
}
