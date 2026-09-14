package observer

// events_golden_test.go — ADR-0103 unit 12 step 0, G4: the live adapter's
// events file (started / stall_no_output / stopped, its OWN lowercase
// envelope — observer.go:57-66, :249-269) byte-equal to a golden captured on
// 8e8f080f BEFORE core_adapter.go / observer.go were edited. The stall is
// scripted: no stdout growth, no workspace activity, a liveness probe that
// answers false, a clock that jumps past StallS from the third read on. Kills
// M20 (a severity uppercased), M21 (an Event field added or renamed).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestObserver_EventsFileMatchesGolden(t *testing.T) {
	t.Parallel()
	sink := &syncSink{}
	t0 := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	o := New(Config{
		StallS: time.Hour, PollS: 5 * time.Millisecond, Cycle: 7, Phase: "build", Agent: "build",
		StdoutLog:     filepath.Join(t.TempDir(), "build-stdout.log"), // never created: no growth
		LivenessProbe: func() bool { return false },
	}, sink)
	var mu sync.Mutex
	calls := 0
	o.nowFunc = func() time.Time { // started (1), lastGrowth (2) at t0; the threshold read and every emit after at +2h
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls <= 2 {
			return t0
		}
		return t0.Add(2 * time.Hour)
	}
	done := make(chan error, 1)
	go func() { done <- o.Watch(context.Background()) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(sink.String(), `"stall_no_output"`) {
		time.Sleep(5 * time.Millisecond)
	}
	if err := o.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Watch returned %v on Stop, want nil", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "adapter-events.golden.ndjson"))
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	if got := sink.String(); got != string(want) {
		t.Fatalf("the adapter's events file drifted from the golden:\n got: %s\nwant: %s", got, want)
	}
}
