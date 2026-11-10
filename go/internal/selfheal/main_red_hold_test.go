package selfheal

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const batchTip = "feedfacefeedface"

type keyRun struct {
	red   []baseattr.Key
	err   error
	calls []string
}

func (r *keyRun) run(_ context.Context, rev string, reds baseattr.Reds) ([]baseattr.Key, error) {
	r.calls = append(r.calls, rev+" "+baseattr.Names(reds.Keys)+" "+strings.Join(reds.Flags, " "))
	return r.red, r.err
}

func (f *boundFixture) holder(enforced bool, run *keyRun, warn *strings.Builder) *StopHold {
	return &StopHold{
		EvolveDir: f.evolveDir, Enforced: enforced, Warn: warn, Run: KeyRun(run.run),
		Event: signalcenter.Event{Origin: "integration batch"},
		Emit:  func(e signalcenter.Event) { f.events = append(f.events, e) },
	}
}

func TestStopHold_AdmitsOnlyABatchThatTurnsAKeyOfTheStopGreen(t *testing.T) {
	t.Parallel()
	bothKeys := []baseattr.Key{boundKey, otherKey}
	cases := map[string]struct {
		red      []baseattr.Key
		runErr   error
		enforced bool
		green    []baseattr.Key
		held     bool
		events   int
		warning  string
	}{
		"enforce, each key green":               {enforced: true, green: bothKeys},
		"enforce, one key green":                {enforced: true, red: []baseattr.Key{otherKey}, green: []baseattr.Key{boundKey}},
		"shadow, one key green":                 {red: []baseattr.Key{boundKey}, green: []baseattr.Key{otherKey}},
		"enforce, each key red":                 {enforced: true, red: bothKeys, held: true},
		"shadow, each key red":                  {red: bothKeys, events: 1},
		"enforce, the key run failed":           {enforced: true, runErr: errors.New("go test did not start"), held: true, warning: "the keys of the stop could not be run on " + batchTip + " (go test did not start)"},
		"shadow, the key run failed":            {runErr: errors.New("go test did not start"), events: 1, warning: "so the batch is not a proven fix"},
		"enforce, a failed run with no red key": {enforced: true, runErr: errors.New("timeout"), red: nil, held: true, warning: "timeout"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newBoundFixture(t)
			f.hold(withSecondKey)
			run := &keyRun{red: c.red, err: c.runErr}
			var warn strings.Builder

			green, err := f.holder(c.enforced, run, &warn).Admit(context.Background(), boundMain, batchTip)

			if !slices.Equal(green, c.green) || errors.Is(err, ErrMainRedHold) != c.held || (err != nil) != c.held {
				t.Errorf("green = %v, err = %v, want green %v and held %v", green, err, c.green, c.held)
			}
			if !slices.Equal(run.calls, []string{batchTip + " " + baseattr.Names(bothKeys) + " -tags integration"}) {
				t.Errorf("key runs = %v, want one run of each key of the stop on the batch tip with the tags of the floor", run.calls)
			}
			if len(f.events) != c.events {
				t.Errorf("%d events, want %d: %+v", len(f.events), c.events, f.events)
			}
			if c.held && !strings.Contains(err.Error(), "(not-a-fix)") {
				t.Errorf("err = %q, want the admission not-a-fix for a batch on the head that was read", err)
			}
			if c.events == 1 && len(f.events) == 1 && (f.events[0].Fields["admission"] != "not-a-fix" || f.events[0].Fields["base"] != boundMain) {
				t.Errorf("fields = %v, want the admission not-a-fix on the head that was read", f.events[0].Fields)
			}
			if (c.warning == "") != (warn.Len() == 0) || !strings.Contains(warn.String(), c.warning) {
				t.Errorf("warning = %q, want %q", warn.String(), c.warning)
			}
		})
	}
}

func TestStopHold_TheRefusalNamesTheLandingTheKeysAndTheRemedy(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(withSecondKey)
	var warn strings.Builder
	h := f.holder(true, &keyRun{red: []baseattr.Key{boundKey, otherKey}}, &warn)

	_, admitErr := h.Admit(context.Background(), boundMain, batchTip)
	redErr := h.Red("batch pull request 12")

	for landing, err := range map[string]error{"batch " + batchTip: admitErr, "batch pull request 12": redErr} {
		if !errors.Is(err, ErrMainRedHold) {
			t.Fatalf("%s: err = %v, want the hold error", landing, err)
		}
		for _, part := range []string{landing, baseattr.Names([]baseattr.Key{boundKey, otherKey}), boundMain, boundFixItem, "one batch with each fix and no other change", "ship the fix"} {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: err = %q, want it to name %q", landing, err, part)
			}
		}
	}
	if len(f.events) != 0 {
		t.Errorf("enforce emitted %d would-refuse events", len(f.events))
	}
}

