package observer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// syncSink is a goroutine-safe wrapper around bytes.Buffer for tests.
// bytes.Buffer's Write + String race when accessed from the observer
// goroutine + test goroutine concurrently; syncSink serializes them
// behind one mutex.
type syncSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

var _ io.Writer = (*syncSink)(nil)

// TestCoreAdapter_Start_ReturnsNoopCancelOnEmptyWorkspace pins the
// guard at the top of Start: pre-cycle / non-phase calls must not
// touch the filesystem and must return a non-nil cancel.
func TestCoreAdapter_Start_ReturnsNoopCancelOnEmptyWorkspace(t *testing.T) {
	t.Parallel()
	a := NewCoreAdapter()
	cancel := a.Start(context.Background(), "", core.PhaseRequest{})
	if cancel == nil {
		t.Fatal("expected non-nil cancel even for empty workspace+phase")
	}
	cancel() // must not panic
	cancel() // must be idempotent
}

// TestCoreAdapter_Start_CreatesEventsFile pins that a real Start opens
// (creates) the <workspace>/<phase>-observer-events.ndjson file so
// downstream tooling can read live events.
func TestCoreAdapter_Start_CreatesEventsFile(t *testing.T) {
	ws := t.TempDir()
	a := NewCoreAdapter()
	// Run very tight thresholds so the test doesn't wait 600s.
	t.Setenv("EVOLVE_OBSERVER_STALL_S", "1")
	t.Setenv("EVOLVE_OBSERVER_POLL_S", "1")
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: ws, Cycle: 123})
	defer cancel()

	eventsPath := filepath.Join(ws, "tdd-observer-events.ndjson")
	// Give the goroutine a moment to write the "started" event.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(eventsPath); err == nil && info.Size() > 0 {
			return // success
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("expected %s to be created with events within 2s", eventsPath)
}

// TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows is the
// cycle-122 shape regression test: workspace exists but the phase's
// stdout-log never appears (codex hung at modal). The observer must
// emit a stall_no_output INCIDENT.
func TestCoreAdapter_Start_EmitsStallEventWhenFileNeverGrows(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	sink := &syncSink{}
	a := &CoreAdapter{
		Sink: sink,
		EnvLookup: func(k string) string {
			switch k {
			case "EVOLVE_OBSERVER_STALL_S":
				return "1"
			case "EVOLVE_OBSERVER_POLL_S":
				return "1"
			}
			return ""
		},
	}
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: ws, Cycle: 123})

	// Wait deterministically until we see the stall event (or fail at deadline).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(sink.String(), `"stall_no_output"`) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()

	out := sink.String()
	if !strings.Contains(out, `"stall_no_output"`) {
		t.Fatalf("expected stall_no_output event when log never created; got:\n%s", out)
	}
	if !strings.Contains(out, `"incident"`) {
		t.Errorf("stall event should be severity=incident; got:\n%s", out)
	}
	if !strings.Contains(out, `"cycle":123`) || !strings.Contains(out, `"phase":"tdd"`) {
		t.Errorf("event should carry cycle/phase attribution; got:\n%s", out)
	}
}

// TestCoreAdapter_Start_StopsCleanlyOnCancel pins the cleanup contract:
// calling the returned cancel function makes the watcher goroutine
// exit within the bounded wait (10s) and the events file gets a
// "stopped" entry.
func TestCoreAdapter_Start_StopsCleanlyOnCancel(t *testing.T) {
	sink := &syncSink{}
	a := &CoreAdapter{Sink: sink}
	t.Setenv("EVOLVE_OBSERVER_POLL_S", "1")

	cancel := a.Start(context.Background(), "scout", core.PhaseRequest{
		Workspace: t.TempDir(), Cycle: 7,
	})
	// Wait for "started"
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(sink.String(), `"started"`) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	start := time.Now()
	cancel()
	if elapsed := time.Since(start); elapsed > 10500*time.Millisecond {
		t.Errorf("cancel took %v, want <10.5s", elapsed)
	}

	out := sink.String()
	if !strings.Contains(out, `"stopped"`) {
		t.Errorf("expected stopped event after cancel; got:\n%s", out)
	}
}

