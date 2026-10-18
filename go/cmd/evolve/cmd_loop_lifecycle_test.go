package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func recordedCenter() (*signalcenter.Center, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return c, got
}

func lifecycleKinds(events []signalcenter.Event) []string {
	var kinds []string
	for _, e := range events {
		if e.Kind == signalcenter.KindLoopStarted || e.Kind == signalcenter.KindLoopExit {
			kinds = append(kinds, string(e.Kind))
		}
	}
	return kinds
}

func TestLoop_EmitsLoopStartedWithItsProcStart(t *testing.T) {
	t.Parallel()
	want, err := proctree.StartOf(os.Getpid())
	if err != nil {
		t.Fatalf("proctree.StartOf(%d) = %v", os.Getpid(), err)
	}
	c, got := recordedCenter()

	announceLoop(c, io.Discard)

	if len(*got) != 1 {
		t.Fatalf("events = %+v, want only loop.started when the loop starts", *got)
	}
	e := (*got)[0]
	if e.Kind != signalcenter.KindLoopStarted || e.Module != signalcenter.ModuleLoop || e.Severity != signalcenter.SeverityInfo || e.Cycle != 0 {
		t.Errorf("event = %+v, want an INFO loop.started of the loop module with no cycle", e)
	}
	if e.Fields["proc_start"] != want || e.Fields["proc_start"] == "" {
		t.Errorf("proc_start = %q, want %q: the reader of the last will compares it with the start time of the pid", e.Fields["proc_start"], want)
	}
}

func TestLoop_EmitsLoopExitAtExit(t *testing.T) {
	t.Parallel()
	c, got := recordedCenter()
	finish := announceLoop(c, io.Discard)
	code := 3

	finish(&code)

	if !slices.Equal(lifecycleKinds(*got), []string{"loop.started", "loop.exit"}) {
		t.Fatalf("lifecycle = %v, want loop.started then loop.exit", lifecycleKinds(*got))
	}
	e := (*got)[1]
	if e.Severity != signalcenter.SeverityInfo || e.Fields["exit"] != "3" || e.Module != signalcenter.ModuleLoop || e.Reason != "the loop process exits with code 3" {
		t.Errorf("loop.exit = %+v, want INFO with the exit code 3", e)
	}
}

func TestLoop_APanicEmitsNoLoopExitSoTheLastWillCanReportTheCrash(t *testing.T) {
	t.Parallel()
	c, got := recordedCenter()
	finish := announceLoop(c, io.Discard)
	code := 0

	recovered := func() (crash any) {
		defer func() { crash = recover() }()
		defer finish(&code)
		panic("the loop crashed")
	}()

	if recovered != "the loop crashed" {
		t.Errorf("recovered %v, want the original panic to continue unwinding", recovered)
	}
	if !slices.Equal(lifecycleKinds(*got), []string{"loop.started"}) {
		t.Errorf("lifecycle = %v, want loop.started alone: a loop.exit for a crash would hide it from the last will", lifecycleKinds(*got))
	}
}

func TestAnnounceLoop_AnUnreadableStartTimeWarnsOnceAndEmitsNothing(t *testing.T) {
	prev := loopProcStartFn
	t.Cleanup(func() { loopProcStartFn = prev })
	loopProcStartFn = func(int) (string, error) { return "", errors.New("sysctl refused") }
	c, got := recordedCenter()
	var stderr bytes.Buffer

	finish := announceLoop(c, &stderr)
	code := 0
	finish(&code)

	if len(*got) != 0 {
		t.Errorf("events = %+v, want none: a loop.started without its start time would make the last will report a live loop as lost", *got)
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "[loop] WARN: no loop.started event") || !strings.Contains(lines[0], "sysctl refused") {
		t.Errorf("stderr = %q, want one WARN that names the cause", stderr.String())
	}
}

func TestRunLoopBatch_AnnouncesTheLoopAndExitsWithItsCode(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	c, got := recordedCenter()
	var role string
	prevDeps, prevPf := wireOrchestratorDepsFn, runLoopPreflightFn
	t.Cleanup(func() { wireOrchestratorDepsFn, runLoopPreflightFn = prevDeps, prevPf })
	wireOrchestratorDepsFn = func(_, _ string, _ io.Writer, run routingRun) orchDeps {
		role = run.role
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger(), Signals: c}
	}
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }
	var stdout, stderr bytes.Buffer

	rc := runLoop([]string{"--project-root", projectRoot, "--evolve-dir", evolveDir, "--goal-text", "anything", "--cycles", "1", "--force-fresh"}, nil, &stdout, &stderr)

	if rc != 2 || role != roleLoop {
		t.Fatalf("rc = %d, role = %q, want the preflight halt (2) under the role loop", rc, role)
	}
	if !slices.Equal(lifecycleKinds(*got), []string{"loop.started", "loop.exit"}) {
		t.Fatalf("lifecycle = %v, want loop.started then loop.exit", lifecycleKinds(*got))
	}
	if exit := (*got)[len(*got)-1]; exit.Kind != signalcenter.KindLoopExit || exit.Fields["exit"] != "2" {
		t.Errorf("last event = %+v, want loop.exit carrying the exit code 2 of the batch", exit)
	}
}

func lifecycleIn(t *testing.T, evolveDir, name string) []string {
	t.Helper()
	var lifecycle []string
	for _, r := range channelRecords(t, evolveDir, name) {
		if r.Signal.Kind == signalcenter.KindLoopStarted || r.Signal.Kind == signalcenter.KindLoopExit {
			lifecycle = append(lifecycle, r.Source+"/"+string(r.Signal.Kind))
		}
	}
	return lifecycle
}

func haltedLoopArgs(projectRoot, evolveDir string) []string {
	return []string{"--project-root", projectRoot, "--evolve-dir", evolveDir, "--goal-text", "anything", "--cycles", "1", "--force-fresh"}
}

