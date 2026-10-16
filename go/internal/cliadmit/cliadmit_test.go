package cliadmit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setSlotDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := slotsPathFn
	slotsPathFn = func(cli string) string {
		return filepath.Join(dir, "cli-"+cli+".slots")
	}
	t.Cleanup(func() { slotsPathFn = prev })
	return dir
}

func writeFixtureHolders(t *testing.T, path string, hs []holder) {
	t.Helper()
	data, err := json.Marshal(hs)
	if err != nil {
		t.Fatalf("writeFixtureHolders: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("writeFixtureHolders mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writeFixtureHolders write: %v", err)
	}
}

func TestAcquire_Unbounded(t *testing.T) {
	setSlotDir(t)
	ctx := context.Background()

	for _, max := range []int{0, -1, -99} {
		release, err := Acquire(ctx, "testcli", max, DefaultTTL)
		if err != nil {
			t.Fatalf("max=%d: unexpected error: %v", max, err)
		}
		release()
	}
}

func TestAcquire_MaxOne(t *testing.T) {
	dir := setSlotDir(t)
	ctx := context.Background()
	path := filepath.Join(dir, "cli-testcli.slots")

	release, err := Acquire(ctx, "testcli", 1, DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read slots: %v", err)
	}
	var got []holder
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 holder, got %d", len(got))
	}
	if got[0].PID != os.Getpid() {
		t.Errorf("holder PID = %d, want %d", got[0].PID, os.Getpid())
	}
}

func TestAcquire_StaleHolderPruned(t *testing.T) {
	dir := setSlotDir(t)
	ctx := context.Background()
	path := filepath.Join(dir, "cli-testcli.slots")

	ttl := 100 * time.Millisecond
	staleTime := time.Now().Add(-200 * time.Millisecond)

	writeFixtureHolders(t, path, []holder{{PID: 99999, Seq: 1, Heartbeat: staleTime}})

	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	release, err := Acquire(ctx2, "testcli", 1, ttl)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	data, _ := os.ReadFile(path)
	var got []holder
	_ = json.Unmarshal(data, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 holder after prune, got %d", len(got))
	}
	if got[0].PID != os.Getpid() {
		t.Errorf("expected our PID %d, got %d", os.Getpid(), got[0].PID)
	}
}

func TestAcquire_ReleaseFreesSlot(t *testing.T) {
	dir := setSlotDir(t)
	ctx := context.Background()
	path := filepath.Join(dir, "cli-testcli.slots")

	release, err := Acquire(ctx, "testcli", 1, DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	release()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after release: %v", err)
	}
	var got []holder
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 holders after release, got %d", len(got))
	}
}

func TestAcquire_ContextCancelUnblocks(t *testing.T) {
	dir := setSlotDir(t)
	path := filepath.Join(dir, "cli-testcli.slots")

	writeFixtureHolders(t, path, []holder{{PID: 99998, Seq: 1, Heartbeat: time.Now()}})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := Acquire(ctx, "testcli", 1, DefaultTTL)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if ctx.Err() == nil {
		t.Errorf("expected ctx to be done, error was: %v", err)
	}
}

func TestAcquire_MutualExclusion(t *testing.T) {
	setSlotDir(t)
	ctx := context.Background()

	releaseA, err := Acquire(ctx, "testcli", 1, DefaultTTL)
	if err != nil {
		t.Fatalf("Acquire A: %v", err)
	}

	doneB := make(chan error, 1)
	var releaseB func()
	go func() {
		r, err := Acquire(ctx, "testcli", 1, DefaultTTL)
		releaseB = r
		doneB <- err
	}()

	time.Sleep(200 * time.Millisecond)

	select {
	case <-doneB:
		t.Fatal("goroutine B should still be blocked, not returned yet")
	default:
	}

	releaseA()

	select {
	case err := <-doneB:
		if err != nil {
			t.Fatalf("goroutine B: %v", err)
		}
		if releaseB != nil {
			releaseB()
		}
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine B did not unblock within 2s after A released")
	}
}
