package cliadmit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquire_MkdirFailureIsReportedWithNoopRelease(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	prev := slotsPathFn
	slotsPathFn = func(string) string { return filepath.Join(blocker, "sub", "x.slots") }
	t.Cleanup(func() { slotsPathFn = prev })
	release, err := Acquire(context.Background(), "mk", 1, DefaultTTL)
	if err == nil {
		t.Fatal("want mkdir error")
	}
	release()
}

func TestAcquire_UnreadableSlotsPathIsReported(t *testing.T) {
	dir := t.TempDir()
	slots := filepath.Join(dir, "cli.slots")
	if err := os.Mkdir(slots, 0o700); err != nil {
		t.Fatal(err)
	}
	prev := slotsPathFn
	slotsPathFn = func(string) string { return slots }
	t.Cleanup(func() { slotsPathFn = prev })
	release, err := Acquire(context.Background(), "rd", 1, DefaultTTL)
	if err == nil {
		t.Fatal("want read error")
	}
	release()
}

func TestAcquire_WaitingBacksOffUntilContextEnds(t *testing.T) {
	dir := t.TempDir()
	slots := filepath.Join(dir, "cli.slots")
	prev := slotsPathFn
	slotsPathFn = func(string) string { return slots }
	t.Cleanup(func() { slotsPathFn = prev })
	hold, err := Acquire(context.Background(), "bo", 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, "bo", 1, time.Hour); err == nil {
		t.Fatal("want context error while the only slot is held")
	}
}

func TestAcquire_ReleaseSurvivesAnUnreadableSlotsPath(t *testing.T) {
	dir := t.TempDir()
	slots := filepath.Join(dir, "cli.slots")
	prev := slotsPathFn
	slotsPathFn = func(string) string { return slots }
	t.Cleanup(func() { slotsPathFn = prev })
	release, err := Acquire(context.Background(), "rl", 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(slots); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(slots, 0o700); err != nil {
		t.Fatal(err)
	}
	release()
	if info, err := os.Stat(slots); err != nil || !info.IsDir() {
		t.Fatalf("release must leave the unreadable path alone: %v", err)
	}
}
