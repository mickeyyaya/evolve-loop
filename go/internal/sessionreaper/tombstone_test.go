package sessionreaper

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

func makeRunIn(t *testing.T, evolveDir, name, session string) (string, string) {
	t.Helper()
	runDir := filepath.Join(evolveDir, "runs", name)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sessionrecord.Append(sessionrecord.PathIn(runDir), sessionrecord.Record{Session: session}); err != nil {
		t.Fatal(err)
	}
	return evolveDir, runDir
}

type countingKiller struct{ calls []string }

func (k *countingKiller) kill(_ context.Context, session string) error {
	k.calls = append(k.calls, session)
	return nil
}

func TestReapOrphans_SecondSweepSkipsReapedRuns(t *testing.T) {
	evolveDir, _ := makeRun(t, "done", "evolve-bridge-done")

	first := &countingKiller{}
	if _, err := ReapOrphans(context.Background(), evolveDir, Options{Kill: first.kill}); err != nil {
		t.Fatal(err)
	}
	if len(first.calls) != 1 {
		t.Fatalf("first sweep should kill the one stale session, killed=%v", first.calls)
	}

	if _, runDirNew := makeRunIn(t, evolveDir, "later", "evolve-bridge-later"); runDirNew == "" {
		t.Fatal("fixture")
	}

	second := &countingKiller{}
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{Kill: second.kill})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range second.calls {
		if s == "evolve-bridge-done" {
			t.Errorf("fully-reaped run was re-swept on the second sweep (unbounded boot re-reap class): killed=%v", second.calls)
		}
	}
	if len(second.calls) != 1 || second.calls[0] != "evolve-bridge-later" {
		t.Errorf("new run appearing between sweeps must still be reaped exactly once, killed=%v report=%+v", second.calls, rep)
	}
}

func TestReapOrphans_PartialFailureNotTombstoned(t *testing.T) {
	evolveDir, _ := makeRun(t, "good", "evolve-bridge-good")
	if _, d := makeRunIn(t, evolveDir, "bad", "evolve-bridge-bad"); d == "" {
		t.Fatal("fixture")
	}

	failBad := func(_ context.Context, session string) error {
		if session == "evolve-bridge-bad" {
			return context.DeadlineExceeded
		}
		return nil
	}
	if _, err := ReapOrphans(context.Background(), evolveDir, Options{Kill: failBad}); err != nil {
		t.Fatal(err)
	}

	second := &countingKiller{}
	if _, err := ReapOrphans(context.Background(), evolveDir, Options{Kill: second.kill}); err != nil {
		t.Fatal(err)
	}
	retriedBad := false
	for _, s := range second.calls {
		switch s {
		case "evolve-bridge-bad":
			retriedBad = true
		case "evolve-bridge-good":
			t.Errorf("fully-successful run was re-swept after a SIBLING run's failure: killed=%v", second.calls)
		}
	}
	if !retriedBad {
		t.Errorf("partially-failed run was tombstoned — killer errors must leave the registry unmarked for retry, killed=%v", second.calls)
	}
}
