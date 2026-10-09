package quotareset_test

// Black-box, environment-agnostic coverage of the quota-reset wake-time
// parameters that replaced EVOLVE_QUOTA_RESET_AT / EVOLVE_QUOTA_RESET_HOURS.
// Every behavior is driven ONLY through the public API quotareset.Compute and
// the typed quotareset.Options — never via os.Getenv / t.Setenv. The clock and
// the hint file are the only inputs, both supplied as parameters, so the suite
// is fully deterministic and repeatable (F.I.R.S.T.).

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
)

// refNow is a deterministic clock: 2026-06-20 14:00:00 UTC. Hint times before
// 14:00 roll to tomorrow; times after stay today.
func refNow() time.Time { return time.Date(2026, 6, 20, 14, 0, 0, 0, time.UTC) }

// fixedClock returns refNow as an Options.Now seam.
func fixedClock() func() time.Time { return func() time.Time { return refNow() } }

// hintWorkspace writes $ws/quota-reset-hint.txt with body and returns ws.
// An empty body produces a size-0 file (the "empty hint" edge case).
func hintWorkspace(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "quota-reset-hint.txt"), []byte(body), 0o644); err != nil {
		t.Fatalf("write hint: %v", err)
	}
	return ws
}

// --- Source 1: opts.ResetAt (operator override) ------------------------------

func TestCompute_Source1_ResetAt(t *testing.T) {
	valid := "2026-06-21T09:00:00Z"
	cases := []struct {
		name       string
		resetAt    string
		wantSource string
		wantISO    string
		wantWake   time.Time // only asserted when wantSource == "operator-override"
	}{
		{"valid-rfc3339", valid, "operator-override", valid, time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)},
		{"unparseable-nonempty", "not-a-time", "operator-override", "not-a-time", refNow()},
		{"empty-falls-through", "", "unknown", "", time.Time{}},
		{"whitespace-only-trimmed-falls-through", "   \t ", "unknown", "", time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := quotareset.Compute("", quotareset.Options{ResetAt: tc.resetAt, Now: fixedClock()})
			// Act/Assert
			if err != nil {
				t.Fatalf("Compute never errors on this path, got %v", err)
			}
			if r.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q", r.Source, tc.wantSource)
			}
			if tc.wantSource == "operator-override" {
				if r.ISO != tc.wantISO {
					t.Errorf("ISO = %q, want %q", r.ISO, tc.wantISO)
				}
				if !r.WakeAt.Equal(tc.wantWake) {
					t.Errorf("WakeAt = %v, want %v", r.WakeAt, tc.wantWake)
				}
			}
		})
	}
}

// --- Precedence across the three sources -------------------------------------

func TestCompute_SourcePrecedence(t *testing.T) {
	ws := hintWorkspace(t, "resets 8:30pm") // a parseable hint, present in all rows
	override := "2026-06-21T09:00:00Z"
	cases := []struct {
		name       string
		workspace  string
		opts       quotareset.Options
		wantSource string
	}{
		{"resetAt-beats-hint", ws, quotareset.Options{ResetAt: override, Now: fixedClock()}, "operator-override"},
		{"resetAt-beats-defaulthours", "", quotareset.Options{ResetAt: override, DefaultHours: 3, Now: fixedClock()}, "operator-override"},
		{"hint-beats-defaulthours", ws, quotareset.Options{DefaultHours: 3, Now: fixedClock()}, "parsed"},
		{"defaulthours-when-alone", "", quotareset.Options{DefaultHours: 3, Now: fixedClock()}, "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := quotareset.Compute(tc.workspace, tc.opts)
			if r.Source != tc.wantSource {
				t.Errorf("Source = %q, want %q", r.Source, tc.wantSource)
			}
		})
	}
}

// --- Source 2: hint-file parsing (exercised THROUGH Compute, public API) -----

func TestCompute_Source2_HintParsing(t *testing.T) {
	cases := []struct {
		name       string
		hint       string
		wantSource string
		wantDay    int // only checked when parsed
		wantHour   int
		wantMin    int
	}{
		{"future-today-8:30pm", "resets 8:30pm", "parsed", 20, 20, 30},
		{"past-today-rolls-tomorrow-5:20am", "resets 5:20am", "parsed", 21, 5, 20},
		{"noon-12:00pm", "back at 12:00pm", "parsed", 21, 12, 0},
		{"midnight-12:00am", "back at 12:00am", "parsed", 21, 0, 0},
		{"midnight-00:00am", "back at 00:00am", "parsed", 21, 0, 0},
		{"unparseable-garbage", "garbage no time here", "unknown", 0, 0, 0},
		{"out-of-range-99:99am", "resets 99:99am", "unknown", 0, 0, 0},
		{"missing-ampm", "resets 8:30", "unknown", 0, 0, 0},
		{"empty-hint-file", "", "unknown", 0, 0, 0},
		{"truncated-past-32-drops-time", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx8:30pm", "unknown", 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := hintWorkspace(t, tc.hint)
			r, _ := quotareset.Compute(ws, quotareset.Options{Now: fixedClock()})
			if r.Source != tc.wantSource {
				t.Fatalf("Source = %q, want %q", r.Source, tc.wantSource)
			}
			if tc.wantSource == "parsed" {
				if r.WakeAt.Day() != tc.wantDay || r.WakeAt.Hour() != tc.wantHour || r.WakeAt.Minute() != tc.wantMin {
					t.Errorf("WakeAt = %v, want day=%d hour=%d min=%d", r.WakeAt, tc.wantDay, tc.wantHour, tc.wantMin)
				}
			}
		})
	}
}

func TestCompute_Source2_SkippedWhenNoWorkspace(t *testing.T) {
	r, _ := quotareset.Compute("", quotareset.Options{Now: fixedClock()})
	if r.Source != "unknown" {
		t.Errorf("Source = %q, want unknown (source 2 skipped when workspace empty)", r.Source)
	}
}

func TestCompute_Source3_DefaultHours(t *testing.T) {
	cases := []struct {
		name       string
		hours      float64
		wantHours  float64
		wantSource string
	}{
		{"zero-is-unknown-now", 0, 0, "unknown"},
		{"positive-used", 3, 3, "default"},
		{"negative-is-unknown-now", -5, 0, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := quotareset.Compute("", quotareset.Options{DefaultHours: tc.hours, Now: fixedClock()})
			want := refNow().Add(time.Duration(tc.wantHours * float64(time.Hour)))
			if r.Source != tc.wantSource {
				t.Fatalf("Source = %q, want %q", r.Source, tc.wantSource)
			}
			if !r.WakeAt.Equal(want) {
				t.Errorf("WakeAt = %v, want %v (hours=%v)", r.WakeAt, want, tc.wantHours)
			}
		})
	}
}

// --- Result projection -------------------------------------------------------

func TestResult_Format(t *testing.T) {
	r := quotareset.Result{ISO: "2026-06-21T09:00:00Z", Source: "operator-override"}
	got := r.Format()
	want := "2026-06-21T09:00:00Z\nsource=operator-override\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}
