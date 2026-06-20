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
		ResetAt: "2026-05-23T20:00:00+0000",
		Now:     func() time.Time { return refNow },
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
		ResetAt: "not-a-timestamp",
		Now:     func() time.Time { return refNow },
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
		DefaultHours: 2.5,
		Now:          func() time.Time { return refNow },
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

func TestCompute_OperatorOverrideValidRFC3339(t *testing.T) {
	// A well-formed RFC3339 override (colon in the zone offset) parses, so
	// WakeAt is the parsed instant — not the synthesized now() of the
	// unparseable branch.
	override := "2026-05-23T20:00:00+00:00"
	r, err := Compute("", Options{
		ResetAt: override,
		Now:     func() time.Time { return refNow },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "operator-override" {
		t.Errorf("Source=%q want operator-override", r.Source)
	}
	if r.ISO != override {
		t.Errorf("ISO=%q want %q", r.ISO, override)
	}
	want, _ := time.Parse(time.RFC3339, override)
	if !r.WakeAt.Equal(want) {
		t.Errorf("WakeAt=%v want parsed instant %v", r.WakeAt, want)
	}
}

func TestCompute_DefaultClockUsedWhenNowNil(t *testing.T) {
	// Now seam left nil → Compute falls back to time.Now(). Assert only the
	// deterministic parts (source + that WakeAt is in the future window) to
	// avoid coupling to the wall clock.
	before := time.Now()
	r, err := Compute("", Options{
		Env:     func(string) string { return "" },
		HoursFn: func() float64 { return 1.0 },
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "default" {
		t.Errorf("Source=%q want default", r.Source)
	}
	// WakeAt = now()+1h; bound generously to stay deterministic under load.
	if r.WakeAt.Before(before.Add(59*time.Minute)) || r.WakeAt.After(time.Now().Add(61*time.Minute)) {
		t.Errorf("WakeAt=%v not ~1h after invocation", r.WakeAt)
	}
}

func TestCompute_HintLongerThan32CharsTruncated(t *testing.T) {
	dir := t.TempDir()
	hintPath := filepath.Join(dir, "quota-reset-hint.txt")
	// >32 chars; the parseable "8:30pm" sits within the first 32 bytes so the
	// truncated head still parses. Padding pushes total length over the cap.
	contents := "your limit resets 8:30pm----------------------------tail"
	if len(contents) <= 32 {
		t.Fatalf("test setup: contents must exceed 32 chars, got %d", len(contents))
	}
	if err := os.WriteFile(hintPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r, err := Compute(dir, Options{Now: func() time.Time { return refNow }})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if r.Source != "parsed" {
		t.Errorf("Source=%q want parsed (8:30pm within first 32 chars)", r.Source)
	}
	if r.WakeAt.Hour() != 20 || r.WakeAt.Minute() != 30 {
		t.Errorf("WakeAt time=%02d:%02d want 20:30", r.WakeAt.Hour(), r.WakeAt.Minute())
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
