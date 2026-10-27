package tokenusage

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFillTelemetry_ResolverStampsFillPctForMeasuredNonClaudeDriver(t *testing.T) {
	events := writeDriverWindowEventsFixture(t, 100_000, 20_000)

	got, err := DefaultResolver(t.TempDir())(Window{
		Driver:        "codex",
		ContextWindow: 200_000,
		EventsLogPath: events,
	})
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if got.Source != SourceEventsResult {
		t.Fatalf("Source = %q, want %q — the fixture did not reach the events tier", got.Source, SourceEventsResult)
	}
	window := 200_000
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
		strconv.Itoa(in) + `,"out":210,"cache_r":` + strconv.Itoa(cacheRead) + `,"cache_c":0}}}` + "\n"
	if err := os.WriteFile(path, []byte(envelope), 0o644); err != nil {
		t.Fatalf("write events fixture: %v", err)
	}
	return path
}
