package rootwire

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordsOf(t *testing.T, evolveDir, name string) []channel.Record {
	t.Helper()
	var out []channel.Record
	for _, r := range rawRecordsOf(t, evolveDir, name) {
		if r.Signal != nil {
			out = append(out, r)
		}
	}
	return out
}

func rawRecordsOf(t *testing.T, evolveDir, name string) []channel.Record {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(evolveDir), name, channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute})
	if err != nil {
		t.Fatalf("channel.New(%s) = %v", name, err)
	}
	batch, err := log.Read(0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("Read(%s) = %v", name, err)
	}
	return batch.Records
}

func kindsIn(t *testing.T, evolveDir, name string) []string {
	t.Helper()
	var kinds []string
	for _, r := range recordsOf(t, evolveDir, name) {
		kinds = append(kinds, string(r.Signal.Kind))
	}
	return kinds
}

func info(module signalcenter.Module, kind signalcenter.Kind, cycle int) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: module, Origin: "Test.emit", Kind: kind, Severity: signalcenter.SeverityInfo, Reason: string(kind)}
}

func warn() signalcenter.Event {
	return signalcenter.Event{
		Module: signalcenter.ModuleSignalCenter, Origin: "Test.emit", Kind: signalcenter.KindSinkDropped,
		Severity: signalcenter.SeverityWarn, Code: signalcenter.CodeSinkDropped, Reason: "a warning",
	}
}

func newCenter() *signalcenter.Center { return signalcenter.New(signalcenter.WithPID(4242)) }

func TestAttach_RoutesTheOrchestratorKindsToTheirChannels(t *testing.T) {
	evolveDir := t.TempDir()
	c := newCenter()
	closeRoot := Attach(c, "loop", evolveDir, os.Stderr)

	c.Emit(info(signalcenter.ModuleLoop, signalcenter.KindLoopStarted, 0))
	c.Emit(info(signalcenter.ModuleOrchestrator, signalcenter.KindPhaseDispatched, 7))
	c.Emit(info(signalcenter.ModuleOrchestrator, signalcenter.KindPhaseOutcome, 7))
	c.Emit(info(signalcenter.ModuleShip, signalcenter.KindShipLanded, 7))
	c.Emit(info(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 7))
	c.Emit(info(signalcenter.ModuleLoop, signalcenter.KindLoopWave, 0))
	c.Emit(warn())
	c.Emit(info(signalcenter.ModuleLoop, signalcenter.KindLoopExit, 0))
	closeRoot()

	want := map[string][]string{
		"cycle":  {"phase.dispatched", "phase.outcome", "ship.landed", "cycle.sealed"},
		"ship":   {"ship.landed"},
		"loop":   {"loop.started", "loop.wave", "loop.exit"},
		"errors": {"signalcenter.sink_dropped"},
		"inbox":  nil,
		"ledger": nil,
		"signals": {"loop.started", "phase.dispatched", "phase.outcome", "ship.landed", "cycle.sealed", "loop.wave",
			"signalcenter.sink_dropped", "loop.exit"},
	}
	for name, kinds := range want {
		if got := kindsIn(t, evolveDir, name); !reflect.DeepEqual(got, kinds) {
			t.Errorf("channel %s holds %v, want %v", name, got, kinds)
		}
	}
}

func TestAttach_AFailedCycleSealedReachesTheErrorsChannelToo(t *testing.T) {
	evolveDir := t.TempDir()
	c := newCenter()
	closeRoot := Attach(c, "cycle", evolveDir, os.Stderr)
	sealed := signalcenter.Event{
		Cycle: 9, Module: signalcenter.ModuleSignalCenter, Origin: "Test.emit", Kind: signalcenter.KindSinkDropped,
		Severity: signalcenter.SeverityIncident, Code: signalcenter.CodeSinkDropped, Reason: "an incident",
	}

	c.Emit(sealed)
	closeRoot()

	if got := kindsIn(t, evolveDir, "errors"); !reflect.DeepEqual(got, []string{"signalcenter.sink_dropped"}) {
		t.Errorf("errors holds %v, want the INCIDENT: severity>=WARN includes INCIDENT", got)
	}
}

func TestAttach_StampsTheRoleAndTheDispatchIDOfTheProcess(t *testing.T) {
	t.Setenv(ipcenv.DispatchIDKey, "01R/7/build/p1n1")
	evolveDir := t.TempDir()
	c := newCenter()
	closeRoot := Attach(c, "ship", evolveDir, os.Stderr)

	c.Emit(info(signalcenter.ModuleShip, signalcenter.KindShipLanded, 7))
	closeRoot()

	records := recordsOf(t, evolveDir, "ship")
	if len(records) != 1 || records[0].Source != "ship" || records[0].Dispatch != "01R/7/build/p1n1" {
		t.Errorf("records = %+v, want one record from role ship stamped with the dispatch id of the environment", records)
	}
}

