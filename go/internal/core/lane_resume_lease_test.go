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

func TestTakeResumeLease_AFreeWorkspaceIsLeasedToThisProcess(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)

	if err := TakeResumeLease(ws, now); err != nil {
		t.Fatalf("TakeResumeLease on a free workspace: %v", err)
	}
	got, ok, err := runlease.Read(ws)
	if err != nil || !ok || got.OwnerPID != os.Getpid() || !runlease.Fresh(got, now, 0) {
		t.Fatalf("lease after take = (%+v, %v, %v), want a fresh lease owned by pid %d", got, ok, err, os.Getpid())
	}
}

func TestTakeResumeLease_AFreshLeaseRefusesWithAnErrorThatNamesTheHolder(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	holder := runlease.Lease{RunID: "lane-a-run", OwnerPID: 4242}
	if err := runlease.Write(ws, holder, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	err := TakeResumeLease(ws, now)
	var held *ResumeHeldError
	if !errors.As(err, &held) || held.Holder.RunID != "lane-a-run" || held.Holder.OwnerPID != 4242 || held.Workspace != ws {
		t.Fatalf("TakeResumeLease under a fresh lease = %v, want a *ResumeHeldError naming run lane-a-run pid 4242", err)
	}
	if !strings.Contains(err.Error(), "lane-a-run") || !strings.Contains(err.Error(), "4242") {
		t.Errorf("refusal text %q must name the holder run and pid", err)
	}
	if got, _, _ := runlease.Read(ws); got.OwnerPID != 4242 {
		t.Errorf("a refused take overwrote the holder lease: %+v", got)
	}
}

func TestTakeResumeLease_AnExpiredLeaseIsTakenOver(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	if err := runlease.Write(ws, runlease.Lease{RunID: "dead-run", OwnerPID: 4242}, now.Add(-2*runlease.DefaultTTL)); err != nil {
		t.Fatal(err)
	}

	if err := TakeResumeLease(ws, now); err != nil {
		t.Fatalf("TakeResumeLease over an expired lease: %v", err)
	}
	if got, _, _ := runlease.Read(ws); got.OwnerPID != os.Getpid() {
		t.Errorf("lease after takeover = %+v, want pid %d", got, os.Getpid())
	}
}

func TestTakeResumeLease_AnUnreadableLeaseRefusesTheResume(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, runlease.FileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := TakeResumeLease(ws, time.Now()); err == nil {
		t.Fatal("TakeResumeLease over a corrupt lease must refuse, not guess the holder")
	}
}

func TestTakeResumeLease_AnUnwritableWorkspaceRefusesTheResume(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "missing")

	if err := TakeResumeLease(ws, time.Now()); err == nil {
		t.Fatal("TakeResumeLease must report a lease it cannot write")
	}
}
