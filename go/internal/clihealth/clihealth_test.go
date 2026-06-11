package clihealth

// RED contract for the CLI health bench store (cycle-283 forensics): codex hit
// its quota wall, the bridge classified rate_limit on every dispatch, and
// NOTHING remembered it — every codex-routed phase re-burned a 5-15min boot
// before falling back, all night. The store is the memory: a tiny standalone
// .evolve/cli-health.json keyed by CLI FAMILY with time-based expiry.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

var t0 = time.Date(2026, 6, 11, 0, 30, 0, 0, time.UTC)

func TestStoreBenchRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := NewStore(root, fixedNow(t0))
	e := Entry{Family: "codex", Reason: "rate_limit", BenchedAt: t0,
		BenchedUntil: t0.Add(30 * time.Minute), Evidence: "You've hit your usage limit", Strikes: 1}
	if err := s.Bench(e); err != nil {
		t.Fatalf("Bench: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if g, ok := got["codex"]; !ok || g.Reason != "rate_limit" || !g.BenchedUntil.Equal(e.BenchedUntil) || g.Strikes != 1 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	// The file must land under .evolve/ so the gitignore ladder covers it.
	if _, err := os.Stat(filepath.Join(root, ".evolve", "cli-health.json")); err != nil {
		t.Errorf("store file not at .evolve/cli-health.json: %v", err)
	}
}

func TestStoreActiveLazyExpiry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := NewStore(root, fixedNow(t0))
	_ = s.Bench(Entry{Family: "codex", Reason: "rate_limit", BenchedAt: t0.Add(-2 * time.Hour),
		BenchedUntil: t0.Add(-1 * time.Hour)}) // expired
	_ = s.Bench(Entry{Family: "agy", Reason: "rate_limit", BenchedAt: t0,
		BenchedUntil: t0.Add(time.Hour)}) // active

	active := s.Active()
	if _, ok := active["codex"]; ok {
		t.Error("expired bench reported active")
	}
	if _, ok := active["agy"]; !ok {
		t.Error("active bench missing from Active()")
	}
	expired := s.Expired()
	if _, ok := expired["codex"]; !ok {
		t.Error("expired bench missing from Expired() (canary candidates)")
	}
	if _, ok := expired["agy"]; ok {
		t.Error("active bench reported expired")
	}
}

func TestStoreCorruptFileDegradesToEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := filepath.Join(root, ".evolve", "cli-health.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(root, fixedNow(t0))
	got, err := s.Load()
	if err != nil {
		t.Fatalf("corrupt store must degrade to empty, not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("corrupt store yielded entries: %+v", got)
	}
	// And a Bench after corruption must recover the file.
	if err := s.Bench(Entry{Family: "codex", Reason: "rate_limit", BenchedAt: t0, BenchedUntil: t0.Add(time.Minute)}); err != nil {
		t.Fatalf("Bench after corruption: %v", err)
	}
}

func TestStoreClear(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := NewStore(root, fixedNow(t0))
	_ = s.Bench(Entry{Family: "codex", Reason: "rate_limit", BenchedAt: t0, BenchedUntil: t0.Add(time.Hour)})
	if err := s.Clear("codex"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if got, _ := s.Load(); len(got) != 0 {
		t.Errorf("Clear left entries: %+v", got)
	}
	// Clearing an absent family is a no-op, not an error.
	if err := s.Clear("agy"); err != nil {
		t.Errorf("Clear absent family errored: %v", err)
	}
}

func TestCooldownForStrikesDoublesAndCaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		strikes int
		want    time.Duration
	}{
		{0, 30 * time.Minute},
		{1, 30 * time.Minute},
		{2, time.Hour},
		{3, 2 * time.Hour},
		{4, 4 * time.Hour},
		{9, 4 * time.Hour}, // capped
	}
	for _, c := range cases {
		if got := CooldownForStrikes(c.strikes); got != c.want {
			t.Errorf("CooldownForStrikes(%d)=%v, want %v", c.strikes, got, c.want)
		}
	}
}

// TestParseResetHintBasics — slice-1 baseline (slice 2 hardens). The verbatim
// cycle-283 wall text must parse to the printed clock time (+margin).
func TestParseResetHintBasics(t *testing.T) {
	t.Parallel()
	// now = 00:30 local; "try again at 6:11 AM" → today 06:11 + 2min margin.
	now := time.Date(2026, 6, 11, 0, 30, 0, 0, time.Local)
	pane := "■ You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), " +
		"visit https://chatgpt.com/codex/settings/usage to purchase more credits or try again at 6:11 AM."
	got, ok := ParseResetHint(pane, now)
	if !ok {
		t.Fatal("cycle-283 wall text must parse")
	}
	want := time.Date(2026, 6, 11, 6, 13, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v (printed time +2min margin)", got, want)
	}

	if _, ok := ParseResetHint("no hint here at all", now); ok {
		t.Error("garbage must not parse")
	}
}
