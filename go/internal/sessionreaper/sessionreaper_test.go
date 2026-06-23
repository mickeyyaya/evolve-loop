package sessionreaper

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/runlease"
	"github.com/mickeyyaya/evolveloop/go/internal/sessionrecord"
)

func TestReapOrphans_FreshLeaseSkipped(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	evolveDir, runDir := makeRun(t, "live", "evolve-bridge-live")
	if err := runlease.Write(runDir, runlease.Lease{RunID: "live"}, now); err != nil {
		t.Fatal(err)
	}
	var killed []string
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{
		Now: func() time.Time { return now },
		Kill: func(_ context.Context, session string) error {
			killed = append(killed, session)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(killed) != 0 || rep.LiveRunsSkipped != 1 || len(rep.Orphaned) != 0 {
		t.Fatalf("fresh run was not skipped: killed=%v report=%+v", killed, rep)
	}
}

func TestReapOrphans_StaleLeaseReaped(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	evolveDir, runDir := makeRun(t, "stale", "evolve-bridge-stale")
	if err := runlease.Write(runDir, runlease.Lease{RunID: "stale"}, now.Add(-2*runlease.DefaultTTL)); err != nil {
		t.Fatal(err)
	}
	var killed []string
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{
		Now: func() time.Time { return now },
		Kill: func(_ context.Context, session string) error {
			killed = append(killed, session)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(killed, []string{"evolve-bridge-stale"}) {
		t.Fatalf("killed=%v", killed)
	}
	if len(rep.Orphaned) != 1 || rep.Orphaned[0].RunDir != runDir || rep.Orphaned[0].Report.Killed != 1 {
		t.Fatalf("report=%+v", rep)
	}
}

func TestReapOrphans_MissingRegistryIsZeroActivity(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(evolveDir, "runs", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{})
	if err != nil || rep.LiveRunsSkipped != 0 || len(rep.Orphaned) != 0 {
		t.Fatalf("missing registry: report=%+v err=%v", rep, err)
	}
}

func TestReapOrphans_AbsentLeaseIsStale(t *testing.T) {
	evolveDir, runDir := makeRun(t, "unleased", "evolve-bridge-unleased")
	var killed []string
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{
		Kill: func(_ context.Context, session string) error {
			killed = append(killed, session)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(killed, []string{"evolve-bridge-unleased"}) || len(rep.Orphaned) != 1 || rep.Orphaned[0].RunDir != runDir {
		t.Fatalf("absent lease was not reaped: killed=%v report=%+v", killed, rep)
	}
}

func TestReapOrphans_MissingRunsAndNonDirectoriesIgnored(t *testing.T) {
	evolveDir := t.TempDir()
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{})
	if err != nil || len(rep.Orphaned) != 0 {
		t.Fatalf("missing runs: report=%+v err=%v", rep, err)
	}
	if err := os.Mkdir(filepath.Join(evolveDir, "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "runs", "loose"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err = ReapOrphans(context.Background(), evolveDir, Options{})
	if err != nil || len(rep.Orphaned) != 0 {
		t.Fatalf("loose file: report=%+v err=%v", rep, err)
	}
}

func TestReapOrphans_InvalidLeaseIsStale(t *testing.T) {
	evolveDir, runDir := makeRun(t, "broken", "evolve-bridge-broken")
	if err := os.WriteFile(runlease.PathIn(runDir), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var killed []string
	rep, err := ReapOrphans(context.Background(), evolveDir, Options{
		Kill: func(_ context.Context, session string) error {
			killed = append(killed, session)
			return nil
		},
	})
	if err != nil || !reflect.DeepEqual(killed, []string{"evolve-bridge-broken"}) || len(rep.Orphaned) != 1 {
		t.Fatalf("invalid lease was not treated as stale: killed=%v report=%+v err=%v", killed, rep, err)
	}
}

func makeRun(t *testing.T, name, session string) (string, string) {
	t.Helper()
	evolveDir := t.TempDir()
	runDir := filepath.Join(evolveDir, "runs", name)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := sessionrecord.Append(sessionrecord.PathIn(runDir), sessionrecord.Record{Session: session}); err != nil {
		t.Fatal(err)
	}
	return evolveDir, runDir
}
