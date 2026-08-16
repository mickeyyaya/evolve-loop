package tokenusage

// fillpct_driverwindow_test.go — RED contract for cycle-1482 task
// `context-fill-driver-window-coverage`.
//
// Today EffectiveWindow maps exactly ONE family (claude); every other driver —
// including the two whose real advertised windows are documented — returns 0 and
// degrades its fill reading to the sentinel. That is safe but blind: a codex or
// agy lane can run at 90% occupancy and the operator sees "unmeasured" forever,
// which is precisely the measurement gap the inbox item
// `context-fill-telemetry-and-cap` was filed to close ("a per-family constant,
// conservative — e.g. 200K for 1M-advertised").
//
// The contract has TWO halves and both must hold together:
//   - families with a documented advertised window get a CONSERVATIVE mapped
//     window (strictly below any advertised maximum, capped here at 400_000), and
//   - every family whose real window nobody has measured — ollama (the served
//     model, not the CLI, owns the window), an unknown CLI, whitespace, or a
//     name that merely LOOKS adjacent to a mapped family — stays at 0, because a
//     fabricated fill reading is worse than no reading.

import (
	"os"
	"path/filepath"
	"testing"
)

// conservativeWindowCeiling is the upper bound a mapped family's effective
// window may take. The whole point of the cap is that it sits BELOW the
// advertised maximum of every family in the table (400K for the codex family,
// 1M for the agy/gemini family), so a mapping at or above this ceiling is not a
// conservative effective window — it is the advertised number copied over.
const conservativeWindowCeiling = 400_000

// measuredNonClaudeFamilies are the driver identities that MUST resolve to a
// finite window after this task. Both the bare family name and the tmux variant
// are listed: production dispatches BridgeRequest.CLI values like "codex-tmux",
// so a mapping that only matches the bare name leaves the driver every fleet
// lane actually launches still unmeasured.
var measuredNonClaudeFamilies = []struct {
	family  string
	variant string
}{
	{"codex", "codex-tmux"},
	{"agy", "agy-tmux"},
}

// TestFillTelemetry_EffectiveWindowMeasuredNonClaudeFamilies is the positive
// half: the families with documented windows report a conservative, finite one,
// and the tmux variant reports the SAME window as its bare family (a variant
// that disagreed with its family would make one lane's fill readings
// incomparable with another's).
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

// TestFillTelemetry_EffectiveWindowUnsupportedFamiliesStayUnmeasured is the
// negative half, and the one that makes the positive half honest. A no-op that
// mapped "everything with a name" to 200K would pass the positive test; only
// this one rejects it. "codexx" and "agyx" are the adversarial cases: a naive
// strings.HasPrefix(driver, "codex") swallows them and starts publishing a
// fabricated window for a CLI nobody has measured.
func TestFillTelemetry_EffectiveWindowUnsupportedFamiliesStayUnmeasured(t *testing.T) {
	unmeasured := []string{
		"ollama",             // the served model owns the window, not the CLI
		"ollama-tmux",        // …and its tmux variant is no better known
		"no-such-cli-family", // unknown CLI
		"codexx",             // adjacent name — must NOT inherit the codex window
		"agyx",               // adjacent name — must NOT inherit the agy window
		"   ",                // whitespace-only identity
		"-",                  // malformed identity
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

// TestFillTelemetry_EffectiveWindowClaudeFamilyUnchanged is the regression
// guard: widening the table must not move the one family that was already
// calibrated. Empty stays claude (the pre-driver Window compatibility path that
// isClaudeDriver and the collector dispatch both depend on).
func TestFillTelemetry_EffectiveWindowClaudeFamilyUnchanged(t *testing.T) {
	for _, driver := range []string{"", "claude", "claude-tmux"} {
		if got := EffectiveWindow(driver); got != claudeEffectiveWindow {
			t.Errorf("EffectiveWindow(%q) = %d, want %d — the claude calibration must survive the table widening", driver, got, claudeEffectiveWindow)
		}
	}
}

// TestFillTelemetry_ResolverStampsFillPctForMeasuredNonClaudeDriver is the
// WIRING half: EffectiveWindow can be perfectly correct and still change
// nothing, because the only thing that puts a fill reading on a launch is
// DefaultResolver stamping it. This drives the real production resolver with a
// real events-tier fixture on a codex Window and asserts the reading comes out
// finite and arithmetically correct against the mapped window.
func TestFillTelemetry_ResolverStampsFillPctForMeasuredNonClaudeDriver(t *testing.T) {
	events := writeDriverWindowEventsFixture(t, 100_000, 20_000)

	got, err := DefaultResolver(t.TempDir())(Window{ // empty config root: no transcript tier for a non-claude driver anyway
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

// TestFillTelemetry_UnmeasuredResolveCarriesSentinelForUnknownDriver is the
// anti-fabrication wiring case, and the strongest single assertion in this file:
// the launch IS measured (the events tier recovered real usage), yet its FAMILY
// is not — so the fill must still be the sentinel. Any implementation that
// defaults an unrecognised family to some window turns this green usage into a
// green-looking, invented percentage.
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

// writeDriverWindowEventsFixture writes a one-envelope *-events.ndjson with the
// given prompt-side counters and returns its path.
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
