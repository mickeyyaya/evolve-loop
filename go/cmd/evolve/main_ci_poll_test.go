package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	pollOriginMain = "a1b2c3d4e5f60718"
	redCheckRuns   = `{"total_count":2,"check_runs":[{"name":"build+test","status":"completed","conclusion":"failure"},{"name":"lint","status":"completed","conclusion":"success"}]}`
	greenCheckRuns = `{"total_count":1,"check_runs":[{"name":"build+test","status":"completed","conclusion":"success"}]}`
)

var pollStart = time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC)

type scriptedCI struct {
	red    bool
	failed []string
	err    error
	asked  []string
}

func (s *scriptedCI) check(_ context.Context, _ string, sha string) (bool, []string, error) {
	s.asked = append(s.asked, sha)
	return s.red, s.failed, s.err
}

func (s *scriptedCI) poll(root string, every time.Duration) *mainCIPoll {
	return &mainCIPoll{
		projectRoot: root,
		every:       every,
		originMain:  func(context.Context) (string, error) { return pollOriginMain, nil },
		ci:          s.check,
	}
}

func ticksAt(t *testing.T, poll *mainCIPoll, at ...time.Time) []signalcenter.Event {
	t.Helper()
	c, got, _ := recordingLoopSignals()
	ticks := make(chan time.Time)
	finish := announceLoopWith(c, io.Discard, announceDeps{
		procStartOf: func(int) (string, error) { return "start", nil },
		ticker:      func() (<-chan time.Time, func()) { return ticks, func() {} },
		mainCI:      poll,
	})
	for _, tick := range at {
		ticks <- tick
	}
	exit := 0
	finish(&exit)
	return *got
}

func eventsWithCode(got []signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	return slices.DeleteFunc(slices.Clone(got), func(e signalcenter.Event) bool { return e.Code != code })
}

func readStop(t *testing.T, root string) mainstop.Stop {
	t.Helper()
	stop, found, err := mainstop.Read(filepath.Join(root, ".evolve"))
	if err != nil || !found {
		t.Fatalf("read the stop: found=%v err=%v", found, err)
	}
	return stop
}

func stopRecorded(t *testing.T, root string) bool {
	t.Helper()
	_, found, err := mainstop.Read(filepath.Join(root, ".evolve"))
	if err != nil {
		t.Fatalf("read the stop: %v", err)
	}
	return found
}

func TestLoopTick_MainCIRedOpensMainRedStopAndEmitsIncident(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ci := &scriptedCI{red: true, failed: []string{"build+test=failure", "lint=timed_out"}}

	got := ticksAt(t, ci.poll(root, 10*time.Minute), pollStart)

	incidents := eventsWithCode(got, core.CodeMainCIRed)
	if len(incidents) != 1 {
		t.Fatalf("GATE_REPO_MAIN_CI_RED events = %+v, want one", incidents)
	}
	e := incidents[0]
	if e.Severity != signalcenter.SeverityIncident || e.Module != signalcenter.ModuleGateRepo {
		t.Errorf("event = %+v, want an INCIDENT of the gate-repo module", e)
	}
	for _, job := range []string{"build+test=failure", "lint=timed_out"} {
		if !strings.Contains(e.Fields["failed"], job) {
			t.Errorf("fields.failed = %q, want the failed job %q", e.Fields["failed"], job)
		}
	}
	if e.Fields["main"] != pollOriginMain {
		t.Errorf("fields.main = %q, want the origin/main tip %q", e.Fields["main"], pollOriginMain)
	}
	stop := readStop(t, root)
	if !stop.Holds() || stop.MainSHA != pollOriginMain || len(stop.Keys) != 2 || stop.Keys[0].Package != core.CIPackage {
		t.Errorf("stop = %+v, want a held stop with one CI key for each failed job", stop)
	}
	if filed, err := inboxfile.FiledIDs(filepath.Join(root, ".evolve", "inbox")); err != nil || !filed[stop.FixItem] {
		t.Errorf("fix item %s filed = %v err = %v, want it in the inbox", stop.FixItem, filed[stop.FixItem], err)
	}
}