func TestAttach_AProcessWithNoDispatchIDWritesNoDispatchKey(t *testing.T) {
	t.Setenv(ipcenv.DispatchIDKey, "")
	evolveDir := t.TempDir()
	c := newCenter()
	closeRoot := Attach(c, "ship", evolveDir, os.Stderr)

	c.Emit(info(signalcenter.ModuleShip, signalcenter.KindShipLanded, 7))
	closeRoot()

	if records := recordsOf(t, evolveDir, "ship"); len(records) != 1 || records[0].Dispatch != "" {
		t.Errorf("records = %+v, want one record with no dispatch stamp", records)
	}
}

func TestAttach_AnUnwritableEventsDirectoryDegradesToOneStderrWarn(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, "events"), []byte("a file where the directory should be"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newCenter()
	var seen []signalcenter.Kind
	c.Subscribe(func(e signalcenter.Event) { seen = append(seen, e.Kind) })
	var stderr bytes.Buffer
	closeRoot := Attach(c, "cycle", evolveDir, &stderr)

	c.Emit(info(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 7))
	c.Emit(warn())
	closeRoot()

	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "[events] WARN:") || !strings.Contains(lines[0], "role cycle") {
		t.Errorf("stderr = %q, want exactly one [events] WARN line that names the role", stderr.String())
	}
	if !reflect.DeepEqual(seen, []signalcenter.Kind{signalcenter.KindCycleSealed, signalcenter.KindSinkDropped}) {
		t.Errorf("another listener saw %v: a Publisher that cannot write must not stop the Center", seen)
	}
}

func TestAttach_ARefusedPublisherConfigWarnsOnceAndTheCenterStillWorks(t *testing.T) {
	evolveDir := t.TempDir()
	c := newCenter()
	var seen []signalcenter.Kind
	c.Subscribe(func(e signalcenter.Event) { seen = append(seen, e.Kind) })
	var stderr bytes.Buffer

	closeRoot := Attach(c, "not-a-role", evolveDir, &stderr)
	c.Emit(info(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 7))
	closeRoot()

	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "[events] WARN: role not-a-role publishes no events") {
		t.Errorf("stderr = %q, want one WARN that the role publishes no events", stderr.String())
	}
	if len(seen) != 1 {
		t.Errorf("another listener saw %v, want the one event", seen)
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "events")); !os.IsNotExist(err) {
		t.Errorf("stat events = %v, want no channel directory for a refused role", err)
	}
}

func TestAttach_TheCloserMakesTheBestEffortChannelsDurable(t *testing.T) {
	evolveDir := t.TempDir()
	c := newCenter()
	closeRoot := Attach(c, "cycle", evolveDir, os.Stderr)

	for range 50 {
		c.Emit(info(signalcenter.ModuleLedger, signalcenter.KindLedgerAppended, 7))
	}
	closeRoot()

	if got := len(kindsIn(t, evolveDir, "ledger")); got != 50 {
		t.Errorf("ledger holds %d records after the closer, want all 50: Close drains the queue before the process exits", got)
	}
}

func TestCloser_FlushesThenClosesThePublisherWithTheDeadline(t *testing.T) {
	t.Parallel()
	var calls []string
	var stderr bytes.Buffer
	closeRoot := closer(
		func() { calls = append(calls, "flush") },
		func(d time.Duration) int { calls = append(calls, "close:"+d.String()); return 0 },
		1750*time.Millisecond, "loop", &stderr)

	closeRoot()

	if !reflect.DeepEqual(calls, []string{"flush", "close:1.75s"}) {
		t.Errorf("calls = %v, want flush before close, closing with the deadline", calls)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing when no event is lost", stderr.String())
	}
}

func TestCloser_WritesOneStderrLineWhenCloseCountsALoss(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	closeRoot := closer(func() {}, func(time.Duration) int { return 3 }, time.Second, "ship", &stderr)

	closeRoot()

	if got, want := stderr.String(), "[events] WARN: 3 event write(s) of role ship were lost, and no gap record names them; docs/architecture/event-channels.md §5 lists the causes\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestCloser_ASecondCallFlushesAndClosesAgainButWarnsOnce(t *testing.T) {
	t.Parallel()
	var calls []string
	var stderr bytes.Buffer
	closeRoot := closer(
		func() { calls = append(calls, "flush") },
		func(time.Duration) int { calls = append(calls, "close"); return 3 },
		time.Second, "loop", &stderr)

	closeRoot()
	closeRoot()

	if !reflect.DeepEqual(calls, []string{"flush", "close", "flush", "close"}) {
		t.Errorf("calls = %v, want a second flush and close: a root that closes before an exec and defers the closer needs the final close to count the losses that follow the first", calls)
	}
	if got := strings.Count(stderr.String(), "[events] WARN:"); got != 1 {
		t.Errorf("stderr = %q, want exactly one [events] WARN for a root that calls the closer twice", stderr.String())
	}
}

