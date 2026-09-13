package landing

// landing_test.go — the leaf's fixtures (a scripted Git keyed on the FULL
// argv, a recording Center) and the module-contract tests: the codes, the
// Null-Object Center, the happy paths silent, never the terminal kind.

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	testBranch    = "main"
	testCycleBr   = "cycle-7-branch"
	testHead      = "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"
	testOriginRef = "0f1e2d3c4b5a69788796a5b4c3d2e1f0fedcba98"
)

type scripted struct {
	stdout string
	exit   int
	err    error
}

// fakeGit scripts git by its full argv (FIFO per argv, the last entry
// sticky) and records every call with the writers it streamed to —
// "discard" (both io.Discard), "streams" (the operator streams), "capture"
// (a builder + io.Discard) — the same classification the host goldens use.
type fakeGit struct {
	scripts map[string][]scripted
	calls   []string
	stdout  strings.Builder
	stderr  strings.Builder
}

func newFakeGit() *fakeGit { return &fakeGit{scripts: map[string][]scripted{}} }

func (f *fakeGit) on(argv string, calls ...scripted) *fakeGit {
	f.scripts[argv] = append(f.scripts[argv], calls...)
	return f
}

func (f *fakeGit) set(argv string, calls ...scripted) *fakeGit {
	f.scripts[argv] = calls
	return f
}

func (f *fakeGit) streams() Streams { return Streams{Stdout: &f.stdout, Stderr: &f.stderr} }

func (f *fakeGit) classify(stdout, stderr io.Writer) string {
	switch {
	case stdout == io.Discard && stderr == io.Discard:
		return "discard"
	case stdout == io.Writer(&f.stdout) && stderr == io.Writer(&f.stderr):
		return "streams"
	case stderr == io.Discard:
		return "capture"
	}
	return "other"
}

func (f *fakeGit) git(_ context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	argv := strings.Join(args, " ")
	f.calls = append(f.calls, argv+" ["+f.classify(stdout, stderr)+"]")
	queue := f.scripts[argv]
	if len(queue) == 0 {
		return 0, nil
	}
	next := queue[0]
	if len(queue) > 1 {
		f.scripts[argv] = queue[1:]
	}
	if next.stdout != "" {
		_, _ = io.WriteString(stdout, next.stdout)
	}
	return next.exit, next.err
}

func recordingCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

// newLanding builds a Landing over the fake git with a recording Center,
// phase "ship" (the host's spelling, handed in — the leaf spells no phase),
// cycle 7 and run r7.
func newLanding(f *fakeGit) (*Landing, *[]signalcenter.Event) {
	c, got := recordingCenter()
	l := New(f.git, f.streams, WithRun("ship", 7, "r7"), WithSignals(func() *signalcenter.Center { return c }))
	return l, got
}

func logSink() (func(string), *[]string) {
	lines := &[]string{}
	return func(s string) { *lines = append(*lines, s) }, lines
}

func wantOneEvent(t *testing.T, got []signalcenter.Event, code signalcenter.Code, origin string, fields map[string]string) signalcenter.Event {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("want exactly one event, got %d: %+v", len(got), got)
	}
	e := got[0]
	if e.Module != signalcenter.ModuleShip || e.Kind != signalcenter.KindShipWarning || e.Severity != signalcenter.SeverityWarn || e.Code != code || e.Origin != origin {
		t.Errorf("event {module=%s kind=%s sev=%s code=%s origin=%s}, want {ship ship.warning WARN %s %s}", e.Module, e.Kind, e.Severity, e.Code, e.Origin, code, origin)
	}
	if e.Cycle != 7 || e.RunID != "r7" || e.Phase != "ship" {
		t.Errorf("event cycle=%d run_id=%q phase=%q, want 7 r7 ship", e.Cycle, e.RunID, e.Phase)
	}
	for k, v := range fields {
		if e.Fields[k] != v {
			t.Errorf("fields[%s]=%q, want %q (all: %v)", k, e.Fields[k], v, e.Fields)
		}
	}
	return e
}

