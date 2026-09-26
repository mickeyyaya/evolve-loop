package tokenusage

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const claudeWindow = 200_000

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

func TestFillTelemetry_PromptTokensSumsInputSideOnly(t *testing.T) {
	u := cyclestate.TokenUsage{Input: 1_000, Output: 9_999_999, CacheRead: 300, CacheWrite: 70}
	if got, want := PromptTokens(u), 1_370; got != want {
		t.Errorf("PromptTokens(%+v) = %d, want %d (Input+CacheRead+CacheWrite; Output must NOT count toward fill)", u, got, want)
	}
	if got := PromptTokens(cyclestate.TokenUsage{}); got != 0 {
		t.Errorf("PromptTokens(zero) = %d, want 0", got)
	}
}

func TestFillTelemetry_EffectiveWindowClaudeFamily(t *testing.T) {
	for _, driver := range []string{"", "claude", "claude-tmux"} {
		if got := EffectiveWindow(driver); got != claudeWindow {
			t.Errorf("EffectiveWindow(%q) = %d, want %d", driver, got, claudeWindow)
		}
	}
}

func TestFillTelemetry_EffectiveWindowUnconfiguredFamily(t *testing.T) {
	if got := EffectiveWindow("no-such-cli-family"); got != 0 {
		t.Errorf("EffectiveWindow(\"no-such-cli-family\") = %d, want 0 (never guess a window for an unknown family)", got)
	}
	if got := FillPct(120_000, EffectiveWindow("no-such-cli-family")); got != FillPctUnmeasured {
		t.Errorf("unknown family fill = %v, want FillPctUnmeasured", got)
	}
}

func TestFillTelemetry_ResolverStampsFillPct(t *testing.T) {
	ws := t.TempDir()
	events := filepath.Join(ws, "build-events.ndjson")
	envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":100000,"out":210,"cache_r":20000,"cache_c":0}}}` + "\n"
	if err := os.WriteFile(events, []byte(envelope), 0o644); err != nil {
		t.Fatalf("write events fixture: %v", err)
	}

	got, err := DefaultResolver(t.TempDir())(Window{
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

func TestFillTelemetry_PromptTokenOverflowIsUnmeasured(t *testing.T) {
	cases := []struct {
		name  string
		usage cyclestate.TokenUsage
	}{
		{"input+cacheRead wraps past MaxInt", cyclestate.TokenUsage{Input: math.MaxInt, CacheRead: 1}},
		{"three halves wrap", cyclestate.TokenUsage{
			Input: math.MaxInt/2 + 1, CacheRead: math.MaxInt/2 + 1, CacheWrite: 2,
		}},
		{"cacheWrite alone tips the sum over", cyclestate.TokenUsage{
			Input: math.MaxInt - 10, CacheRead: 5, CacheWrite: 100,
		}},
		{"a negative counter cannot fabricate an empty context", cyclestate.TokenUsage{
			Input: -500_000, CacheRead: 1_000,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pct := FillPct(PromptTokens(c.usage), claudeWindow)
			if pct != FillPctUnmeasured {
				t.Fatalf("FillPct(PromptTokens(%+v), %d) = %v, want FillPctUnmeasured (%v): a wrapped/negative prompt total must degrade to the sentinel, never to a fabricated percentage",
					c.usage, claudeWindow, pct, FillPctUnmeasured)
			}
			if got := FillWarn("build", pct, 60); got != "" {
				t.Errorf("FillWarn on the unmeasured sentinel = %q, want silence", got)
			}
		})
	}
}

func TestFillTelemetry_OverflowGuardKeepsHonestReadings(t *testing.T) {
	cases := []struct {
		name  string
		usage cyclestate.TokenUsage
		want  float64
	}{
		{"ordinary launch", cyclestate.TokenUsage{Input: 100_000, CacheRead: 20_000, Output: 9_999_999}, 60},
		{"empty prompt is a measured zero", cyclestate.TokenUsage{Output: 1_000}, 0},
		{"over-full is still not clamped", cyclestate.TokenUsage{Input: 200_000, CacheWrite: 40_000}, 120},
		{"large but non-wrapping total stays a real reading", cyclestate.TokenUsage{Input: 1_000_000_000}, 500_000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FillPct(PromptTokens(c.usage), claudeWindow)
			if math.Abs(got-c.want) > 0.001 {
				t.Errorf("FillPct(PromptTokens(%+v), %d) = %v, want %v — the overflow guard must not swallow honest readings", c.usage, claudeWindow, got, c.want)
			}
		})
	}
}
