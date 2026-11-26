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

const noneParsed = "(none parsed)"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeMainRedCIUnprovable, "the required CI run of the main tip was red at a key that no prover of the stop can run or that is green at main, so ship opened no main-red stop for that key; fields.keys (the keys left out, or (none parsed)), main (the main tip), run (the URL of the run), detail (the hint of the CI row), error (the cause when the run at main failed); next: run the gate on the CI artifact, because the train refuses on the red main until it is green")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeMainRedCIOpened, "the required CI run of the main tip was red and no main-red stop held, so ship opened the stop with the failed tests that are red at main; fields.keys (package.Test of each test), main (the main tip), run (the URL of the run), open_error (empty when the stop and its fix item are written)")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeMainRedCIUnread, "ship could not read the required CI run of the main tip, so the CI source judged nothing and the lane went on; fields.main (the main tip), error (the cause); next: repair the gh login or the network, because a red main opens no stop while the source is unreadable")
}

type mainCI struct {
	opts *Options
	warn io.Writer
	attr baseattr.Attributor
}

type unproved struct {
	name, detail string
}

func provable(f ciwatch.FailingTest) bool {
	return f.Package != "" && f.Test != ""
}

func (m mainCI) sync(ctx context.Context, stop mainstop.Stop, holds bool) (mainstop.Stop, bool) {
	sha, err := m.attr.Resolve(ctx, m.attr.MainRef)
	if err != nil {
		fmt.Fprintf(m.warn, "[ship] WARN main-red stop: %s could not be resolved (%v), so the CI source was not read\n", m.attr.MainRef, err)
		return stop, holds
	}
	if holds {
		if stop.MainSHA == sha {
			m.claim(sha)
		}
		return stop, holds
	}
	if mainstop.Judged(m.opts.evolveDir(), sha) {
		return stop, holds
	}
	run, err := m.opts.mainCI(ctx, sha)
	if err != nil {
		m.unread(sha, err)
		return stop, holds
	}
	if run.Status != ciwatch.StatusCompleted || run.Conclusion == ciwatch.ConclusionSuccess || !m.claim(sha) {
		return stop, holds
	}
	return m.judge(ctx, run, sha)
}

func (m mainCI) claim(sha string) bool {
	claimed, err := mainstop.ClaimJudged(m.opts.evolveDir(), sha)
	if err != nil {
		fmt.Fprintf(m.warn, "[ship] WARN main-red stop: the judged record of %s could not be written (%v), so the CI source judged nothing\n", sha, err)
	}
	return claimed
}

func (m mainCI) judge(ctx context.Context, run ciwatch.RunStatus, sha string) (mainstop.Stop, bool) {
	red, runErr := m.reproduced(ctx, sha, provableKeys(run.FailingTests))
	held, left := partitionRows(run.FailingTests, red)
	if len(held) == 0 && len(left) == 0 {
		left = []unproved{{name: noneParsed}}
	}
	if len(left) > 0 {
		m.unprovable(run, sha, left, runErr)
	}
	if len(held) == 0 {
		return mainstop.Stop{}, false
	}
	if m.open(run, sha, held) != nil {
		m.release(sha)
	}
	return selfheal.ReadStop(m.opts.evolveDir(), m.warn)
}

func (m mainCI) reproduced(ctx context.Context, sha string, keys []baseattr.Key) ([]baseattr.Key, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	return m.attr.RunAt(ctx, sha, mainstop.ProofReds(keys))
}

func provableKeys(rows []ciwatch.FailingTest) []baseattr.Key {
	var keys []baseattr.Key
	for _, f := range rows {
		if k := keyOf(f); provable(f) && !slices.Contains(keys, k) {
			keys = append(keys, k)
		}
	}
	return keys
}

func keyOf(f ciwatch.FailingTest) baseattr.Key {
	return baseattr.Key{Package: f.Package, Test: f.Test}
}

func partitionRows(rows []ciwatch.FailingTest, red []baseattr.Key) (held []baseattr.Key, left []unproved) {
	for _, f := range rows {
		name := rowName(f)
		switch {
		case name == "":
		case provable(f) && slices.Contains(red, keyOf(f)):
			if !slices.Contains(held, keyOf(f)) {
				held = append(held, keyOf(f))
			}
		case !slices.ContainsFunc(left, func(u unproved) bool { return u.name == name }):
			left = append(left, unproved{name: name, detail: f.Detail})
		}
	}
	return held, left
}

func rowName(f ciwatch.FailingTest) string {
	if provable(f) {
		return f.Package + "." + f.Test
	}
	return cmp.Or(f.Step, f.Test, f.Package, strings.Join(f.Jobs, ", "))
}

func (m mainCI) unprovable(run ciwatch.RunStatus, sha string, left []unproved, runErr error) {
	var names, details []string
	for _, u := range left {
		names = append(names, u.name)
		if u.detail != "" {
			details = append(details, u.detail)
		}
	}
	keys := strings.Join(names, ", ")
	fields := map[string]string{"keys": keys, "main": sha, "run": run.RunURL}
	if len(details) > 0 {
		fields["detail"] = strings.Join(details, "; ")
	}
	if runErr != nil {
		fields["error"] = runErr.Error()
	}
	m.emit(CodeMainRedCIUnprovable, "mainCI.unprovable", fmt.Sprintf("the required CI run of main %s is red at %s, which no prover of the stop can run or which is green at main, so no main-red stop holds it; next: run the gate on the CI artifact, because the train refuses on the red main until it is green", sha, keys), fields)
}

func (m mainCI) release(sha string) {
	if err := mainstop.ReleaseJudged(m.opts.evolveDir(), sha); err != nil {
		fmt.Fprintf(m.warn, "[ship] WARN main-red stop: the judged record of %s could not be released (%v), so a later lane will not judge it again\n", sha, err)
	}
}

func (m mainCI) open(run ciwatch.RunStatus, sha string, held []baseattr.Key) error {
	_, err := core.OpenMainRedStop(m.opts.ProjectRoot, baseattr.Verdict{BaseRed: held, MainRed: held, Main: sha})
	keys := baseattr.Names(held)
	m.emit(CodeMainRedCIOpened, "mainCI.open", fmt.Sprintf("the required CI run of main %s is red (%s), so the main-red stop opened; next: build the fix item %s", sha, keys, mainstop.FixItemID(sha)),
		map[string]string{"keys": keys, "main": sha, "run": run.RunURL, "open_error": errStr(err)})
	return err
}

func (m mainCI) unread(sha string, err error) {
	fmt.Fprintf(m.warn, "[ship] WARN main-red stop: the required CI run of main could not be read (%v), so the CI source judged nothing\n", err)
	m.emit(CodeMainRedCIUnread, "mainCI.unread", fmt.Sprintf("the required CI run of main %s could not be read (%v), so the lane went on", sha, err),
		map[string]string{"main": sha, "error": err.Error()})
}

func (m mainCI) emit(code signalcenter.Code, origin, reason string, fields map[string]string) {
	m.opts.Signals.Emit(signalcenter.Event{
		Cycle: m.opts.CycleID, RunID: m.opts.RunID, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: origin, Kind: signalcenter.KindShipWarning, Severity: signalcenter.SeverityWarn, Code: code,
		Reason: reason, Fields: fields,
	})
}