func TestLoopTick_MainCIRedPersistingEmitsOneIncidentAndFilesOneFixItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ci := &scriptedCI{red: true, failed: []string{"build+test=failure"}}

	got := ticksAt(t, ci.poll(root, time.Minute), pollStart, pollStart.Add(time.Minute), pollStart.Add(2*time.Minute))

	if len(ci.asked) != 3 {
		t.Fatalf("CI asked %d times, want a poll on each tick of a one-minute interval", len(ci.asked))
	}
	if n := len(eventsWithCode(got, core.CodeMainCIRed)); n != 1 {
		t.Errorf("%d incidents for one red stretch, want 1", n)
	}
}

func TestLoopTick_MainCIPollHonoursThePolicyInterval(t *testing.T) {
	t.Parallel()
	ci := &scriptedCI{}

	ticksAt(t, ci.poll(t.TempDir(), 10*time.Minute),
		pollStart, pollStart.Add(time.Minute), pollStart.Add(9*time.Minute), pollStart.Add(11*time.Minute))

	if len(ci.asked) != 2 {
		t.Errorf("CI asked %d times, want the first tick and the first tick after 10 minutes", len(ci.asked))
	}
}

func TestLoopTick_MainCIUnknownOpensNoStopAndEmitsNoIncident(t *testing.T) {
	t.Parallel()
	for name, ci := range map[string]*scriptedCI{
		"the CI query fails":          {err: errors.New("gh api check-runs: exit status 1")},
		"no completed check-run":      {err: errors.New("no completed check-runs visible for a1b2c3d4e5f6")},
		"a red verdict names no jobs": {red: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()

			got := ticksAt(t, ci.poll(root, time.Minute), pollStart)

			if n := len(eventsWithCode(got, core.CodeMainCIRed)); n != 0 || stopRecorded(t, root) {
				t.Errorf("incidents = %d stop recorded = %v, want neither when CI is unknown", n, stopRecorded(t, root))
			}
		})
	}
}

func TestLoopTick_MainCIGreenClosesTheStopAndConsumesTheFixItem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := core.OpenMainCIRedStop(root, core.MainCIRed{Main: "01d01d01d01d01d0", Failed: []string{"build+test=failure"}}); err != nil {
		t.Fatal(err)
	}
	held := readStop(t, root)
	ci := &scriptedCI{}

	got := ticksAt(t, ci.poll(root, time.Minute), pollStart)

	stop := readStop(t, root)
	if stop.Holds() || stop.ReleasedAt == nil || !strings.Contains(stop.ReleasedBy, pollOriginMain) {
		t.Errorf("stop = %+v, want it released by the green origin/main tip %s", stop, pollOriginMain)
	}
	if released := eventsWithCode(got, selfheal.CodeMainRedStopReleased); len(released) != 1 || released[0].Fields["fix_item"] != held.FixItem {
		t.Errorf("release events = %+v, want one that names the fix item %s", released, held.FixItem)
	}
	if n := len(eventsWithCode(got, core.CodeMainCIRed)); n != 0 {
		t.Errorf("%d CI-red incidents on a green main", n)
	}
}

func TestLoopTick_MainCIGreenKeepsTheStopOnALaneReproducedKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	laneKey := baseattr.Key{Package: "example.com/m/pkg/a", Test: "TestOne"}
	if _, err := core.OpenMainRedStop(root, baseattr.Verdict{MainRed: []baseattr.Key{laneKey}, Main: "9a2bbe0e1c0ffee"}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.OpenMainCIRedStop(root, core.MainCIRed{Main: "9a2bbe0e1c0ffee", Failed: []string{"build+test=failure"}}); err != nil {
		t.Fatal(err)
	}
	ci := &scriptedCI{}

	got := ticksAt(t, ci.poll(root, time.Minute), pollStart)

	stop := readStop(t, root)
	if !stop.Holds() || !slices.Equal(stop.Keys, []baseattr.Key{laneKey}) {
		t.Errorf("keys = %v holds = %v, want only the CI key closed and the lane key kept", stop.Keys, stop.Holds())
	}
	if n := len(eventsWithCode(got, selfheal.CodeMainRedStopReleased)); n != 0 {
		t.Errorf("%d release events while a lane key still holds the stop", n)
	}
}

