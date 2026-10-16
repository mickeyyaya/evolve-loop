package flock

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestWithPathLock_SerializesConcurrentRMW(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "counter")
	if err := os.WriteFile(data, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}
	const n = 64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := WithPathLock(data, func() error {
				raw, err := os.ReadFile(data)
				if err != nil {
					return err
				}
				v, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
				return os.WriteFile(data, []byte(strconv.Itoa(v+1)), 0o644)
			}); err != nil {
				t.Errorf("WithPathLock: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	raw, err := os.ReadFile(data)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	if got != n {
		t.Fatalf("WithPathLock lost updates: counter=%d want %d", got, n)
	}
}

func TestWithPathLock_ReturnsPathLockError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WithPathLock(filepath.Join(blocker, "state.json"), func() error {
		t.Fatal("callback must not run when PathLock fails")
		return nil
	}); err == nil {
		t.Fatal("WithPathLock on a directory path must surface PathLock error")
	}
}

func TestPathLock_LocksSidecarNotData(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "state.json")
	release, err := PathLock(data)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := os.Stat(data + ".lock"); err != nil {
		t.Fatalf("sidecar lock file not created: %v", err)
	}
	if _, err := os.Stat(data); !os.IsNotExist(err) {
		t.Fatalf("PathLock must not create the data file, got stat err=%v", err)
	}
}
