package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/rootwire"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func useRealRootEvents(t *testing.T) {
	t.Helper()
	prev := attachRootEventsFn
	attachRootEventsFn = rootwire.Attach
	t.Cleanup(func() { attachRootEventsFn = prev })
}

func channelRecords(t *testing.T, evolveDir, name string) []channel.Record {
	t.Helper()
	log, err := channel.New(paths.EventChannelsRoot(evolveDir), name, channel.Config{SegmentBytes: 1 << 20, LockDeadline: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := log.Read(0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("Read(%s) = %v", name, err)
	}
	var records []channel.Record
	for _, r := range batch.Records {
		if r.Signal != nil {
			records = append(records, r)
		}
	}
	return records
}

func channelKinds(t *testing.T, evolveDir, name string) []string {
	t.Helper()
	var kinds []string
	for _, r := range channelRecords(t, evolveDir, name) {
		kinds = append(kinds, r.Source+"/"+string(r.Signal.Kind))
	}
	return kinds
}

func rootEvent(module signalcenter.Module, kind signalcenter.Kind, cycle int) signalcenter.Event {
	return signalcenter.Event{Cycle: cycle, Module: module, Origin: "Test.rootEvent", Kind: kind, Severity: signalcenter.SeverityInfo, Reason: string(kind)}
}

func TestNewRoleSignalCenter_PublishesUnderItsRoleAndTheCloserDrainsTheQueues(t *testing.T) {
	useRealRootEvents(t)
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	signals, closeSignals := newRoleSignalCenter(roleSubagent, root, evolveDir, io.Discard)

	signals.Emit(rootEvent(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 7))
	signals.Emit(rootEvent(signalcenter.ModuleLedger, signalcenter.KindLedgerAppended, 7))
	closeSignals()

	if got := channelKinds(t, evolveDir, "cycle"); !slices.Equal(got, []string{"subagent/cycle.sealed"}) {
		t.Errorf("cycle holds %v, want the sealed event stamped with the role subagent", got)
	}
	if got := channelKinds(t, evolveDir, "ledger"); !slices.Equal(got, []string{"subagent/ledger.appended"}) {
		t.Errorf("ledger holds %v, want the queued event on disk once the closer returned", got)
	}
	record, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 7), signalcenter.StreamFileName))
	if err != nil || strings.Count(string(record), "\n") != 2 {
		t.Errorf("cycle record = %q, %v, want its two lines: the Publisher is one listener beside the sinks", record, err)
	}
}

func TestRootSignalCenter_CycleZeroEventsNeverReachTheCycleSink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	signals := newRootSignalCenter(root, evolveDir, io.Discard)

	signals.Emit(rootEvent(signalcenter.ModuleLoop, signalcenter.KindLoopWave, 0))
	signals.Emit(rootEvent(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 7))
	signals.Flush()

	if _, err := os.Stat(core.RunWorkspacePath(root, 0)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stat runs/cycle-0 = %v, want absent: a cycle 0 event reached the per-cycle sink", err)
	}
	for _, e := range signals.Recent() {
		if e.Kind == signalcenter.KindSinkDropped {
			t.Errorf("a signalcenter.sink_dropped event %+v: the per-cycle sink was asked for a path it has none for", e)
		}
	}
	cycleRecord, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 7), signalcenter.StreamFileName))
	if err != nil || strings.Contains(string(cycleRecord), `"kind":"loop.wave"`) || !strings.Contains(string(cycleRecord), `"kind":"cycle.sealed"`) {
		t.Errorf("cycle 7 record = %q, %v, want only its own event", cycleRecord, err)
	}
	projectRecord, err := os.ReadFile(filepath.Join(evolveDir, signalcenter.StreamFileName))
	if err != nil || !strings.Contains(string(projectRecord), `"kind":"loop.wave"`) || strings.Contains(string(projectRecord), `"kind":"cycle.sealed"`) {
		t.Errorf("project record = %q, %v, want the cycle-less event only: the project file stays until the channels replace it", projectRecord, err)
	}
}