// TestCoreAdapter_Start_ResolveDurationFallsBackOnBadEnv guards the
// env-var parser: a non-numeric or non-positive value falls back to
// the default. Otherwise a typo like EVOLVE_OBSERVER_STALL_S=10m
// would silently disable stall detection.
func TestCoreAdapter_Start_ResolveDurationFallsBackOnBadEnv(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", DefaultStallS},
		{"   ", DefaultStallS},
		{"600", 600 * time.Second},
		{"-5", DefaultStallS},
		{"0", DefaultStallS},
		{"10m", DefaultStallS}, // not a bare integer → fallback
		{"abc", DefaultStallS},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			a := &CoreAdapter{EnvLookup: func(_ string) string { return tc.in }}
			got := a.resolveDuration("EVOLVE_OBSERVER_STALL_S", DefaultStallS)
			if got != tc.want {
				t.Errorf("in=%q got=%v want=%v", tc.in, got, tc.want)
			}
		})
	}
}

// TestCoreAdapter_ResolveNudgeS_ZeroMeansDisable pins the cycle-124 Task 6
// semantic split between resolveNudgeS and resolveDuration. resolveDuration
// falls back to default on "0" (a value useful only as a typo); but for
// nudging, "0" is the documented opt-out — set it explicitly to disable.
// Unset → default (default-on per cycle-124). Garbage → default (typo
// must not silently disable nudging). Negative → default (defensive).
func TestCoreAdapter_ResolveNudgeS_ZeroMeansDisable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", DefaultNudgeS},        // unset → default (300s, default-on)
		{"0", 0},                   // explicit opt-out — disable nudging
		{"300", 300 * time.Second}, // identity
		{"60", 60 * time.Second},   // custom
		{"  ", DefaultNudgeS},      // whitespace-only → Atoi returns err → default
		{"-5", DefaultNudgeS},      // negative → default (defensive)
		{"abc", DefaultNudgeS},     // garbage → default (don't silently disable on typo)
		{"5m", DefaultNudgeS},      // not bare integer → default
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			a := &CoreAdapter{EnvLookup: func(_ string) string { return tc.in }}
			got := a.resolveNudgeS(DefaultNudgeS)
			if got != tc.want {
				t.Errorf("in=%q got=%v want=%v", tc.in, got, tc.want)
			}
		})
	}
}

// TestCoreAdapter_ResolveString_FallsBackOnEmpty pins the cycle-124 Task 6
// helper for EVOLVE_OBSERVER_NUDGE_BODY (and any future env var that
// carries a free-form string). Empty → default; whitespace-only is
// considered set (callers may want to inject a space-padded body).
func TestCoreAdapter_ResolveString_FallsBackOnEmpty(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, def, want string
	}{
		{"", "default body", "default body"},
		{"custom body", "default body", "custom body"},
		{"   ", "default body", "   "}, // whitespace IS a value; only empty falls back
	}
	for _, tc := range cases {
		t.Run(tc.in+"|"+tc.def, func(t *testing.T) {
			a := &CoreAdapter{EnvLookup: func(_ string) string { return tc.in }}
			got := a.resolveString("EVOLVE_OBSERVER_NUDGE_BODY", tc.def)
			if got != tc.want {
				t.Errorf("in=%q def=%q got=%q want=%q", tc.in, tc.def, got, tc.want)
			}
		})
	}
}

