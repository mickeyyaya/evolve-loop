package tokenusage

// fillpct_test.go — RED contract for cycle-1444 task `context-fill-telemetry-record`.
//
// RED: fillpct.go does not exist yet. PromptTokens / EffectiveWindow / FillPct /
// FillPctUnmeasured / FillWarn and the Result.FillPct field are all undefined, so
// this file fails to COMPILE until Builder adds them (compile-fail = RED evidence).
//
// The contract, in one line: context fill is a DERIVED reading off the usage the
// existing scanner already recovers (prompt-side tokens ÷ the driver family's
// effective window) — never a second independent measurement path — and an
// unmeasurable reading is an explicit sentinel, never 0%, never Inf/NaN.

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// claudeWindow is the conservative per-family effective window the research
// update pins for the claude family (200K — deliberately below any advertised
// 1M, per the 2026-08-03 reliability finding embedded in the inbox item).
const claudeWindow = 200_000

// TestFillTelemetry_PctFromPromptTokensAndWindow pins the arithmetic: FillPct is
// a PERCENTAGE (0–100), not a 0–1 ratio, so a WARN threshold expressed in
// percent compares directly against it.
func TestFillTelemetry_PctFromPromptTokensAndWindow(t *testing.T) {
	cases := []struct {
		name   string
		prompt int
		window int
		want   float64
	}{
		{"quarter full", 50_000, 200_000, 25},
		{"exactly the warn default", 120_000, 200_000, 60},
		{"just past the warn default", 122_000, 200_000, 61},
		{"empty prompt is a measured zero", 0, 200_000, 0},
		{"over-full is not clamped away", 240_000, 200_000, 120},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FillPct(c.prompt, c.window)
			if math.Abs(got-c.want) > 0.001 {
				t.Errorf("FillPct(%d, %d) = %v, want %v (percentage, not ratio)", c.prompt, c.window, got, c.want)
			}
		})
	}
}

// TestFillTelemetry_ZeroWindowGuard is the load-bearing negative test: an
// unconfigured (zero or nonsense) window must yield the explicit unmeasured
// sentinel — never a divide-by-zero Inf/NaN that would poison every downstream
// comparison, and never a plain 0 that reads as "measured empty".
func TestFillTelemetry_ZeroWindowGuard(t *testing.T) {
	for _, window := range []int{0, -1, -200_000} {
		got := FillPct(120_000, window)
		if math.IsInf(got, 0) || math.IsNaN(got) {
			t.Fatalf("FillPct(120000, %d) = %v — divide-by-zero leaked Inf/NaN", window, got)
		}
		if got != FillPctUnmeasured {
			t.Errorf("FillPct(120000, %d) = %v, want FillPctUnmeasured (%v)", window, got, FillPctUnmeasured)
		}
	}
	if FillPctUnmeasured >= 0 {
		t.Errorf("FillPctUnmeasured = %v — the sentinel must be negative so it can never collide with a real 0–100%% reading", FillPctUnmeasured)
	}
}

// TestFillTelemetry_PromptTokensSumsInputSideOnly pins WHAT fills the context:
// every input-side count (fresh input + cache read + cache write) occupies the
// window; generated output does not. Summing Output in would overstate fill and
// make the WARN fire on long answers rather than on big prompts.
func TestFillTelemetry_PromptTokensSumsInputSideOnly(t *testing.T) {
	u := cyclestate.TokenUsage{Input: 1_000, Output: 9_999_999, CacheRead: 300, CacheWrite: 70}
	if got, want := PromptTokens(u), 1_370; got != want {
		t.Errorf("PromptTokens(%+v) = %d, want %d (Input+CacheRead+CacheWrite; Output must NOT count toward fill)", u, got, want)
	}
	if got := PromptTokens(cyclestate.TokenUsage{}); got != 0 {
		t.Errorf("PromptTokens(zero) = %d, want 0", got)
	}
}

// TestFillTelemetry_EffectiveWindowClaudeFamily pins the family table for the
// one family this cycle measures. Empty driver means claude for backward
// compatibility, exactly as isClaudeDriver already treats it — the window table
// must not disagree with the collector dispatch about who is a claude launch.
func TestFillTelemetry_EffectiveWindowClaudeFamily(t *testing.T) {
	for _, driver := range []string{"", "claude", "claude-tmux"} {
		if got := EffectiveWindow(driver); got != claudeWindow {
			t.Errorf("EffectiveWindow(%q) = %d, want %d", driver, got, claudeWindow)
		}
	}
}