func TestListenWhen_PassesOnlyTheEventsThatItsPredicateKeeps(t *testing.T) {
	t.Parallel()
	var cycled, projected []string
	forCycles := listenWhen(cycleStamped, func(e signalcenter.Event) { cycled = append(cycled, string(e.Kind)+"@"+strconv.Itoa(e.Cycle)) })
	forProject := listenWhen(legacyProjectRecord, func(e signalcenter.Event) { projected = append(projected, string(e.Kind)+"@"+strconv.Itoa(e.Cycle)) })

	for _, e := range []signalcenter.Event{
		{Kind: signalcenter.KindLoopWave, Cycle: 0},
		{Kind: signalcenter.KindPhaseOutcome, Cycle: 3},
		{Kind: signalcenter.KindLoopStarted, Cycle: 0},
		{Kind: signalcenter.KindLoopExit, Cycle: 0},
		{Kind: signalcenter.KindLoopHalt, Cycle: 0},
	} {
		forCycles(e)
		forProject(e)
	}

	if !slices.Equal(cycled, []string{"phase.outcome@3"}) {
		t.Errorf("the per-cycle sink saw %v, want only the cycle-stamped event", cycled)
	}
	if !slices.Equal(projected, []string{"loop.wave@0", "loop.halt@0"}) {
		t.Errorf("the project file saw %v, want the cycle-less events of the legacy kinds, and neither lifecycle kind", projected)
	}
}

var kindsBeforeTheChannels = []signalcenter.Kind{
	"phase.dispatched", "phase.outcome", "phase.aborted", "gate.rejected", "gate.corrected", "gate.passed",
	"ship.landed", "ship.error", "ship.warning", "system.failure", "quota.paused", "bridge.warning", "bridge.tripwire",
	"pane.liveness", "ledger.appended", "outcome.warning", "failurediag.warning", "carryover.warning", "failurelearning.warning",
	"config.warning", "observer.warning", "audit.warning", "inbox.warning", "runner.warning", "advisor.warning",
	"cycle.sealed", "loop.wave", "loop.warning", "loop.halt", "loop.escalation",
	"signalcenter.listener_panicked", "signalcenter.registry_drift", "signalcenter.sink_dropped",
}

func TestLegacyProjectRecord_AllowsOnlyTheKindsThatExistedBeforeTheChannels(t *testing.T) {
	t.Parallel()
	if len(kindsBeforeTheChannels) != 33 {
		t.Fatalf("the frozen list holds %d kinds, want the 33 that existed before the channels", len(kindsBeforeTheChannels))
	}
	for _, k := range kindsBeforeTheChannels {
		if !k.Known() {
			t.Errorf("kind %s left the registry: a kind of the legacy project file cannot be retired without a decision about that file", k)
		}
	}
	for _, k := range signalcenter.Kinds() {
		want := slices.Contains(kindsBeforeTheChannels, k)
		if got := legacyProjectRecord(signalcenter.Event{Kind: k}); got != want {
			t.Errorf("legacyProjectRecord(%s) = %v, want %v: the project file is frozen at the kinds that existed before the channels, so a new kind reaches the channels only", k, got, want)
		}
		if legacyProjectRecord(signalcenter.Event{Kind: k, Cycle: 5}) {
			t.Errorf("a cycle-stamped %s is a project record: it belongs to the file of its cycle", k)
		}
	}
	if legacyProjectRecord(signalcenter.Event{Kind: "future.kind"}) {
		t.Error("a kind that nobody registered is a project record: the allow-list admits only the frozen kinds")
	}
}

func TestRootSignalCenter_TheLoopLifecycleStaysOutOfTheLegacyProjectFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	signals := newRootSignalCenter(root, evolveDir, io.Discard)

	signals.Emit(rootEvent(signalcenter.ModuleLoop, signalcenter.KindLoopStarted, 0))
	signals.Emit(rootEvent(signalcenter.ModuleLoop, signalcenter.KindLoopExit, 0))
	signals.Flush()

	if _, err := os.Stat(filepath.Join(evolveDir, signalcenter.StreamFileName)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stat signals.ndjson = %v, want absent: the file is frozen at its legacy kinds, and a clean loop writes nothing to it", err)
	}
}

