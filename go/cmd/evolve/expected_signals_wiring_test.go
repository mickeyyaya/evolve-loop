package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func writeStream(t *testing.T, dir string, events ...signalcenter.Event) {
	t.Helper()
	var buf bytes.Buffer
	for i, e := range events {
		e.SchemaVersion, e.Seq = signalcenter.SchemaVersion, uint64(i+1)
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(append(line, '\n'))
	}
	if err := os.WriteFile(filepath.Join(dir, signalcenter.StreamFileName), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func oldWaveStart() signalcenter.Event {
	return signalcenter.Event{
		TS: time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano), Module: signalcenter.ModuleLoop, Origin: "t",
		Kind: signalcenter.KindWaveStarted, Severity: signalcenter.SeverityInfo, Reason: "wave 1 started", Fields: map[string]string{"wave": "1"},
	}
}

func warnsOf(c *signalcenter.Center) *[]signalcenter.Event {
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) {
		if e.Kind == signalcenter.KindExpectedAbsent {
			*got = append(*got, e)
		}
	})
	return got
}

func tick(c *signalcenter.Center) {
	c.Emit(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: "t", Kind: signalcenter.KindLoopTick, Severity: signalcenter.SeverityInfo, Reason: "tick"})
}

func wireAt(c *signalcenter.Center, dir string, console *bytes.Buffer, at time.Time) {
	wireExpectedSignalsWith(c, expectedWiring{evolveDir: dir, console: console, now: func() time.Time { return at }})
}

func TestWireExpectedSignals_AStartFromAnotherProcessWarnsOnTheNextTickAndOnlyOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	u13WritePolicy(t, dir, `{"expected_signal":{"wave_seconds":3600}}`)
	waveCenter, closeWave := newRoleSignalCenter("wave", root, dir, io.Discard)
	waveCenter.Emit(waveEvent(signalcenter.KindWaveStarted, "wave 1 launched", map[string]string{"wave": "1", "run_id": "r1"}))
	closeWave()

	later := time.Now().Add(2 * time.Hour)
	var console bytes.Buffer
	first := signalcenter.New(signalcenter.WithClock(func() time.Time { return later }))
	first.Subscribe(first.NDJSONSink(func(int) string { return filepath.Join(dir, signalcenter.StreamFileName) }))
	warns := warnsOf(first)
	wireAt(first, dir, &console, later)
	if len(*warns) != 1 || (*warns)[0].Fields["pair"] != "wave" {
		t.Fatalf("the boot poll finds the other process's start: %+v", *warns)
	}
	tick(first)
	if len(*warns) != 1 {
		t.Errorf("a tick repeats nothing: %d", len(*warns))
	}

	second := signalcenter.New()
	again := warnsOf(second)
	wireAt(second, dir, &console, later)
	tick(second)
	if len(*again) != 0 || console.Len() != 0 {
		t.Errorf("a second boot over the same stream warns nothing: warns=%d console=%q", len(*again), console.String())
	}
}

func TestWireExpectedSignals_ATickPollsTheStreamForStartsThatArrivedAfterBoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	u13WritePolicy(t, dir, `{"expected_signal":{"wave_seconds":3600}}`)
	loop := signalcenter.New()
	warns := warnsOf(loop)
	var console bytes.Buffer
	wireAt(loop, dir, &console, time.Now().Add(2*time.Hour))
	tick(loop)
	if len(*warns) != 0 {
		t.Fatalf("nothing is open yet: %d", len(*warns))
	}
	waveCenter, closeWave := newRoleSignalCenter("wave", root, dir, io.Discard)
	waveCenter.Emit(waveEvent(signalcenter.KindWaveStarted, "wave 2 launched", map[string]string{"wave": "2", "run_id": "r2"}))
	closeWave()
	tick(loop)
	if len(*warns) != 1 {
		t.Errorf("the tick reads the new start from the stream: %d", len(*warns))
	}
}

func TestWireExpectedSignals_AMissingStreamIsNotAnError(t *testing.T) {
	t.Parallel()
	c := signalcenter.New()
	warns := warnsOf(c)
	var console bytes.Buffer
	wireExpectedSignals(c, t.TempDir(), &console)
	tick(c)
	if len(*warns) != 0 || console.Len() != 0 {
		t.Errorf("a missing stream is a clean start: warns=%d console=%q", len(*warns), console.String())
	}
}

func TestWireExpectedSignals_AnUnreadableStreamWarnsAndKeepsListening(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, signalcenter.StreamFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	var console bytes.Buffer
	wireExpectedSignals(signalcenter.New(), dir, &console)
	if !strings.Contains(console.String(), "[expected-signal] WARN") || !strings.Contains(console.String(), "stream") {
		t.Errorf("an unreadable stream is a console WARN: %q", console.String())
	}
}

