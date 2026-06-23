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

	"github.com/mickeyyaya/evolveloop/go/internal/quotareset"
)

// builtInHours mirrors quotareset's internal fallback (5.4167h ≈ 5h25min). It is
// recomputed here from the same literal so the assertion is exact, not magic.
const builtInHours = 5.4167

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
		{"empty-falls-through", "", "default", "", time.Time{}},
		{"whitespace-only-trimmed-falls-through", "   \t ", "default", "", time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange: no workspace (skip source 2), DefaultHours 0 (built-in) so a
			// fall-through deterministically lands on Source 3 = "default".
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
		{"unparseable-garbage", "garbage no time here", "default", 0, 0, 0},
		{"out-of-range-99:99am", "resets 99:99am", "default", 0, 0, 0},
		{"missing-ampm", "resets 8:30", "default", 0, 0, 0},
		{"empty-hint-file", "", "default", 0, 0, 0},
		{"truncated-past-32-drops-time", "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx8:30pm", "default", 0, 0, 0},
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
	// workspace="" means source 2 is never consulted — result must come from source 3.
	r, _ := quotareset.Compute("", quotareset.Options{Now: fixedClock()})
	if r.Source != "default" {
		t.Errorf("Source = %q, want default (source 2 skipped when workspace empty)", r.Source)
	}
}

// --- Source 3: DefaultHours / HoursFn ----------------------------------------

func TestCompute_Source3_DefaultHours(t *testing.T) {
	cases := []struct {
		name      string
		hours     float64
		wantHours float64 // expected effective hours added to now
	}{
		{"zero-uses-builtin", 0, builtInHours},
		{"positive-used", 3, 3},
		{"negative-ignored-uses-builtin", -5, builtInHours},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := quotareset.Compute("", quotareset.Options{DefaultHours: tc.hours, Now: fixedClock()})
			want := refNow().Add(time.Duration(tc.wantHours * float64(time.Hour)))
			if r.Source != "default" {
				t.Fatalf("Source = %q, want default", r.Source)
			}
			if !r.WakeAt.Equal(want) {
				t.Errorf("WakeAt = %v, want %v (hours=%v)", r.WakeAt, want, tc.wantHours)
			}
		})
	}
}

func TestCompute_Source3_HoursFnOverridesDefaultHours(t *testing.T) {
	cases := []struct {
		name      string
		hoursFn   func() float64
		defHours  float64
		wantHours float64
	}{
		{"hoursfn-beats-defaulthours", func() float64 { return 2 }, 10, 2},
		{"hoursfn-zero-wakes-now", func() float64 { return 0 }, 10, 0},
		{"hoursfn-negative-wakes-in-past", func() float64 { return -1 }, 10, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := quotareset.Compute("", quotareset.Options{HoursFn: tc.hoursFn, DefaultHours: tc.defHours, Now: fixedClock()})
			want := refNow().Add(time.Duration(tc.wantHours * float64(time.Hour)))
			if !r.WakeAt.Equal(want) {
				t.Errorf("WakeAt = %v, want %v", r.WakeAt, want)
			}
			if tc.wantHours < 0 && !r.WakeAt.Before(refNow()) {
				t.Errorf("negative HoursFn must wake in the past, got %v (now %v)", r.WakeAt, refNow())
			}
		})
	}
}

// --- Result projection -------------------------------------------------------

// TestCompute_OptionsEnvSeamNotCalled locks the env-agnostic invariant from the
// consumer side: the legacy Options.Env DI seam is dead, and Compute must never
// invoke it. A future edit that silently wired env back in through this seam
// would flip `called` and fail loudly. (Options.Env is a func parameter, not the
// system environment — control stays entirely on the public API.)
func TestCompute_OptionsEnvSeamNotCalled(t *testing.T) {
	called := false
	r, _ := quotareset.Compute("", quotareset.Options{
		Now: fixedClock(),
		Env: func(string) string { called = true; return "leaked" },
	})
	if called {
		t.Error("Compute invoked the dead Options.Env seam — env-agnostic invariant violated")
	}
	if r.Source != "default" {
		t.Errorf("Source = %q, want default", r.Source)
	}
}

func TestResult_Format(t *testing.T) {
	r := quotareset.Result{ISO: "2026-06-21T09:00:00Z", Source: "operator-override"}
	got := r.Format()
	want := "2026-06-21T09:00:00Z\nsource=operator-override\n"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}