func TestAttach_ALossAfterAnEarlyCloseGetsItsGapAtTheFinalClose(t *testing.T) {
	evolveDir := t.TempDir()
	c := newCenter()
	var stderr bytes.Buffer
	closeRoot := Attach(c, "loop", evolveDir, &stderr)
	c.Emit(info(signalcenter.ModuleLedger, signalcenter.KindLedgerAppended, 7))
	closeRoot()

	c.Emit(info(signalcenter.ModuleLedger, signalcenter.KindLedgerAppended, 7))
	c.Emit(info(signalcenter.ModuleLedger, signalcenter.KindLedgerAppended, 7))
	closeRoot()

	var gaps []channel.Gap
	for _, r := range rawRecordsOf(t, evolveDir, "ledger") {
		if r.Gap != nil {
			gaps = append(gaps, *r.Gap)
		}
	}
	if len(kindsIn(t, evolveDir, "ledger")) != 1 || len(gaps) != 1 || gaps[0].Dropped != 2 || gaps[0].Reason != channel.ReasonQueueFull {
		t.Errorf("ledger holds %v and gaps %+v, want the one event before the early close and one queue_full gap that names the 2 events after it", kindsIn(t, evolveDir, "ledger"), gaps)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing: the gap record names the loss, so no loss is left to report", stderr.String())
	}
}

func TestAttach_AnEmptyOrRelativeEvolveDirPublishesNothingAndWarnsOnce(t *testing.T) {
	cwd := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	for name, evolveDir := range map[string]string{"empty": "", "relative": ".evolve", "nested relative": filepath.Join("proj", ".evolve")} {
		t.Run(name, func(t *testing.T) {
			c := newCenter()
			var seen []signalcenter.Kind
			c.Subscribe(func(e signalcenter.Event) { seen = append(seen, e.Kind) })
			var stderr bytes.Buffer

			closeRoot := Attach(c, "phase-observer", evolveDir, &stderr)
			c.Emit(warn())
			closeRoot()

			lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
			if len(lines) != 1 || !strings.HasPrefix(lines[0], "[events] WARN: role phase-observer publishes no events: the evolve directory") || !strings.Contains(lines[0], "not an absolute path") {
				t.Errorf("stderr = %q, want one WARN that the evolve directory is not an absolute path", stderr.String())
			}
			if len(seen) != 1 {
				t.Errorf("another listener saw %v, want the one event: the Center keeps working", seen)
			}
			entries, rerr := os.ReadDir(cwd)
			if rerr != nil || len(entries) != 0 {
				t.Errorf("the working directory holds %v (%v), want it empty: a relative path must not put channel logs under the cwd", entries, rerr)
			}
		})
	}
}

func TestDeadlineOf_IsTheChannelCountTimesTheLockDeadline(t *testing.T) {
	t.Parallel()

	if got := deadlineOf(policy.EventsDefaults()); got != 12*250*time.Millisecond {
		t.Errorf("default deadline = %v, want 12 channels x 250ms", got)
	}
	custom := policy.EventsPolicy{Channels: make([]policy.EventChannel, 2), LockDeadlineMS: 100}
	if got := deadlineOf(custom); got != 200*time.Millisecond {
		t.Errorf("custom deadline = %v, want 2 channels x 100ms", got)
	}
}

func TestConfigOf_MapsThePolicyToThePublisherConfig(t *testing.T) {
	t.Parallel()
	ev := policy.EventsPolicy{
		Channels:  []policy.EventChannel{{Name: "a", Route: "module=loop", QoS: "lossless"}, {Name: "b", Route: "", QoS: "best_effort", SegmentMB: 2}},
		SegmentMB: 16, LockDeadlineMS: 250, QueueEvents: 8, QueueBytes: 4096, EnqueueDeadlineMS: 5,
	}

	cfg := configOf("loop-chain", filepath.Join("proj", ".evolve"), "d1", ev)

	if cfg.Root != paths.EventChannelsRoot(filepath.Join("proj", ".evolve")) || cfg.Role != "loop-chain" || cfg.Dispatch != "d1" {
		t.Errorf("root %q role %q dispatch %q", cfg.Root, cfg.Role, cfg.Dispatch)
	}
	if cfg.Log.SegmentBytes != 16<<20 || cfg.Log.LockDeadline != 250*time.Millisecond {
		t.Errorf("log = %+v, want 16 MiB segments and a 250ms lock deadline", cfg.Log)
	}
	if cfg.QueueEvents != 8 || cfg.QueueBytes != 4096 || cfg.EnqueueDeadline != 5*time.Millisecond {
		t.Errorf("queue = %d events, %d bytes, enqueue %v", cfg.QueueEvents, cfg.QueueBytes, cfg.EnqueueDeadline)
	}
	if len(cfg.Channels) != 2 || cfg.Channels[0].SegmentBytes != 0 || cfg.Channels[1].SegmentBytes != 2<<20 || string(cfg.Channels[1].QoS) != "best_effort" {
		t.Errorf("channels = %+v, want a channel to inherit the default size or override it", cfg.Channels)
	}
}
