package runlease

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

func TestWriteReadFresh_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Lease{RunID: "01JTEST", OwnerPID: 123}, t0); err != nil {
		t.Fatalf("Write: %v", err)
	}
	l, ok, err := Read(dir)
	if err != nil || !ok {
		t.Fatalf("Read: ok=%v err=%v", ok, err)
	}
	if l.RunID != "01JTEST" || l.OwnerPID != 123 {
		t.Errorf("round-trip mismatch: %+v", l)
	}
	if !Fresh(l, t0.Add(5*time.Minute), 0) {
		t.Error("5min-old heartbeat must be fresh within the 10min default TTL")
	}
	if Fresh(l, t0.Add(11*time.Minute), 0) {
		t.Error("11min-old heartbeat must be stale past the 10min default TTL")
	}
	if !Fresh(l, t0.Add(20*time.Minute), 30*time.Minute) {
		t.Error("explicit TTL must override the default")
	}
}

func TestRead_AbsentIsNotAnError(t *testing.T) {
	_, ok, err := Read(t.TempDir())
	if err != nil || ok {
		t.Fatalf("absent lease: want ok=false err=nil, got ok=%v err=%v", ok, err)
	}
}

func TestRead_GarbageIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(dir); err == nil {
		t.Fatal("unparsable lease must surface an error (caller decides; never silently live/dead)")
	}
}

func TestFresh_UnparsableTimestampNeverFresh(t *testing.T) {
	if Fresh(Lease{HeartbeatAt: "not-a-time"}, t0, 0) {
		t.Error("garbage heartbeat must never prove liveness")
	}
}

func TestWrite_RefreshesHeartbeat(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Lease{RunID: "r"}, t0); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, Lease{RunID: "r"}, t0.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	l, _, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !Fresh(l, t0.Add(15*time.Minute), 0) {
		t.Error("refreshed lease must be fresh relative to the new heartbeat")
	}
}
