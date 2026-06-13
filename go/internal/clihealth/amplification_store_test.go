package clihealth

import (
	"testing"
	"time"
)

func TestStoreBenchOverwritesSameFamilyAndKeepsOtherFamilies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := NewStore(root, fixedNow(t0))

	if err := s.Bench(Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    t0,
		BenchedUntil: t0.Add(30 * time.Minute),
		Evidence:     "first",
		Strikes:      1,
	}); err != nil {
		t.Fatalf("initial codex Bench: %v", err)
	}
	if err := s.Bench(Entry{
		Family:       "agy",
		Reason:       "rate_limit",
		BenchedAt:    t0,
		BenchedUntil: t0.Add(time.Hour),
		Evidence:     "separate family",
		Strikes:      1,
	}); err != nil {
		t.Fatalf("agy Bench: %v", err)
	}
	if err := s.Bench(Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    t0.Add(time.Minute),
		BenchedUntil: t0.Add(2 * time.Hour),
		Evidence:     "replacement",
		Strikes:      2,
	}); err != nil {
		t.Fatalf("replacement codex Bench: %v", err)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("stored family count = %d, want 2: %+v", len(got), got)
	}
	if got["codex"].Evidence != "replacement" || got["codex"].Strikes != 2 {
		t.Fatalf("codex entry was not replaced by latest bench: %+v", got["codex"])
	}
	if got["agy"].Evidence != "separate family" {
		t.Fatalf("agy entry was not preserved across codex replacement: %+v", got["agy"])
	}
}

func TestStoreClearOnlyRemovesRequestedFamily(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := NewStore(root, fixedNow(t0))

	for _, family := range []string{"codex", "agy"} {
		if err := s.Bench(Entry{
			Family:       family,
			Reason:       "rate_limit",
			BenchedAt:    t0,
			BenchedUntil: t0.Add(time.Hour),
			Evidence:     family,
			Strikes:      1,
		}); err != nil {
			t.Fatalf("Bench(%s): %v", family, err)
		}
	}

	if err := s.Clear("codex"); err != nil {
		t.Fatalf("Clear(codex): %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load after Clear: %v", err)
	}
	if _, ok := got["codex"]; ok {
		t.Fatalf("Clear(codex) left codex entry: %+v", got)
	}
	if got["agy"].Evidence != "agy" {
		t.Fatalf("Clear(codex) removed or changed unrelated family: %+v", got)
	}
}

func TestStoreActiveAndExpiredReturnIndependentSnapshots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := NewStore(root, fixedNow(t0))

	if err := s.Bench(Entry{
		Family:       "codex",
		Reason:       "rate_limit",
		BenchedAt:    t0,
		BenchedUntil: t0.Add(time.Hour),
		Evidence:     "active",
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench(active): %v", err)
	}
	if err := s.Bench(Entry{
		Family:       "agy",
		Reason:       "rate_limit",
		BenchedAt:    t0.Add(-2 * time.Hour),
		BenchedUntil: t0.Add(-time.Hour),
		Evidence:     "expired",
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench(expired): %v", err)
	}

	active := s.Active()
	expired := s.Expired()
	delete(active, "codex")
	active["injected"] = Entry{Family: "injected", BenchedUntil: t0.Add(time.Hour)}
	delete(expired, "agy")
	expired["stale-injected"] = Entry{Family: "stale-injected", BenchedUntil: t0.Add(-time.Hour)}

	reloadedActive := s.Active()
	if _, ok := reloadedActive["codex"]; !ok {
		t.Fatalf("mutating Active snapshot changed store-backed active set: %+v", reloadedActive)
	}
	if _, ok := reloadedActive["injected"]; ok {
		t.Fatalf("mutating Active snapshot injected a store entry: %+v", reloadedActive)
	}

	reloadedExpired := s.Expired()
	if _, ok := reloadedExpired["agy"]; !ok {
		t.Fatalf("mutating Expired snapshot changed store-backed expired set: %+v", reloadedExpired)
	}
	if _, ok := reloadedExpired["stale-injected"]; ok {
		t.Fatalf("mutating Expired snapshot injected a store entry: %+v", reloadedExpired)
	}
}