// TestCoreAdapter_ResolveNudgeS_Bounds covers integer-parser edge cases
// the cycle-124 helper must handle without surprises. Most paths share
// strconv.Atoi semantics — but the "0 means disable" sentinel + "negative
// fallback to default" branches deserve their own coverage beyond the
// happy-path table in TestCoreAdapter_ResolveNudgeS_ZeroMeansDisable.
func TestCoreAdapter_ResolveNudgeS_Bounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
	}{
		// strconv.Atoi accepts leading zeros — preserves the parsed int.
		{"060", 60 * time.Second},
		// Atoi accepts leading + sign.
		{"+60", 60 * time.Second},
		// Atoi rejects leading whitespace (returns err) → default.
		{" 60", DefaultNudgeS},
		// Atoi rejects scientific notation → default.
		{"6e1", DefaultNudgeS},
		// Atoi rejects floats → default.
		{"60.5", DefaultNudgeS},
		// Hexadecimal → Atoi rejects → default.
		{"0x3c", DefaultNudgeS},
		// Very large positive integer — Atoi parses to int (32-bit safe
		// on common platforms; 86400 = 1 day).
		{"86400", 86400 * time.Second},
		// MaxInt32 — still parses; this is the practical upper bound.
		{"2147483647", time.Duration(2147483647) * time.Second},
		// "-0" → Atoi parses as 0 (not negative) → n=0 → returns 0 (disable).
		// This is the same path as plain "0", documented as the explicit
		// opt-out sentinel for nudging.
		{"-0", 0},
		// Trailing whitespace → Atoi error → default.
		{"60 ", DefaultNudgeS},
		// Tab-separated number → Atoi error → default.
		{"60\t", DefaultNudgeS},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			a := &CoreAdapter{EnvLookup: func(_ string) string { return tc.in }}
			got := a.resolveNudgeS(DefaultNudgeS)
			if got != tc.want {
				t.Errorf("in=%q got=%v want=%v", tc.in, got, tc.want)
			}
		})
	}
}

// TestCoreAdapter_ResolveString_UnicodeAndSpecial covers the resolveString
// helper across non-ASCII bodies and special chars the operator might
// configure for EVOLVE_OBSERVER_NUDGE_BODY in localized deployments.
func TestCoreAdapter_ResolveString_UnicodeAndSpecial(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, def, want string
	}{
		{"続けて要約してください", "default", "続けて要約してください"},   // Japanese
		{"계속하거나 마무리하세요", "default", "계속하거나 마무리하세요"}, // Korean
		{"是否还在工作？", "default", "是否还在工作？"},           // Chinese
		{"continue\\n", "default", "continue\\n"},   // backslash literal (operator-controlled escape)
		{"\t\t", "default", "\t\t"},                 // tabs preserved
		{"line1\nline2", "default", "line1\nline2"}, // literal newline (Atoi never sees this)
		{"🚀", "default", "🚀"},                       // emoji
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			a := &CoreAdapter{EnvLookup: func(_ string) string { return tc.in }}
			got := a.resolveString("EVOLVE_OBSERVER_NUDGE_BODY", tc.def)
			if got != tc.want {
				t.Errorf("in=%q got=%q want=%q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCoreAdapter_ResolveDuration_NovelBounds covers parser edges NOT
// already exercised by TestCoreAdapter_Start_ResolveDurationFallsBackOnBadEnv
// above (which handles "", whitespace, "600", "-5", "0", "10m", "abc").
// The novel surface here: leading `+` sign, leading-zero base-10 (NOT
// octal — Atoi is base-10 strict), and MaxInt32 as a sanity upper bound.
// time.Duration is int64 so `MaxInt32 * Second` (2.1e9 * 1e9 = 2.1e18) is
// safely below int64 max (~9.2e18). Cycle-124 test-review LOW: this test
// was trimmed from 8 cases to 3 — the dropped 5 cases overlapped the
// existing fallback table.
func TestCoreAdapter_ResolveDuration_NovelBounds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"+60", 60 * time.Second},                               // Atoi accepts leading +
		{"060", 60 * time.Second},                               // leading zero — Atoi treats as base-10, NOT octal
		{"2147483647", time.Duration(2147483647) * time.Second}, // MaxInt32
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			a := &CoreAdapter{EnvLookup: func(_ string) string { return tc.in }}
			got := a.resolveDuration("EVOLVE_OBSERVER_STALL_S", DefaultStallS)
			if got != tc.want {
				t.Errorf("in=%q got=%v want=%v", tc.in, got, tc.want)
			}
		})
	}
}