func TestRunLoopBatch_AnnouncesThroughARealPublisherBeforeItCloses(t *testing.T) {
	useRealRootEvents(t)
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prevDeps, prevPf := wireOrchestratorDepsFn, runLoopPreflightFn
	t.Cleanup(func() { wireOrchestratorDepsFn, runLoopPreflightFn = prevDeps, prevPf })
	wireOrchestratorDepsFn = func(root, dir string, console io.Writer, run routingRun) orchDeps {
		signals, closeSignals := newRoleSignalCenter(run.centerRole(), root, dir, console)
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger(), Signals: signals, CloseSignals: closeSignals}
	}
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }

	rc := runLoop(haltedLoopArgs(projectRoot, evolveDir), nil, io.Discard, io.Discard)

	if rc != 2 {
		t.Fatalf("rc = %d, want the preflight halt (2)", rc)
	}
	for _, name := range []string{"loop", "signals"} {
		if got := lifecycleIn(t, evolveDir, name); !slices.Equal(got, []string{"loop/loop.started", "loop/loop.exit"}) {
			t.Errorf("channel %s lifecycle = %v, want the pair of the role loop: loop.exit is emitted before the Publisher closes, so the best-effort channel holds it too", name, got)
		}
	}
	records := channelRecords(t, evolveDir, "signals")
	if last := records[len(records)-1]; last.Signal.Kind != signalcenter.KindLoopExit || last.Signal.Fields["exit"] != "2" {
		t.Errorf("last record of signals = %+v, want loop.exit with the exit code 2", last.Signal)
	}
}

func TestRunLoopBatch_UsesTheAnnouncerOfItsConfig(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	c, got := recordedCenter()
	prevDeps, prevPf := wireOrchestratorDepsFn, runLoopPreflightFn
	t.Cleanup(func() { wireOrchestratorDepsFn, runLoopPreflightFn = prevDeps, prevPf })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger(), Signals: c}
	}
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }
	cfg, rc := parseLoopArgs(haltedLoopArgs(projectRoot, evolveDir), io.Discard)
	if rc != 0 {
		t.Fatalf("parseLoopArgs rc = %d", rc)
	}
	var announcedOn *signalcenter.Center
	exitSeen := -1
	cfg.announce = func(signals *signalcenter.Center, _ io.Writer) func(*int) {
		announcedOn = signals
		return func(exit *int) { exitSeen = *exit }
	}

	exit := runLoopBatch(cfg, nil, io.Discard, io.Discard)

	if exit != 2 || announcedOn != c || exitSeen != 2 || len(lifecycleKinds(*got)) != 0 {
		t.Errorf("exit = %d, announced on the batch Center = %v, exit seen = %d, lifecycle = %v, want the halt, the injected announcer on the batch Center with exit 2, and no built-in announcement", exit, announcedOn == c, exitSeen, lifecycleKinds(*got))
	}
}

func TestLoopConfig_AnnouncesTheLoopUnlessAnAnnouncerIsInjected(t *testing.T) {
	t.Parallel()
	c, got := recordedCenter()
	quiet, quietGot := recordedCenter()

	loopConfig{}.announcer()(c, io.Discard)
	loopConfig{announce: silentLoop}.announcer()(quiet, io.Discard)

	if !slices.Equal(lifecycleKinds(*got), []string{"loop.started"}) || len(*quietGot) != 0 {
		t.Errorf("default announcer emitted %v and the silent one emitted %d events, want loop.started and none", lifecycleKinds(*got), len(*quietGot))
	}
}

func TestRunLoopChain_AChainedBatchGetsASilentAnnouncer(t *testing.T) {
	root, evolveDir := u13ChainEnv(t, 1)
	u13StubRefresh(t, func() bool { return false })
	var announced []int
	prev := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prev })
	runLoopBatchFn = func(cfg loopConfig, _ io.Reader, _, _ io.Writer) int {
		c, got := recordedCenter()
		code := 0
		cfg.announcer()(c, io.Discard)(&code)
		announced = append(announced, len(*got))
		return 0
	}

	runLoopChain(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 1}, nil, io.Discard, io.Discard)

	if !slices.Equal(announced, []int{0}) {
		t.Errorf("a chained batch announced %v events, want one batch that announces nothing: the chain Center announces the process", announced)
	}
}

func TestRunLoopChain_AnnouncesTheLoopOnTheChainCenterBeforeItCloses(t *testing.T) {
	useRealRootEvents(t)
	root, evolveDir := u13ChainEnv(t, 1)
	u13StubRefresh(t, func() bool { return false })
	prev := runLoopBatchFn
	t.Cleanup(func() { runLoopBatchFn = prev })
	runLoopBatchFn = func(loopConfig, io.Reader, io.Writer, io.Writer) int { return 0 }
	var stdout bytes.Buffer

	exit := runLoopChain(loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, policy.ChainConfig{Enabled: true, MaxBatches: 1}, nil, &stdout, io.Discard)

	for _, name := range []string{"loop", "signals"} {
		if got := lifecycleIn(t, evolveDir, name); !slices.Equal(got, []string{"loop-chain/loop.started", "loop-chain/loop.exit"}) {
			t.Errorf("channel %s lifecycle = %v, want one pair from the role loop-chain: loop.exit is emitted before the Publisher closes, so the best-effort channel holds it too", name, got)
		}
	}
	records := channelRecords(t, evolveDir, "loop")
	if last := records[len(records)-1]; last.Signal.Fields["exit"] != strconv.Itoa(exit) || last.Signal.PID != os.Getpid() {
		t.Errorf("last record = %+v, want the exit code %d of the chain and this pid", last.Signal, exit)
	}
}
