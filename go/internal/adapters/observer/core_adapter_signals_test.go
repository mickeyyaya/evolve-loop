package observer

import (
	"context"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type recordingCenter struct {
	c      *signalcenter.Center
	mu     sync.Mutex
	events []signalcenter.Event
}

func newRecordingCenter() *recordingCenter {
	r := &recordingCenter{c: signalcenter.New()}
	r.c.Subscribe(func(e signalcenter.Event) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, e)
	})
	return r
}

func (r *recordingCenter) all() []signalcenter.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]signalcenter.Event(nil), r.events...)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = orig
	_ = w.Close()
	b, _ := io.ReadAll(r)
	_ = r.Close()
	return string(b)
}

// The workspace path is a regular file, so opening the events sink fails.
func TestCoreAdapter_SinkOpenFailureIsASignalAndTheAdapterIsWired(t *testing.T) {
	wsFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(wsFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc := newRecordingCenter()
	a := NewCoreAdapter(fastObserverPolicy())
	a.Signals = func() *signalcenter.Center { return rc.c }
	if !a.SignalsWired() {
		t.Fatal("SignalsWired reports the accessor")
	}
	var cancel func()
	stderr := captureStderr(t, func() { cancel = a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: wsFile, Cycle: 3}) })
	cancel()
	cancel()
	if stderr != "" {
		t.Errorf("no bytes on os.Stderr — the fault is a signal: %q", stderr)
	}
	got := rc.all()
	if len(got) != 1 || got[0].Code != observerengine.CodeEventsSinkOpenFailed {
		t.Fatalf("exactly one sink-open fault: %+v", got)
	}
	e := got[0]
	wantPath := observerengine.PathsFor(wsFile, "tdd").Events
	if e.Module != signalcenter.ModuleObserver || e.Kind != signalcenter.KindObserverWarning || e.Severity != signalcenter.SeverityWarn || e.Origin != "CoreAdapter.Start" || e.Cycle != 3 || e.Phase != "tdd" {
		t.Errorf("event: %+v", e)
	}
	if e.Fields["step"] != "open_sink" || e.Fields["path"] != wantPath || !strings.HasSuffix(e.Reason, "(phase runs unobserved)") {
		t.Errorf("fields/reason: %+v", e)
	}
	unwired := NewCoreAdapter()
	if unwired.SignalsWired() {
		t.Error("no accessor → unwired")
	}
	unwired.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: wsFile, Cycle: 3})() // must not panic
}

func TestCoreAdapter_WatcherLeakIsASignal(t *testing.T) {
	rc := newRecordingCenter()
	a := &CoreAdapter{Signals: func() *signalcenter.Center { return rc.c }}
	var wg sync.WaitGroup
	wg.Add(1) // the watcher never exits
	closer := &countingCloser{}
	obs := New(Config{StallS: time.Hour, PollS: time.Hour}, io.Discard)
	cancels := 0
	h := watchHandles{cancel: func() { cancels++ }, obs: obs, wg: &wg, sinkCloser: closer}
	finish := a.finishFn(5, "build", h, 50*time.Millisecond)
	var stderr string
	start := time.Now()
	stderr = captureStderr(t, func() { finish(); finish() })
	if el := time.Since(start); el > time.Second {
		t.Errorf("cancel returned after %v", el)
	}
	if stderr != "" {
		t.Errorf("no bytes on os.Stderr: %q", stderr)
	}
	got := rc.all()
	if len(got) != 1 || got[0].Code != observerengine.CodeWatcherLeaked || got[0].Origin != "CoreAdapter.Start" || got[0].Cycle != 5 || got[0].Phase != "build" {
		t.Fatalf("one leak signal: %+v", got)
	}
	if f := got[0].Fields; f["step"] != "cancel" || f["timeout_s"] != "0.05" {
		t.Errorf("fields: %v", f)
	}
	if closer.Count() != 0 {
		t.Error("the leaked watcher's sink is never closed from under it")
	}
	if cancels != 1 {
		t.Errorf("once: the underlying cancel ran %d times", cancels)
	}
	wg.Done()
}

func TestCoreAdapter_UsesTheLayoutProjection(t *testing.T) {
	ws := t.TempDir()
	a := NewCoreAdapter(fastObserverPolicy())
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: ws, Cycle: 1})
	events := observerengine.PathsFor(ws, "tdd").Events
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(events); err == nil && info.Size() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if info, err := os.Stat(events); err != nil || info.Size() == 0 {
		t.Errorf("the events sink is the projected path: %v", err)
	}
	if observerEventsSuffix != observerengine.EventsSuffix {
		t.Error("the exclusion suffix projects from the engine's layout")
	}
}

func TestCoreAdapter_StdoutLayoutAgreesWithTheBridgePIDFileConvention(t *testing.T) {
	t.Parallel()
	p := observerengine.PathsFor("/ws", "build")
	if got, want := core.BridgePIDFile(p.Stdout), "/ws/build.bridge-pid"; got != want {
		t.Errorf("BridgePIDFile(PathsFor(ws, phase).Stdout) = %q, want %q — the two -stdout.log spellings drifted", got, want)
	}
}

func TestCoreAdapter_HasNoStderrLinesAndNoEnvReads(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(fset, name, src, parser.ImportsOnly); err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"os.Stderr", "os.Getenv", "EnvLookup"} {
			if strings.Contains(string(src), banned) {
				t.Errorf("%s references %s: the adapter signals its faults and reads no environment", name, banned)
			}
		}
	}
}

func TestCoreAdapter_OriginHasOneHome(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	spellings := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		spellings += strings.Count(string(src), "\"CoreAdapter.")
	}
	if spellings != 1 {
		t.Errorf("the adapter origin is spelled %d times in production sources, want exactly 1 (const originAdapterStart)", spellings)
	}
}

func TestDefaultNudgeS_MatchesPolicyCompiledDefault(t *testing.T) {
	t.Parallel()
	if want := time.Duration(*policy.Policy{}.ObserverConfig().NudgeS) * time.Second; DefaultNudgeS != want {
		t.Errorf("DefaultNudgeS = %v, policy's compiled NudgeS = %v", DefaultNudgeS, want)
	}
}