func TestLoopTick_MainCIPollWiredToOriginMainAndGH(t *testing.T) {
	origin, runtime := syncFixture(t)
	originAdvance(t, origin)
	installCheckRunsGH(t, redCheckRuns, false)
	wantTip := gitrun(t, origin, "rev-parse", "main")

	got := ticksAt(t, newMainCIPoll(runtime, time.Minute), pollStart)

	if n := len(eventsWithCode(got, core.CodeMainCIRed)); n != 1 {
		t.Fatalf("incidents = %d, want 1 from a red check-run of the real gh path", n)
	}
	stop := readStop(t, runtime)
	if stop.MainSHA != wantTip || len(stop.Keys) != 1 || stop.Keys[0].Test != "build+test=failure" {
		t.Errorf("stop = %+v, want the fetched origin/main tip %s and the failed job build+test=failure", stop, wantTip)
	}

	installCheckRunsGH(t, greenCheckRuns, false)
	ticksAt(t, newMainCIPoll(runtime, time.Minute), pollStart.Add(time.Hour))
	if readStop(t, runtime).Holds() {
		t.Error("a green check-run left the stop held")
	}
}

func TestNewLoopMainCIPoll_LoadsPolicyAndFallsBackWhenMissingOrInvalid(t *testing.T) {
	t.Parallel()
	if got := newLoopMainCIPoll(t.TempDir()).every; got != 10*time.Minute {
		t.Errorf("missing policy interval = %v, want 10m", got)
	}
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := newLoopMainCIPoll(root).every; got != 10*time.Minute {
		t.Errorf("invalid policy interval = %v, want 10m", got)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"gates":{"main_ci_poll_minutes":3}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := newLoopMainCIPoll(root).every; got != 3*time.Minute {
		t.Errorf("policy interval = %v, want 3m", got)
	}
}

func TestOriginMainTip_ErrorsWhenOriginMissingOrHasNoMainRef(t *testing.T) {
	t.Parallel()
	if _, err := originMainTip(t.TempDir())(context.Background()); !errors.Is(err, errOriginMainLSRemote) {
		t.Errorf("non-git err = %v, want %v", err, errOriginMainLSRemote)
	}
	bare := gittest.Bare(t)
	repo := gittest.Fixture(t)
	repo.Git("remote", "add", "origin", bare.Dir)
	if _, err := originMainTip(repo.Dir)(context.Background()); !errors.Is(err, errNoOriginMainRef) {
		t.Errorf("empty origin err = %v, want %v", err, errNoOriginMainRef)
	}
}

func TestMainCIPollTick_OriginMainErrorLogsWarningAndOpensNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var warn bytes.Buffer
	ciCalled := false
	poll := &mainCIPoll{
		projectRoot: root,
		every:       time.Minute,
		originMain:  func(context.Context) (string, error) { return "", errors.New("origin unreachable") },
		ci: func(context.Context, string, string) (bool, []string, error) {
			ciCalled = true
			return false, nil, nil
		},
	}
	c, got, _ := recordingLoopSignals()

	poll.tick(pollStart, c, &warn)

	if ciCalled || stopRecorded(t, root) || len(eventsWithCode(*got, core.CodeMainCIRed)) != 0 {
		t.Errorf("ciCalled=%v stopRecorded=%v incidents=%d, want none on originMain error", ciCalled, stopRecorded(t, root), len(eventsWithCode(*got, core.CodeMainCIRed)))
	}
	if !strings.Contains(warn.String(), "origin unreachable") {
		t.Errorf("warn = %q, want origin unreachable warning", warn.String())
	}
}

func TestMainCIPollOpen_LogsWarningWhenStopCannotBeWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	c, got, _ := recordingLoopSignals()

	newMainCIPoll(root, time.Minute).open(core.MainCIRed{Main: pollOriginMain, Failed: []string{"build+test=failure"}}, c, &warn)

	if len(eventsWithCode(*got, core.CodeMainCIRed)) != 0 || !strings.Contains(warn.String(), "main CI poll:") {
		t.Errorf("incidents=%d warn=%q, want warning logged and no incident when stop cannot open", len(eventsWithCode(*got, core.CodeMainCIRed)), warn.String())
	}
}

func TestMainCIPollCloseOnGreen_LogsWarningWhenConsumeFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := core.OpenMainCIRedStop(root, core.MainCIRed{Main: pollOriginMain, Failed: []string{"build+test=failure"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "inbox", "processed"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	c, _, _ := recordingLoopSignals()

	newMainCIPoll(root, time.Minute).closeOnGreen(pollOriginMain, pollStart, c, &warn)

	if !strings.Contains(warn.String(), "release the stop:") {
		t.Errorf("warn = %q, want release the stop warning", warn.String())
	}
}