// TestCoreAdapter_ResolveString_EnvLookupNilFallsBackToOSGetenv pins the
// dependency-injection invariant: when EnvLookup is nil, the helper
// must fall back to os.Getenv — NOT panic on the nil func. This is the
// production path; the test sets the env var so the assertion can run
// without polluting os.Environ across tests.
func TestCoreAdapter_ResolveString_EnvLookupNilFallsBackToOSGetenv(t *testing.T) {
	t.Setenv("EVOLVE_OBSERVER_TEST_KEY", "from-os")
	a := &CoreAdapter{} // EnvLookup nil
	got := a.resolveString("EVOLVE_OBSERVER_TEST_KEY", "default")
	if got != "from-os" {
		t.Fatalf("nil EnvLookup must defer to os.Getenv; got %q", got)
	}
}

// TestCoreAdapter_ResolveNudgeS_EnvLookupNilFallsBackToOSGetenv mirrors
// the above for the nudge resolver.
func TestCoreAdapter_ResolveNudgeS_EnvLookupNilFallsBackToOSGetenv(t *testing.T) {
	t.Setenv("EVOLVE_OBSERVER_NUDGE_S", "120")
	a := &CoreAdapter{} // EnvLookup nil
	got := a.resolveNudgeS(DefaultNudgeS)
	if got != 120*time.Second {
		t.Fatalf("nil EnvLookup must defer to os.Getenv; got %v want 120s", got)
	}
}

// TestCoreAdapter_Start_ConcurrentSamePhase_IsIsolatedSafe pins
// thread-safety: two phases starting in parallel (unusual but possible
// under multi-execute) get isolated sinks + cancels.
func TestCoreAdapter_Start_ConcurrentSamePhase_IsIsolatedSafe(t *testing.T) {
	a := NewCoreAdapter()
	t.Setenv("EVOLVE_OBSERVER_POLL_S", "1")
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws := t.TempDir()
			cancel := a.Start(context.Background(), "scout", core.PhaseRequest{Workspace: ws, Cycle: 1})
			defer cancel()
			// Brief work simulation.
			time.Sleep(50 * time.Millisecond)
		}()
	}
	wg.Wait()
	// Test passes if no race detected by -race + no panic.
}

// TestCoreAdapter_Start_DegradesToNoopWhenEventsFileUnopenable covers the
// os.OpenFile error branch in Start: when Sink is nil and the events file
// cannot be created, the adapter must degrade to a no-op cancel (never block
// the phase, per ADR-0030) rather than panic. We force the failure by pointing
// Workspace at a path that is a regular FILE, so the events path's parent is
// not a directory and OpenFile fails.
func TestCoreAdapter_Start_DegradesToNoopWhenEventsFileUnopenable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	wsFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(wsFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &CoreAdapter{} // Sink nil → adapter tries to open the events file
	cancel := a.Start(context.Background(), "tdd", core.PhaseRequest{Workspace: wsFile, Cycle: 1})
	if cancel == nil {
		t.Fatal("expected non-nil (no-op) cancel when events file cannot be opened")
	}
	cancel() // must not panic
	cancel() // idempotent

	// No events file should have been created under the bogus workspace path.
	if _, err := os.Stat(filepath.Join(wsFile, "tdd-observer-events.ndjson")); err == nil {
		t.Error("events file unexpectedly created under a non-directory workspace")
	}
}

// helper to consume NDJSON if any future test needs it.
func decodeEvents(t *testing.T, body []byte) []Event {
	t.Helper()
	var out []Event
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if line == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		out = append(out, e)
	}
	return out
}

// Ensure decodeEvents stays referenced (it's a useful helper future
// tests will lean on; this prevents 'unused' lint failure).
var _ = decodeEvents