// TestFillTelemetry_EffectiveWindowUnconfiguredFamily is the negative half: a
// family with no configured window reports 0 (unconfigured) so FillPct degrades
// to the sentinel. Guessing a window for an unknown CLI would publish a
// fabricated fill reading.
func TestFillTelemetry_EffectiveWindowUnconfiguredFamily(t *testing.T) {
	if got := EffectiveWindow("no-such-cli-family"); got != 0 {
		t.Errorf("EffectiveWindow(\"no-such-cli-family\") = %d, want 0 (never guess a window for an unknown family)", got)
	}
	if got := FillPct(120_000, EffectiveWindow("no-such-cli-family")); got != FillPctUnmeasured {
		t.Errorf("unknown family fill = %v, want FillPctUnmeasured", got)
	}
}

// TestFillTelemetry_ResolverStampsFillPct is the SINGLE-SOURCING proof: fill%
// must ride out of the production resolver on the usage it already recovered,
// not from a second scan. A claude-driver launch with no transcript falls to the
// events tier; the recovered prompt-side counts (100_000 in + 20_000 cache_r)
// are 60.0% of the 200K claude window.
func TestFillTelemetry_ResolverStampsFillPct(t *testing.T) {
	ws := t.TempDir()
	events := filepath.Join(ws, "build-events.ndjson")
	envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":100000,"out":210,"cache_r":20000,"cache_c":0}}}` + "\n"
	if err := os.WriteFile(events, []byte(envelope), 0o644); err != nil {
		t.Fatalf("write events fixture: %v", err)
	}

	got, err := DefaultResolver(t.TempDir())(Window{ // empty config root: no transcript tier
		Driver:        "claude-tmux",
		EventsLogPath: events,
	})
	if err != nil {
		t.Fatalf("resolver returned error (telemetry must be best-effort): %v", err)
	}
	if got.Source != SourceEventsResult {
		t.Fatalf("Source = %q, want %q — fixture did not reach the events tier", got.Source, SourceEventsResult)
	}
	if math.Abs(got.FillPct-60) > 0.001 {
		t.Errorf("Result.FillPct = %v, want 60 (120000 prompt-side tokens / 200000 window) — fill%% must derive from the usage this same resolve recovered", got.FillPct)
	}
}

// TestFillTelemetry_UnmeasuredResolveCarriesSentinel is the anti-false-zero
// test: when NO tier observed the launch, prompt tokens are zero — but that is
// "unmeasured", not "0% full". Stamping 0.0 here would make every uncovered
// driver look like an empty context forever.
func TestFillTelemetry_UnmeasuredResolveCarriesSentinel(t *testing.T) {
	got, err := DefaultResolver(t.TempDir())(Window{Driver: "claude-tmux"})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if got.Source != SourceNone {
		t.Fatalf("Source = %q, want %q", got.Source, SourceNone)
	}
	if got.FillPct != FillPctUnmeasured {
		t.Errorf("uncovered resolve FillPct = %v, want FillPctUnmeasured — a measured-zero and an unmeasured launch must not read alike", got.FillPct)
	}
}

// TestFillWarn_FiresOnlyStrictlyAboveThreshold pins the boundary and the message
// content. The message must name the phase, or an operator reading the WARN
// cannot tell WHICH launch is close to compaction.
func TestFillWarn_FiresOnlyStrictlyAboveThreshold(t *testing.T) {
	cases := []struct {
		name      string
		pct       float64
		threshold int
		wantWarn  bool
	}{
		{"below threshold is silent", 59, 60, false},
		{"exactly at threshold is silent", 60, 60, false},
		{"just above threshold warns", 60.1, 60, true},
		{"far above threshold warns", 91.4, 60, true},
		{"unmeasured never warns", FillPctUnmeasured, 60, false},
		{"raised threshold suppresses", 70, 80, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FillWarn("build", c.pct, c.threshold)
			if (got != "") != c.wantWarn {
				t.Fatalf("FillWarn(\"build\", %v, %d) = %q, want warn=%v", c.pct, c.threshold, got, c.wantWarn)
			}
			if c.wantWarn && !strings.Contains(got, "build") {
				t.Errorf("warn %q does not name the phase — an unattributed fill WARN is unactionable", got)
			}
		})
	}
}
