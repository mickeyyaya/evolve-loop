package tokenusage

import (
	"os"
	"path/filepath"
	"testing"
)

// conservativeWindowCeiling is the smallest advertised maximum in the table (codex 400K; agy advertises 1M).
const conservativeWindowCeiling = 400_000

// Production launches the tmux variants, so each family is checked under both identities.
var measuredNonClaudeFamilies = []struct {
	family  string
	variant string
}{
	{"codex", "codex-tmux"},
	{"agy", "agy-tmux"},
}

func TestFillTelemetry_EffectiveWindowMeasuredNonClaudeFamilies(t *testing.T) {
	for _, f := range measuredNonClaudeFamilies {
		t.Run(f.family, func(t *testing.T) {
			got := EffectiveWindow(f.family)
			if got <= 0 {
				t.Fatalf("EffectiveWindow(%q) = %d, want a finite conservative window — this family's launches are still reported unmeasured", f.family, got)
			}
			if got > conservativeWindowCeiling {
				t.Errorf("EffectiveWindow(%q) = %d, want <= %d — an effective window must sit BELOW the advertised maximum, not at it", f.family, got, conservativeWindowCeiling)
			}
			if variant := EffectiveWindow(f.variant); variant != got {
				t.Errorf("EffectiveWindow(%q) = %d but EffectiveWindow(%q) = %d — the tmux variant is the driver production actually launches; it must share its family's window", f.variant, variant, f.family, got)
			}
			if pct := FillPct(got/2, got); pct != 50 {
				t.Errorf("half-full %s launch reads %v%%, want 50 — the mapped window is not the divisor", f.family, pct)
			}
		})
	}
}

func TestFillTelemetry_EffectiveWindowUnsupportedFamiliesStayUnmeasured(t *testing.T) {
	unmeasured := []string{
		"ollama", // the served model owns the window, not the CLI
		"ollama-tmux",
		"no-such-cli-family",
		"codexx", // adjacent name — must NOT inherit the codex window
		"agyx",   // adjacent name — must NOT inherit the agy window
		"   ",
		"-",
	}
	for _, driver := range unmeasured {
		t.Run(driver, func(t *testing.T) {
			if got := EffectiveWindow(driver); got != 0 {
				t.Errorf("EffectiveWindow(%q) = %d, want 0 — publishing a guessed window for an unmeasured family fabricates the reading", driver, got)
			}
			if got := FillPct(120_000, EffectiveWindow(driver)); got != FillPctUnmeasured {
				t.Errorf("fill for %q = %v, want FillPctUnmeasured", driver, got)
			}
		})
	}
}

func TestFillTelemetry_EffectiveWindowClaudeFamilyUnchanged(t *testing.T) {
	for _, driver := range []string{"", "claude", "claude-tmux"} {
		if got := EffectiveWindow(driver); got != claudeEffectiveWindow {
			t.Errorf("EffectiveWindow(%q) = %d, want %d — the claude calibration must survive the table widening", driver, got, claudeEffectiveWindow)
		}
	}
}

func TestFillTelemetry_ResolverStampsFillPctForMeasuredNonClaudeDriver(t *testing.T) {
	events := writeDriverWindowEventsFixture(t, 100_000, 20_000)

	got, err := DefaultResolver(t.TempDir())(Window{
		Driver:        "codex",
		EventsLogPath: events,
	})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if got.Source != SourceEventsResult {
		t.Fatalf("Source = %q, want %q — the fixture did not reach the events tier", got.Source, SourceEventsResult)
	}
	window := EffectiveWindow("codex")
	if window <= 0 {
		t.Fatalf("EffectiveWindow(\"codex\") = %d — the resolver cannot stamp a reading without a mapped window", window)
	}
	want := float64(120_000) / float64(window) * 100
	if got.FillPct != want {
		t.Errorf("resolver stamped FillPct = %v, want %v (120000 prompt-side tokens / %d window) — a measured codex launch must not read as unmeasured", got.FillPct, want, window)
	}
}

func TestFillTelemetry_UnmeasuredResolveCarriesSentinelForUnknownDriver(t *testing.T) {
	events := writeDriverWindowEventsFixture(t, 100_000, 20_000)

	got, err := DefaultResolver(t.TempDir())(Window{
		Driver:        "no-such-cli-family",
		EventsLogPath: events,
	})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if got.Source != SourceEventsResult {
		t.Fatalf("Source = %q, want %q — the usage half must still be recovered for an unmapped family", got.Source, SourceEventsResult)
	}
	if got.Usage.Input != 100_000 {
		t.Errorf("Usage.Input = %d, want 100000 — the unmapped family must lose only its FILL reading, not its usage", got.Usage.Input)
	}
	if got.FillPct != FillPctUnmeasured {
		t.Errorf("unmapped family stamped FillPct = %v, want FillPctUnmeasured — measured usage is not a licence to invent a window", got.FillPct)
	}
}

func writeDriverWindowEventsFixture(t *testing.T, in, cacheRead int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "build-events.ndjson")
	envelope := `{"kind":"result","data":{"cost_usd":0.4,"tokens":{"in":` +
		itoa(in) + `,"out":210,"cache_r":` + itoa(cacheRead) + `,"cache_c":0}}}` + "\n"
	if err := os.WriteFile(path, []byte(envelope), 0o644); err != nil {
		t.Fatalf("write events fixture: %v", err)
	}
	return path
}