func normalizedLines(t *testing.T, path string) []signalcenter.Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var events []signalcenter.Event
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var e signalcenter.Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		e.TS = ""
		events = append(events, e)
	}
	return events
}

func TestRootSignalCenter_ThePublisherLeavesTheCycleRecordUnchanged(t *testing.T) {
	useRealRootEvents(t)
	emitAll := func(c *signalcenter.Center) {
		c.Emit(rootEvent(signalcenter.ModuleLoop, signalcenter.KindLoopWave, 0))
		c.Emit(rootEvent(signalcenter.ModuleOrchestrator, signalcenter.KindPhaseOutcome, 4))
		c.Emit(signalcenter.Event{Cycle: 4, Module: signalcenter.ModuleSignalCenter, Origin: "Test.emit", Kind: signalcenter.KindSinkDropped,
			Severity: signalcenter.SeverityWarn, Code: signalcenter.CodeSinkDropped, Reason: "a warning", Fields: map[string]string{"a": "b"}})
		c.Emit(rootEvent(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 4))
	}
	plainRoot, publishedRoot := t.TempDir(), t.TempDir()
	plain := newRootSignalCenter(plainRoot, filepath.Join(plainRoot, ".evolve"), io.Discard)
	published, closeSignals := newRoleSignalCenter(roleCycle, publishedRoot, filepath.Join(publishedRoot, ".evolve"), io.Discard)

	emitAll(plain)
	plain.Flush()
	emitAll(published)
	closeSignals()

	for _, rel := range []string{filepath.Join("runs", "cycle-4"), ""} {
		plainPath := filepath.Join(plainRoot, ".evolve", rel, signalcenter.StreamFileName)
		publishedPath := filepath.Join(publishedRoot, ".evolve", rel, signalcenter.StreamFileName)
		if want, got := normalizedLines(t, plainPath), normalizedLines(t, publishedPath); !slices.EqualFunc(want, got, func(a, b signalcenter.Event) bool {
			return a.Seq == b.Seq && a.Kind == b.Kind && a.Reason == b.Reason && a.Cycle == b.Cycle && a.Code == b.Code && a.Severity == b.Severity && a.Fields["a"] == b.Fields["a"]
		}) {
			t.Errorf("%s changed when the Publisher joined:\nwithout %+v\nwith    %+v", filepath.Join(rel, signalcenter.StreamFileName), want, got)
		}
	}
}

func TestNewRoleSignalCenter_AnUnwritableEventsDirectoryNeverFailsTheRootAndWarnsOnce(t *testing.T) {
	useRealRootEvents(t)
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "events"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	signals, closeSignals := newRoleSignalCenter(roleCycle, root, evolveDir, &console)

	signals.Emit(rootEvent(signalcenter.ModuleOrchestrator, signalcenter.KindCycleSealed, 7))
	closeSignals()

	warns := 0
	for _, line := range strings.Split(console.String(), "\n") {
		if strings.HasPrefix(line, "[events] WARN:") {
			warns++
		}
	}
	if warns != 1 {
		t.Errorf("console = %q, want exactly one [events] WARN", console.String())
	}
	if record, err := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, 7), signalcenter.StreamFileName)); err != nil || !strings.Contains(string(record), `"kind":"cycle.sealed"`) {
		t.Errorf("cycle record = %q, %v: a failing Publisher must not take the cycle record down with it", record, err)
	}
}

func TestOrchDeps_CloseSignalsCallsItsCloserOrFlushesAStubbedCenter(t *testing.T) {
	t.Parallel()
	closed := 0

	orchDeps{CloseSignals: func() { closed++ }}.closeSignals()
	orchDeps{}.closeSignals()
	flushed := signalcenter.New()
	var seen []signalcenter.Kind
	flushed.Subscribe(func(e signalcenter.Event) { seen = append(seen, e.Kind) })
	flushed.Emit(rootEvent(signalcenter.ModuleLoop, signalcenter.KindLoopWave, 0))
	orchDeps{Signals: flushed}.closeSignals()

	if closed != 1 || len(seen) != 1 {
		t.Errorf("closer ran %d times and the stubbed Center saw %v, want the closer once and the Center flushed without a panic", closed, seen)
	}
}