func TestWireExpectedSignals_ABadPolicyWarnsAndTheDefaultsApply(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	u13WritePolicy(t, dir, `{"expected_signal":{"phase_seconds":-1}}`)
	writeStream(t, dir, oldWaveStart())
	c := signalcenter.New()
	warns := warnsOf(c)
	var console bytes.Buffer
	wireExpectedSignals(c, dir, &console)
	if !strings.Contains(console.String(), "policy") || len(*warns) != 1 {
		t.Errorf("the bad policy is reported and the default deadlines apply: console=%q warns=%d", console.String(), len(*warns))
	}
}

func TestWireExpectedSignals_ThePolicyDeadlineGovernsTheWarning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	u13WritePolicy(t, dir, `{"expected_signal":{"wave_seconds":999999999}}`)
	writeStream(t, dir, oldWaveStart())
	c := signalcenter.New()
	warns := warnsOf(c)
	wireExpectedSignals(c, dir, &bytes.Buffer{})
	tick(c)
	if len(*warns) != 0 {
		t.Errorf("a long policy deadline keeps the start open: %d", len(*warns))
	}
}

func TestNewRoleSignalCenter_OnlyTheLoopRoleGetsTheExpectedSignalListener(t *testing.T) {
	t.Parallel()
	for role, want := range map[string]int{roleLoop: 1, roleShip: 0} {
		root := t.TempDir()
		dir := filepath.Join(root, ".evolve")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeStream(t, dir, oldWaveStart())
		c, closeSignals := newRoleSignalCenter(role, root, dir, &bytes.Buffer{})
		tick(c)
		closeSignals()
		raw, err := os.ReadFile(filepath.Join(dir, signalcenter.StreamFileName))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(raw), string(signalcenter.KindExpectedAbsent)); got != want {
			t.Errorf("role %s: %d persisted warnings, want %d", role, got, want)
		}
	}
}

func TestPairProjectRecord_KeepsEachPairKindOnceWhateverItsCycle(t *testing.T) {
	t.Parallel()
	for kind := range signalcenter.ExpectedKinds() {
		cycled := signalcenter.Event{Kind: kind, Cycle: 9}
		if !pairProjectRecord(cycled) {
			t.Errorf("%s of cycle 9 belongs to the project stream", kind)
		}
		if legacyProjectRecord(signalcenter.Event{Kind: kind}) && pairProjectRecord(signalcenter.Event{Kind: kind}) {
			t.Errorf("%s of cycle 0 must not be written twice", kind)
		}
	}
	if pairProjectRecord(signalcenter.Event{Kind: signalcenter.KindGatePassed, Cycle: 9}) {
		t.Error("a cycle-stamped event outside the pairs stays in its run stream")
	}
}

func TestRootSignalCenter_APairEventOfACycleReachesTheProjectStreamOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	c := newRootSignalCenter(root, dir, io.Discard)
	for _, cycle := range []int{0, 4} {
		c.Emit(signalcenter.Event{Cycle: cycle, Module: signalcenter.ModuleOrchestrator, Origin: "t", Kind: signalcenter.KindPhaseDispatched, Severity: signalcenter.SeverityInfo, Reason: "r", Phase: "build"})
	}
	c.Flush()
	raw, err := os.ReadFile(filepath.Join(dir, signalcenter.StreamFileName))
	if err != nil || strings.Count(string(raw), string(signalcenter.KindPhaseDispatched)) != 2 {
		t.Errorf("two events, two lines: %q %v", raw, err)
	}
}

func TestWireExpectedSignals_OnlyALoopTickPollsTheStream(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	u13WritePolicy(t, dir, `{"expected_signal":{"wave_seconds":3600}}`)
	c := signalcenter.New()
	warns := warnsOf(c)
	wireExpectedSignals(c, dir, &bytes.Buffer{})
	writeStream(t, dir, oldWaveStart())
	c.Emit(signalcenter.Event{Module: signalcenter.ModuleLoop, Origin: "t", Kind: signalcenter.KindLoopWave, Severity: signalcenter.SeverityInfo, Reason: "not a tick"})
	if len(*warns) != 0 {
		t.Fatalf("an event that is not a tick does not poll: %+v", *warns)
	}
	tick(c)
	if len(*warns) != 1 {
		t.Errorf("a tick polls and warns once: %d", len(*warns))
	}
}

func TestExpectedPairs_EveryDeadlineKeyResolvesThroughPolicy(t *testing.T) {
	t.Parallel()
	var none *policy.ExpectedSignalPolicy
	for _, p := range signalcenter.DefaultExpectedPairs() {
		if d, ok := none.Deadline(p.DeadlineKey); !ok || d <= 0 {
			t.Errorf("pair %s: deadline key %q resolves to (%v, %v), want a policy default: a renamed key turns the pair off", p.Name, p.DeadlineKey, d, ok)
		}
	}
}