func wantShipError(t *testing.T, err error, code shiperr.ShipErrorCode, class shiperr.ShipErrorClass, msg string) *shiperr.ShipError {
	t.Helper()
	se, ok := shiperr.AsShipError(err)
	if !ok {
		t.Fatalf("want a *shiperr.ShipError, got %T %v", err, err)
	}
	if se.Code != code || se.Class != class || se.Stage != shiperr.StageAtomicShip {
		t.Errorf("error [%s/%s @%s], want [%s/%s @atomic-ship]", se.Code, se.Class, se.Stage, code, class)
	}
	if msg != "" && se.Message != msg {
		t.Errorf("message\n got %q\nwant %q", se.Message, msg)
	}
	return se
}

// Test 28 — a recording Center over green Integrate / Push (three sites) /
// WriteBinding sees ZERO events; every fault fixture emits exactly its code
// once; no event of the terminal kind ship.error on any row.
func TestLanding_NeverEmitsShipErrorAndHappyPathsAreSilent(t *testing.T) {
	f := newFakeGit()
	f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
	l, got := newLanding(f)
	if err := l.Integrate(context.Background(), Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve"}); err != nil {
		t.Fatal(err)
	}
	for _, site := range []PushSite{SiteDirect, SiteWorktree, SitePushOnly} {
		if _, err := l.Push(context.Background(), PushRequest{Branch: testBranch, Site: site}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.WriteBinding(t.TempDir(), binding()); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatalf("the happy paths emit nothing, got %+v", *got)
	}
	faults := map[signalcenter.Code]func(){
		CodeBinaryResetFailed:  func() { resetFailureFixture(t, l, f) },
		CodePushRepairDeclined: func() { declinedFixture(t, l, f) },
		CodeHeadReadFailed:     func() { headReadFailureFixture(t, l, f) },
		CodeBindingWriteFailed: func() { bindingFailureFixture(t, l) },
	}
	for code, fault := range faults {
		*got = (*got)[:0]
		fault()
		if len(*got) != 1 || (*got)[0].Code != code {
			t.Errorf("%s: exactly one event with its code, got %+v", code, *got)
		}
		for _, e := range *got {
			if e.Kind == signalcenter.KindShipError || e.Kind.Terminal() {
				t.Errorf("%s: the leaf never emits the terminal kind: %+v", code, e)
			}
		}
	}
}

// Test 29 — a nil accessor and a nil Center are the Null Object: a failing
// reset neither panics nor emits; SignalsWired reports the truth; WithRun
// stamps phase, cycle and run_id (without it all three are empty — the leaf
// hard-codes no phase name; the host passes core.PhaseShip's spelling).
func TestWarn_IsANullObjectWithoutACenter(t *testing.T) {
	f := newFakeGit()
	f.on("checkout HEAD -- go/evolve", scripted{exit: 1})
	for name, l := range map[string]*Landing{
		"no accessor":  New(f.git, f.streams),
		"nil center":   New(f.git, f.streams, WithSignals(func() *signalcenter.Center { return nil })),
		"nil accessor": New(f.git, f.streams, WithSignals(nil)),
	} {
		if l.SignalsWired() {
			t.Errorf("%s: SignalsWired must be false", name)
		}
		if err := l.Integrate(context.Background(), Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve"}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	c, got := recordingCenter()
	l := New(f.git, f.streams, WithSignals(func() *signalcenter.Center { return c }))
	if !l.SignalsWired() {
		t.Fatal("SignalsWired must be true with a Center")
	}
	if err := l.Integrate(context.Background(), Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve"}); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || (*got)[0].Phase != "" || (*got)[0].Cycle != 0 || (*got)[0].RunID != "" {
		t.Errorf("without WithRun the event carries no phase/cycle/run_id: %+v", *got)
	}
	*got = (*got)[:0]
	stamped := New(f.git, f.streams, WithRun("phase-x", 1641, "run-x"), WithSignals(func() *signalcenter.Center { return c }))
	_ = stamped.Integrate(context.Background(), Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve"})
	if len(*got) != 1 || (*got)[0].Phase != "phase-x" || (*got)[0].Cycle != 1641 || (*got)[0].RunID != "run-x" {
		t.Errorf("WithRun stamps the phase as handed in (no literal in the leaf), the cycle and the run_id: %+v", *got)
	}
}

// Test 30 — the four codes are registered under module ship with docs, match
// the code grammar, and share no name with a projected ShipErrorCode; 30b:
// every error the leaf builds is transient or precondition at atomic-ship.
func TestCodes_RegisteredUnderModuleShipDisjointFromShipErrorCodes(t *testing.T) {
	codes := []signalcenter.Code{CodeBinaryResetFailed, CodePushRepairDeclined, CodeHeadReadFailed, CodeBindingWriteFailed}
	docs := map[signalcenter.Code]string{}
	for _, d := range signalcenter.RegisteredCodes()[signalcenter.ModuleShip] {
		docs[d.Code] = d.Doc
	}
	for _, c := range codes {
		if m, ok := signalcenter.IsRegistered(c); !ok || m != signalcenter.ModuleShip {
			t.Errorf("%s must be registered under module ship (got %q, %v)", c, m, ok)
		}
		if !c.Valid() || !c.BelongsTo(signalcenter.ModuleShip) || !strings.HasPrefix(string(c), "SHIP_LANDING_") {
			t.Errorf("%s must be a valid SHIP_LANDING_* code", c)
		}
		if docs[c] == "" {
			t.Errorf("%s must carry a doc", c)
		}
		for _, sc := range shiperr.AllCodes() {
			if shiperr.SignalCode(sc) == c {
				t.Errorf("%s collides with the projected ship error code %s", c, sc)
			}
		}
	}
	f := newFakeGit()
	f.on("merge --ff-only "+testCycleBr, scripted{exit: 128})
	f.on("push origin "+testBranch, scripted{exit: 1})
	f.on("rev-parse origin/"+testBranch, scripted{stdout: testOriginRef + "\n"})
	f.on("rev-parse HEAD", scripted{stdout: testHead + "\n"})
	f.on("merge-base --is-ancestor "+testOriginRef+" HEAD", scripted{exit: 1})
	l, _ := newLanding(f)
	var errs []error
	for _, fleet := range []bool{true, false} {
		errs = append(errs, l.Integrate(context.Background(), Integration{Branch: testBranch, CycleBranch: testCycleBr, Binary: "go/evolve", Fleet: fleet}))
	}
	for _, site := range []PushSite{SiteDirect, SiteWorktree, SitePushOnly} {
		_, err := l.Push(context.Background(), PushRequest{Branch: testBranch, Site: site, RepairAttempted: true})
		errs = append(errs, err)
	}
	_, err := l.Push(context.Background(), PushRequest{Branch: testBranch, Site: SiteDirect})
	errs = append(errs, err)
	for _, err := range errs {
		se, ok := shiperr.AsShipError(err)
		if !ok {
			t.Fatalf("every landing error is a *shiperr.ShipError: %v", err)
		}
		if (se.Class != shiperr.ShipClassTransient && se.Class != shiperr.ShipClassPrecondition) || se.Stage != shiperr.StageAtomicShip {
			t.Errorf("[%s/%s @%s]: the leaf builds only transient/precondition errors at atomic-ship", se.Code, se.Class, se.Stage)
		}
		if se.Debug[shiperr.StepKey] == "" {
			t.Errorf("%s: every leaf-built error stamps Debug[step]", se.Code)
		}
	}
}

// New takes the Git port and applies its Options in any order (WithRun and
// WithSignals are independent) — the two exported types named here for the
// API gate are the leaf's construction contract.
func TestNew_TakesTheGitPortAndAppliesOptionsInAnyOrder(t *testing.T) {
	f := newFakeGit()
	var git Git = f.git
	c, got := recordingCenter()
	signals := func() *signalcenter.Center { return c }
	for name, opts := range map[string][]Option{
		"run then signals": {WithRun("ship", 9, "r9"), WithSignals(signals)},
		"signals then run": {WithSignals(signals), WithRun("ship", 9, "r9")},
	} {
		*got = (*got)[:0]
		l := New(git, f.streams, opts...)
		bindingFailureFixture(t, l)
		if len(*got) != 1 || (*got)[0].Cycle != 9 || (*got)[0].RunID != "r9" {
			t.Errorf("%s: %+v", name, *got)
		}
	}
}
