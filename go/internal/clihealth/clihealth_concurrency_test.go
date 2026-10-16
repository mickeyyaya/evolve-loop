package clihealth

import (
	"sync"
	"testing"
	"time"
)

func TestStore_BenchWallFirstStrike(t *testing.T) {
	s := NewStore(t.TempDir(), fixedNow(time.Unix(0, 0)))
	e, err := s.BenchWall("codex", "rate_limit", "wall text")
	if err != nil {
		t.Fatalf("BenchWall: %v", err)
	}
	if e.Strikes != 1 || e.Family != "codex" || e.Reason != "rate_limit" {
		t.Errorf("first BenchWall = %+v, want strikes=1 family=codex reason=rate_limit", e)
	}
	if len(s.Active()) != 1 {
		t.Errorf("Active() = %d, want 1 after a wall", len(s.Active()))
	}
}

func TestStore_BenchWallAccumulatesStrikesUnderConcurrency(t *testing.T) {
	s := NewStore(t.TempDir(), fixedNow(time.Unix(0, 0)))
	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.BenchWall("codex", "rate_limit", "")
		}()
	}
	wg.Wait()
	all, _ := s.Load()
	if all["codex"].Strikes != n {
		t.Errorf("strikes = %d, want %d (no lost increments under concurrent walls)", all["codex"].Strikes, n)
	}
}

func TestStore_ClearConcurrentWithBench(t *testing.T) {
	s := NewStore(t.TempDir(), fixedNow(time.Unix(0, 0)))
	if _, err := s.BenchWall("agy", "rate_limit", ""); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = s.Clear("agy") }()
	go func() { defer wg.Done(); _, _ = s.BenchWall("codex", "rate_limit", "") }()
	wg.Wait()
	all, _ := s.Load()
	if _, ok := all["codex"]; !ok {
		t.Error("codex bench lost under concurrent Clear(agy)")
	}
}
