package clihealth

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recordingCenter() (*signalcenter.Center, func() []signalcenter.Event) {
	c := signalcenter.New()
	var mu sync.Mutex
	var got []signalcenter.Event
	c.Subscribe(func(e signalcenter.Event) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e)
	})
	return c, func() []signalcenter.Event {
		c.Flush()
		mu.Lock()
		defer mu.Unlock()
		return append([]signalcenter.Event(nil), got...)
	}
}

func authBenchStore(t *testing.T) *Store {
	t.Helper()
	now := time.Date(2026, 11, 2, 11, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), func() time.Time { return now })
	for _, e := range []Entry{
		{Family: "claude", Reason: CredentialPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
		{Family: "agy", Scope: "gemini-3-pro", Reason: CredentialPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
		{Family: "agy", Reason: QuotaExhaustedPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
		{Family: "agy", Model: "claude-opus", Reason: UsageProbePattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
		{Family: "codex", Reason: CredentialPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)},
	} {
		if err := s.Bench(e); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestClearAuthBench_ASuccessClearsEveryAuthBenchOfTheFamilyAndSignalsEach(t *testing.T) {
	t.Parallel()
	s := authBenchStore(t)
	signals, events := recordingCenter()

	cleared, err := s.ClearAuthBench("agy", SourceLaunch, signals)

	if err != nil || len(cleared) != 1 || cleared[0].Key() != "agy@gemini-3-pro" {
		t.Fatalf("cleared=%+v err=%v; want only the scoped agy auth bench", cleared, err)
	}
	benches, _ := s.Load()
	for _, key := range []string{"agy", "agy\x00claude-opus", "claude", "codex"} {
		if _, ok := benches[key]; !ok {
			t.Errorf("bench %q is gone; a REPL success clears only the auth benches of its own family", key)
		}
	}
	if _, ok := benches["agy@gemini-3-pro"]; ok {
		t.Error("the agy auth bench is still on the store")
	}
	got := events()
	if len(got) != 1 {
		t.Fatalf("events=%+v; want one %s", got, CodeAuthBenchCleared)
	}
	e := got[0]
	if e.Code != CodeAuthBenchCleared || e.Module != signalcenter.ModuleBridge || e.Severity != signalcenter.SeverityInfo ||
		e.Fields["family"] != "agy" || e.Fields["reason"] != CredentialPattern || e.Fields["source"] != string(SourceLaunch) || e.Fields["key"] != "agy@gemini-3-pro" {
		t.Fatalf("event %+v; want the cleared code with family, reason, source and key", e)
	}
	if _, owner := signalcenter.IsRegistered(CodeAuthBenchCleared); !owner {
		t.Fatalf("%s is not registered", CodeAuthBenchCleared)
	}
}

func TestClearAuthBench_QuotaBenchesStayAndNothingIsWritten(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 2, 11, 0, 0, 0, time.UTC)
	root := t.TempDir()
	s := NewStore(root, func() time.Time { return now })
	signals, events := recordingCenter()

	if cleared, err := s.ClearAuthBench("claude", SourceSmoke, signals); err != nil || len(cleared) != 0 {
		t.Fatalf("cleared=%+v err=%v on an empty store", cleared, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".evolve")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an empty store gained .evolve/ (%v); a success with no auth bench writes nothing", err)
	}
	if err := s.Bench(Entry{Family: "claude", Reason: QuotaExhaustedPattern, BenchedAt: now, BenchedUntil: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if cleared, err := s.ClearAuthBench("claude", SourceSmoke, signals); err != nil || len(cleared) != 0 {
		t.Fatalf("cleared=%+v err=%v; a REPL can boot while the quota is spent", cleared, err)
	}
	if _, benched := s.Active()["claude"]; !benched {
		t.Fatal("the quota bench is gone")
	}
	if got := events(); len(got) != 0 {
		t.Fatalf("events=%+v; nothing was cleared", got)
	}
}

func TestClearAuthBench_AWriteFailureClearsNothingAndSignalsNothing(t *testing.T) {
	t.Parallel()
	s := authBenchStore(t)
	signals, events := recordingCenter()
	dir := filepath.Dir(s.path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	cleared, err := s.ClearAuthBench("claude", SourceUsageProbe, signals)

	if err == nil || cleared != nil {
		t.Fatalf("cleared=%+v err=%v; want the write error", cleared, err)
	}
	if got := events(); len(got) != 0 {
		t.Fatalf("events=%+v; a failed clear signals nothing", got)
	}
}

func TestAuthBench_OnlyTheCredentialWallIsAuthClass(t *testing.T) {
	t.Parallel()
	if !AuthBench(CredentialPattern) {
		t.Fatalf("%q is the auth bench", CredentialPattern)
	}
	for _, reason := range []string{"rate_limit", ExhaustedPattern, QuotaExhaustedPattern, UsageProbePattern, BootTimeoutPattern, ModelUnsupportedPattern} {
		if AuthBench(reason) {
			t.Errorf("%q is not an auth bench", reason)
		}
	}
}
