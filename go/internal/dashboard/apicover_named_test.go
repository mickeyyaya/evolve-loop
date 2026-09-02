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
)

// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) through the shapes its real
// consumer — cmd_dashboard.go — relies on: the one-shot Collect for
// --snapshot, New/Options/Server for the served mode, the artifact reader the
// detail page calls, and the closed state vocabulary the page colours by.
func TestAPICoverNamedExports(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	root := seedProject(t, now)

	// Collect + the Snapshot tree.
	snap := Collect(root, now)
	var (
		_ Snapshot        = *snap
		_ LoopStatus      = snap.Loop
		_ QueueSummary    = snap.Queue
		_ []QueueItem     = snap.Queue.Pending
		_ []CycleSummary  = snap.Cycles
		_ []PhaseRun      = snap.Cycles[1].Phases
		_ *Failure        = snap.Cycles[2].Failure
		_ []Finding       = snap.Cycles[2].Failure.Findings
		_ []AuditRound    = snap.Cycles[2].Failure.Rounds
		_ Trend           = snap.Trend
		_ []TrendPoint    = snap.Trend.Points
		_ []RoundBucket   = snap.Trend.RoundHistogram
		_ FingerprintStat = snap.Fingerprints[0]
	)
	for _, st := range []string{StateRunning, StatePass, StateWarn, StateFail, StateHalted, StateIncomplete} {
		if st == "" {
			t.Fatal("empty state constant")
		}
	}
	if snap.Cycles[0].State != StateRunning || snap.Cycles[2].State != StateFail {
		t.Fatalf("states: %s %s", snap.Cycles[0].State, snap.Cycles[2].State)
	}

	// Artifacts.
	list, err := ListArtifacts(root, 3)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListArtifacts: %v %d", err, len(list))
	}
	var _ ArtifactInfo = list[0]
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

	// Server: New, Options, Handler, Run, Serve, ListenAndServe, DefaultAddr.
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