func TestOrchDeps_CloseSignalsWaitsForADrainInFlightWhenNoCloserIsWired(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	c.Subscribe(func(e signalcenter.Event) {
		if e.Reason == "slow" {
			close(entered)
			<-release
		}
	})
	go c.Emit(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: "Test.slow", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "slow"})
	<-entered
	done := make(chan struct{})

	go func() {
		orchDeps{Signals: c}.closeSignals()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("closeSignals returned while the Center still drained an event: a stubbed deps value must flush its Center")
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("closeSignals did not return after the drain ended")
	}
}

func TestWireOrchestratorDeps_PublishesAsTheRoleOfItsRun(t *testing.T) {
	useRealRootEvents(t)
	for name, tc := range map[string]struct {
		run      routingRun
		wantRole string
	}{
		"a cycle run":      {routingRun{}, "cycle"},
		"a loop batch run": {routingRun{role: roleLoop}, "loop"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			evolveDir := filepath.Join(root, ".evolve")
			if err := os.MkdirAll(evolveDir, 0o755); err != nil {
				t.Fatal(err)
			}

			d := wireOrchestratorDeps(root, evolveDir, io.Discard, tc.run)
			d.Signals.Emit(rootEvent(signalcenter.ModuleLoop, signalcenter.KindLoopWave, 0))
			d.closeSignals()

			if got := channelKinds(t, evolveDir, "loop"); !slices.Equal(got, []string{tc.wantRole + "/loop.wave"}) {
				t.Errorf("loop channel holds %v, want the event stamped with role %s", got, tc.wantRole)
			}
		})
	}
}

func simulateProject(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	initRepoWithCommit(t, root)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", ".gitignore")
	gitOut(t, root, "commit", "-q", "-m", "ignore the state directory")
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func TestWireSimulateOrchestrator_PublishesAsSimulateAndDispatchesEachPhase(t *testing.T) {
	useRealRootEvents(t)
	root, evolveDir := simulateProject(t)

	d := wireSimulateOrchestrator(root, evolveDir, io.Discard)
	res, err := d.Orchestrator.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "simulate-publishes"})
	d.closeSignals()

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	dispatched, sealed := 0, 0
	for _, kind := range channelKinds(t, evolveDir, "cycle") {
		switch kind {
		case "simulate/phase.dispatched":
			dispatched++
		case "simulate/cycle.sealed":
			sealed++
		default:
			if !strings.HasPrefix(kind, "simulate/") {
				t.Errorf("record %q has another role than simulate", kind)
			}
		}
	}
	if dispatched != len(res.PhasesRun) || sealed != 1 {
		t.Errorf("cycle channel has %d phase.dispatched and %d cycle.sealed, want %d and 1", dispatched, sealed, len(res.PhasesRun))
	}
}

func TestWireSimulateOrchestrator_AnUnwritableEventsDirectoryNeverFailsTheCycle(t *testing.T) {
	useRealRootEvents(t)
	root, evolveDir := simulateProject(t)
	if err := os.WriteFile(filepath.Join(evolveDir, "events"), []byte("a file where the directory should be"), 0o644); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer

	d := wireSimulateOrchestrator(root, evolveDir, &console)
	res, err := d.Orchestrator.RunCycle(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "simulate-unwritable-events"})
	d.closeSignals()

	if err != nil || res.FinalVerdict != core.VerdictPASS {
		t.Fatalf("RunCycle = %q, %v, want the same PASS as with a writable events directory", res.FinalVerdict, err)
	}
	warns := 0
	for _, line := range strings.Split(console.String(), "\n") {
		if strings.HasPrefix(line, "[events] WARN:") {
			warns++
		}
	}
	if warns != 1 {
		t.Errorf("console holds %d [events] WARN lines, want exactly 1:\n%s", warns, console.String())
	}
	if record, rerr := os.ReadFile(filepath.Join(core.RunWorkspacePath(root, res.Cycle), signalcenter.StreamFileName)); rerr != nil || !strings.Contains(string(record), `"kind":"cycle.sealed"`) {
		t.Errorf("cycle record = %q, %v, want the cycle.sealed line: the cycle record does not depend on the channels", record, rerr)
	}
}
