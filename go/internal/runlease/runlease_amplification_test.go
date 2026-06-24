package runlease_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestWriteStampsHeartbeatFromNowInUTC(t *testing.T) {
	runDir := t.TempDir()
	now := time.Date(2026, 6, 12, 23, 45, 6, 789, time.FixedZone("TST", 8*60*60))

	if err := runlease.Write(runDir, runlease.Lease{
		RunID:       "run-299",
		OwnerPID:    123,
		HeartbeatAt: "caller-supplied-heartbeat-must-not-survive",
	}, now); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	got, ok, err := runlease.Read(runDir)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Read returned ok=false after successful Write")
	}

	wantHeartbeat := now.UTC().Format(time.RFC3339Nano)
	if got.HeartbeatAt != wantHeartbeat {
		t.Fatalf("HeartbeatAt = %q, want %q", got.HeartbeatAt, wantHeartbeat)
	}
	if got.RunID != "run-299" || got.OwnerPID != 123 {
		t.Fatalf("Read lease = %+v, want RunID and OwnerPID preserved", got)
	}
}

func TestFreshUsesDefaultTTLWhenTTLIsNonPositive(t *testing.T) {
	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-runlease.DefaultTTL + time.Nanosecond).Format(time.RFC3339Nano)
	lease := runlease.Lease{HeartbeatAt: heartbeat}

	if !runlease.Fresh(lease, now, 0) {
		t.Fatalf("Fresh with ttl=0 should use DefaultTTL and accept heartbeat just inside the window")
	}
	if !runlease.Fresh(lease, now, -time.Second) {
		t.Fatalf("Fresh with negative ttl should use DefaultTTL and accept heartbeat just inside the window")
	}

	expired := runlease.Lease{HeartbeatAt: now.Add(-runlease.DefaultTTL - time.Nanosecond).Format(time.RFC3339Nano)}
	if runlease.Fresh(expired, now, 0) {
		t.Fatalf("Fresh with ttl=0 accepted heartbeat just outside DefaultTTL")
	}

	if runlease.Fresh(runlease.Lease{HeartbeatAt: "not-rfc3339"}, now, runlease.DefaultTTL) {
		t.Fatalf("Fresh accepted an unparsable heartbeat")
	}
}