func TestStopHold_ShadowRecordsOneWouldRefuseForEachHolder(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	var warn strings.Builder
	h := f.holder(false, &keyRun{red: []baseattr.Key{boundKey}}, &warn)

	if err := h.Red("batch pull request 12"); err != nil {
		t.Fatal(err)
	}
	if err := h.Red("batch pull request 13"); err != nil {
		t.Fatal(err)
	}
	if green, err := h.Admit(context.Background(), boundMain, batchTip); err != nil || len(green) != 0 {
		t.Fatalf("Admit = %v, %v, want a landing with no green key", green, err)
	}

	if len(f.events) != 1 {
		t.Fatalf("%d events, want one for the holder: %+v", len(f.events), f.events)
	}
	e := f.events[0]
	if e.Origin != "integration batch" || e.Module != signalcenter.ModuleGateRepo || e.Kind != signalcenter.KindShipWarning || e.Severity != signalcenter.SeverityWarn || e.Code != core.CodeMainRedStopWouldRefuse {
		t.Errorf("event = %+v, want a warn would-refuse of gate.repo with the origin of the caller", e)
	}
	want := map[string]string{"keys": baseattr.Names([]baseattr.Key{boundKey}), "base": boundMain, "main": boundMain, "admission": "not-a-fix", "fix_item": boundFixItem, "landing": "batch pull request 12"}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
		}
	}
	if !strings.Contains(e.Reason, "batch pull request 12") || !strings.Contains(e.Reason, "not-a-fix") {
		t.Errorf("reason = %q, want the landing and the admission", e.Reason)
	}
}

func TestStopHold_ABatchOnAMainThatIsNotTheTipOfTheProofIsAskedToRebase(t *testing.T) {
	t.Parallel()
	f := newBoundFixture(t)
	f.hold(unchanged)
	var warn strings.Builder

	green, err := f.holder(false, &keyRun{}, &warn).Admit(context.Background(), "", batchTip)

	if err != nil || len(green) != 0 || len(f.events) != 1 || f.events[0].Fields["admission"] != "rebase" || f.events[0].Fields["main"] != "" || f.events[0].Fields["base"] != "" {
		t.Errorf("green = %v, err = %v, events = %+v, want one would-refuse with the rebase admission for an unread main head", green, err, f.events)
	}
}

func TestStopHold_NoStopThatHoldsHoldsNoLanding(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		prepare func(f *boundFixture)
		warning string
	}{
		"no record": {func(*boundFixture) {}, ""},
		"a released stop": {func(f *boundFixture) {
			f.hold(func(s mainstop.Stop) mainstop.Stop { return s.Released(s.Keys, "main abc", t0) })
		}, ""},
		"a record that does not parse": {func(f *boundFixture) {
			if err := os.WriteFile(mainstop.Path(f.evolveDir), []byte("{not json"), 0o644); err != nil {
				f.t.Fatal(err)
			}
		}, "the record could not be read"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newBoundFixture(t)
			c.prepare(f)
			run := &keyRun{red: []baseattr.Key{boundKey}}
			var warn strings.Builder
			h := f.holder(true, run, &warn)

			redErr := h.Red("batch pull request 12")
			green, admitErr := h.Admit(context.Background(), boundMain, batchTip)

			if redErr != nil || admitErr != nil || len(green) != 0 || len(run.calls) != 0 || len(f.events) != 0 {
				t.Errorf("red = %v, admit = %v, %v, key runs = %v, events = %+v, want a free landing and no key run", redErr, green, admitErr, run.calls, f.events)
			}
			if (c.warning == "") != (warn.Len() == 0) || !strings.Contains(warn.String(), c.warning) {
				t.Errorf("warning = %q, want %q", warn.String(), c.warning)
			}
		})
	}
}

func TestStopHold_TheHoldNamesTheCommitOfTheStopWhenMainMovedOn(t *testing.T) {
	t.Parallel()
	const movedHead = "0badc0de0badc0de"
	f := newBoundFixture(t)
	f.hold(unchanged)
	var warn strings.Builder
	red := &keyRun{red: []baseattr.Key{boundKey}}

	if _, err := f.holder(false, red, &warn).Admit(context.Background(), movedHead, batchTip); err != nil {
		t.Fatal(err)
	}
	_, err := f.holder(true, red, &warn).Admit(context.Background(), movedHead, batchTip)

	if len(f.events) != 1 || f.events[0].Fields["main"] != boundMain || f.events[0].Fields["base"] != movedHead || f.events[0].Fields["admission"] != "not-a-fix" {
		t.Errorf("events = %+v, want the commit of the stop as main and the head that was read as base", f.events)
	}
	if !errors.Is(err, ErrMainRedHold) || !strings.Contains(err.Error(), "red on main "+boundMain) {
		t.Errorf("err = %v, want the hold to name the commit of the stop", err)
	}
}

func TestStopRefusal_ARebaseNamesTheHeadThatTheCallerRead(t *testing.T) {
	t.Parallel()
	stop := mainstop.Stop{MainSHA: boundMain, Keys: []baseattr.Key{boundKey}, FixItem: boundFixItem}
	base := signalcenter.Event{Cycle: 7, RunID: "run-7", Origin: "checkMainRedStop"}

	e := StopRefusal{Landing: "the lane", Head: "0badc0de0badc0de", Base: "cafe", Admission: mainstop.RefuseRebase}.WouldRefuse(base, stop)

	want := map[string]string{"main": "0badc0de0badc0de", "base": "cafe", "admission": "rebase", "landing": "the lane", "fix_item": boundFixItem, "keys": baseattr.Names(stop.Keys)}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q", k, e.Fields[k], v)
		}
	}
	if e.Cycle != 7 || e.RunID != "run-7" || e.Origin != "checkMainRedStop" || !strings.HasPrefix(e.Reason, "main-red stop (shadow): the lane lands, and enforce refuses it (rebase)") {
		t.Errorf("event = %+v, want the identity of the caller and the landing in the reason", e)
	}
}
