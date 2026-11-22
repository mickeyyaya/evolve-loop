package dashboard

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func TestAPICoverNamedExports(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	root := seedProject(t, now)

	snap := Collect(root, now)
	var queue QueueSummary = snap.Queue
	if len(queue.Pending) == 0 {
		t.Fatal("seeded queue has no pending item")
	}
	var item QueueItem = queue.Pending[0]
	if item.ID == "" {
		t.Fatalf("pending item has no id: %+v", item)
	}
	if len(snap.Quality) != len(qualitypanel.Indexes()) {
		t.Fatalf("quality count = %d, want %d", len(snap.Quality), len(qualitypanel.Indexes()))
	}
	var reading QualityReading = snap.Quality[0]
	if reading.Index == "" || reading.Stage == "" {
		t.Fatalf("quality reading %+v lacks index/stage", reading)
	}
	if len(snap.Trend.Points) == 0 || len(snap.Fingerprints) == 0 {
		t.Fatalf("seeded trend/fingerprints empty: %d %d", len(snap.Trend.Points), len(snap.Fingerprints))
	}
	var point TrendPoint = snap.Trend.Points[0]
	var stat FingerprintStat = snap.Fingerprints[0]
	if point.Cycle == 0 || stat.Fingerprint == "" || stat.Count == 0 {
		t.Fatalf("trend point %+v / fingerprint %+v carry no data", point, stat)
	}
	if rounds := snap.Cycles[2].Failure.Rounds; len(rounds) > 0 {
		var round AuditRound = rounds[0]
		if round.Round == 0 {
			t.Fatalf("audit round is unnamed: %+v", round)
		}
	}
	var ()
	for _, st := range []string{StateRunning, StatePass, StateWarn, StateFail, StateHalted, StateIncomplete} {
		if st == "" {
			t.Fatal("empty state constant")
		}
	}
	if snap.Cycles[0].State != StateRunning || snap.Cycles[2].State != StateFail {
		t.Fatalf("states: %s %s", snap.Cycles[0].State, snap.Cycles[2].State)
	}
	if got := New(root, Options{}).Snapshot(now); len(got.Cycles) != len(snap.Cycles) || got.Cycles[0].State != StateRunning || got.Cycles[2].State != StateFail {
		t.Fatalf("Server.Snapshot = %d cycles, want the %d of Collect with the same states", len(got.Cycles), len(snap.Cycles))
	}

	list, err := ListArtifacts(root, 3)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListArtifacts: %v %d", err, len(list))
	}
	var info ArtifactInfo = list[0]
	if info.Name == "" || info.Size <= 0 {
		t.Fatalf("ListArtifacts[0] = %+v, want a named non-empty file", info)
	}
	if _, err := ReadArtifact(root, 3, core.RunStateFile); err != nil {
		t.Fatalf("ReadArtifact: %v", err)
	}
	if _, err := ReadArtifact(root, 3, "../x.md"); !errors.Is(err, ErrArtifactNotAllowed) {
		t.Fatalf("traversal: %v", err)
	}
	writeFile(t, filepath.Join(core.RunWorkspacePath(root, 3), "huge.log"), string(make([]byte, ArtifactMaxBytes+1)))
	if _, err := ReadArtifact(root, 3, "huge.log"); !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("cap: %v", err)
	}

	if DefaultAddr == "" || DefaultAddr[:10] != "127.0.0.1:" {
		t.Fatalf("DefaultAddr must be loopback: %q", DefaultAddr)
	}
	s := New(root, Options{PollInterval: 5 * time.Millisecond, MaxCycles: 3, KeepAlive: time.Second, Now: func() time.Time { return now }})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	if resp, body := get(t, ts.URL+"/api/snapshot"); resp.StatusCode != 200 || len(body) == 0 {
		t.Fatalf("snapshot via Handler: %d", resp.StatusCode)
	}
	cancel()

	cancelled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := s.ListenAndServe(cancelled, "127.0.0.1:0"); err != nil {
		t.Fatalf("ListenAndServe with a cancelled ctx: %v", err)
	}
	if err := s.ListenAndServe(cancelled, "256.0.0.1:1"); err == nil {
		t.Fatal("ListenAndServe on an unbindable address must error")
	}
	_ = os.Remove(paths.LoopStopPath(paths.EvolveDirOf(root)))
}
