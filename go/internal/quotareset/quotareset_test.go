package quotareset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var refNow = time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)

func TestCompute_OperatorOverrideISO(t *testing.T) {
	r, err := Compute("", Options{
		Env: func(name string) string {
			if name == "EVOLVE_QUOTA_RESET_AT" {
				return "2026-05-23T20:00:00+0000"
			}
			return ""
		},
		Now: func() time.Time { return refNow },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "operator-override" {
		t.Errorf("Source=%q", r.Source)
	}
	if r.ISO != "2026-05-23T20:00:00+0000" {
		t.Errorf("ISO=%q", r.ISO)
	}
}

func TestCompute_OperatorOverrideUnparseable(t *testing.T) {
	r, err := Compute("", Options{
		Env: func(name string) string {
			if name == "EVOLVE_QUOTA_RESET_AT" {
				return "not-a-timestamp"
			}
			return ""
		},
		Now: func() time.Time { return refNow },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "operator-override" {
		t.Errorf("Source=%q", r.Source)
	}
	if r.ISO != "not-a-timestamp" {
		t.Errorf("ISO=%q", r.ISO)
	}
}

func TestCompute_ParsedHint_FutureToday(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	if err := os.WriteFile(hintPath, []byte("resets 8:30pm"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 14:00 → 20:30 same day
	r, err := Compute(dir, Options{
		Env: func(_ string) string { return "" },
		Now: func() time.Time { return refNow },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "parsed" {
		t.Errorf("Source=%q want parsed", r.Source)
	}
	if !r.WakeAt.After(refNow) {
		t.Errorf("WakeAt %v not after now %v", r.WakeAt, refNow)
	}
	if r.WakeAt.Hour() != 20 || r.WakeAt.Minute() != 30 {
		t.Errorf("WakeAt time=%02d:%02d", r.WakeAt.Hour(), r.WakeAt.Minute())
	}
}

func TestCompute_ParsedHint_RollsToTomorrow(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	if err := os.WriteFile(hintPath, []byte("5:20am"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 14:00 → next 05:20 is tomorrow
	r, err := Compute(dir, Options{
		Env: func(_ string) string { return "" },
		Now: func() time.Time { return refNow },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "parsed" {
		t.Errorf("Source=%q", r.Source)
	}
	expectedDay := refNow.AddDate(0, 0, 1).Day()
	if r.WakeAt.Day() != expectedDay {
		t.Errorf("WakeAt day=%d want %d (tomorrow)", r.WakeAt.Day(), expectedDay)
	}
}

func TestCompute_ParsedHint_12am(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	if err := os.WriteFile(hintPath, []byte("12:00am"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r, err := Compute(dir, Options{Now: func() time.Time { return refNow }})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.WakeAt.Hour() != 0 {
		t.Errorf("12am should map to hour=0, got %d", r.WakeAt.Hour())
	}
}

func TestCompute_ParsedHint_12pm(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	if err := os.WriteFile(hintPath, []byte("12:00pm"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	earlyAM := time.Date(2026, 5, 23, 6, 0, 0, 0, time.UTC)
	r, err := Compute(dir, Options{Now: func() time.Time { return earlyAM }})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.WakeAt.Hour() != 12 {
		t.Errorf("12pm should map to hour=12, got %d", r.WakeAt.Hour())
	}
}

func TestCompute_HintMalformed_FallsThrough(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	if err := os.WriteFile(hintPath, []byte("garbage no time"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r, err := Compute(dir, Options{
		Now:     func() time.Time { return refNow },
		HoursFn: func() float64 { return 5.0 },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "default" {
		t.Errorf("Source=%q want default", r.Source)
	}
}

func TestCompute_DefaultFallback(t *testing.T) {
	r, err := Compute("", Options{
		Env:     func(_ string) string { return "" },
		Now:     func() time.Time { return refNow },
		HoursFn: func() float64 { return 5.4167 },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "default" {
		t.Errorf("Source=%q", r.Source)
	}
	expected := refNow.Add(time.Duration(5.4167 * float64(time.Hour)))
	if r.WakeAt.Sub(expected).Abs() > time.Second {
		t.Errorf("WakeAt=%v want ~%v", r.WakeAt, expected)
	}
}

func TestCompute_EnvOverrideHours(t *testing.T) {
	r, err := Compute("", Options{
		Env: func(name string) string {
			if name == "EVOLVE_QUOTA_RESET_HOURS" {
				return "2.5"
			}
			return ""
		},
		Now: func() time.Time { return refNow },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "default" {
		t.Errorf("Source=%q", r.Source)
	}
	expected := refNow.Add(150 * time.Minute)
	if r.WakeAt.Sub(expected).Abs() > time.Second {
		t.Errorf("WakeAt=%v want ~%v", r.WakeAt, expected)
	}
}

func TestFormat(t *testing.T) {
	r := Result{ISO: "2026-05-23T20:00:00+0000", Source: "parsed"}
	out := r.Format()
	if !strings.Contains(out, "2026-05-23T20:00:00+0000\nsource=parsed\n") {
		t.Errorf("unexpected format:\n%s", out)
	}
}

func TestParseHint_InvalidMinutes(t *testing.T) {
	if _, ok := parseHint("99:99am", refNow); ok {
		t.Errorf("99:99am should not parse")
	}
}

func TestCompute_EmptyHintFile(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	if err := os.WriteFile(hintPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r, err := Compute(dir, Options{
		Now:     func() time.Time { return refNow },
		HoursFn: func() float64 { return 5.0 },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "default" {
		t.Errorf("empty hint should fall through to default, got %q", r.Source)
	}
}
