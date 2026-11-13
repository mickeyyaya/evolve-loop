package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

var leaseTestNow = time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)

func TestTakeResumeLease_AFreeWorkspaceIsLeasedToThisProcess(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	if err := TakeResumeLease(ws, leaseTestNow); err != nil {
		t.Fatalf("TakeResumeLease on a free workspace: %v", err)
	}
	got, ok, err := runlease.Read(ws)
	if err != nil || !ok || got.OwnerPID != os.Getpid() || !runlease.Fresh(got, leaseTestNow, 0) {
		t.Fatalf("lease after take = (%+v, %v, %v), want a fresh lease owned by pid %d", got, ok, err, os.Getpid())
	}
}

func TestTakeResumeLease_AFreshLeaseRefusesWithAnErrorThatNamesTheHolder(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := runlease.Write(ws, runlease.Lease{RunID: "lane-a-run", OwnerPID: 4242}, leaseTestNow.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	err := TakeResumeLease(ws, leaseTestNow)

	var held *ResumeHeldError
	if !errors.As(err, &held) || held.Holder.RunID != "lane-a-run" || held.Holder.OwnerPID != 4242 || held.Workspace != ws {
		t.Fatalf("TakeResumeLease under a fresh lease = %v, want a *ResumeHeldError naming run lane-a-run pid 4242", err)
	}
	if !strings.Contains(err.Error(), "lane-a-run") || !strings.Contains(err.Error(), "4242") {
		t.Errorf("refusal text %q must name the holder run and pid", err)
	}
	if after, _, _ := runlease.Read(ws); after.OwnerPID != 4242 {
		t.Errorf("a refused take overwrote the holder lease: %+v", after)
	}
}

func TestTakeResumeLease_AnExpiredLeaseIsTakenOver(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := runlease.Write(ws, runlease.Lease{RunID: "dead-run", OwnerPID: 4242}, leaseTestNow.Add(-2*runlease.DefaultTTL)); err != nil {
		t.Fatal(err)
	}

	if err := TakeResumeLease(ws, leaseTestNow); err != nil {
		t.Fatalf("TakeResumeLease over an expired lease: %v", err)
	}
	if after, _, _ := runlease.Read(ws); after.OwnerPID != os.Getpid() {
		t.Errorf("lease after takeover = %+v, want pid %d", after, os.Getpid())
	}
}

func TestTakeResumeLease_AFreshUnreadableLeaseRefusesTheResumeWithTheCause(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, runlease.FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := TakeResumeLease(ws, time.Now())

	var held *ResumeHeldError
	if !errors.As(err, &held) || held.Cause == nil || !strings.Contains(err.Error(), "unreadable lease") {
		t.Fatalf("TakeResumeLease over a fresh corrupt lease = %v, want a *ResumeHeldError that carries the parse cause", err)
	}
}

func TestTakeResumeLease_AnAgedUnreadableLeaseIsTakenOver(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	path := filepath.Join(ws, runlease.FileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := leaseTestNow.Add(-2 * runlease.DefaultTTL)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	if err := TakeResumeLease(ws, leaseTestNow); err != nil {
		t.Fatalf("TakeResumeLease over an aged corrupt lease: %v", err)
	}
	if after, ok, err := runlease.Read(ws); err != nil || !ok || after.OwnerPID != os.Getpid() {
		t.Errorf("lease after takeover = (%+v, %v, %v), want pid %d", after, ok, err, os.Getpid())
	}
}

func TestReleaseResumeLease_RemovesOnlyTheAdmissionLeaseOfThisProcess(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := TakeResumeLease(ws, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseResumeLease(ws); err != nil {
		t.Fatalf("ReleaseResumeLease: %v", err)
	}
	if _, ok, _ := runlease.Read(ws); ok {
		t.Error("the admission lease of this process must be removed")
	}

	other := t.TempDir()
	if err := runlease.Write(other, runlease.Lease{RunID: "cycle-run", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseResumeLease(other); err != nil {
		t.Fatalf("ReleaseResumeLease on a run lease: %v", err)
	}
	if got, ok, _ := runlease.Read(other); !ok || got.RunID != "cycle-run" {
		t.Errorf("ReleaseResumeLease removed a lease it does not own: (%+v, %v)", got, ok)
	}

	corrupt := t.TempDir()
	if err := os.WriteFile(filepath.Join(corrupt, runlease.FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseResumeLease(corrupt); err == nil {
		t.Error("ReleaseResumeLease must report an unreadable lease")
	}
}

func TestTakeResumeLease_AnUnwritableWorkspaceRefusesTheResume(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing")

	if err := TakeResumeLease(missing, leaseTestNow); err == nil {
		t.Fatal("TakeResumeLease must report a lease it cannot write")
	}
}
