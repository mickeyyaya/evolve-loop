package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func salvageNow() time.Time { return time.Date(2026, 8, 5, 23, 0, 0, 0, time.UTC) }

// TestSalvageProbeDiagnostics_DurableBeforeTeardown: the probe's scratch
// workspace is deleted after every refresh, but the bridge writes diagnostics
// there that must OUTLIVE it (adversarial-review HIGH: a quota wall mid-probe
// writes escalation-report.json with the repair instructions, then the
// deferred RemoveAll deleted the only copy — silent loss on a documented
// failure class). Salvage copies escalation reports (timestamped),
// launch-error files, and APPENDS llm-calls.ndjson records to the durable
// token-telemetry ledger under evolveDir/models-probe, WARNing loudly with
// the salvaged paths.
func TestSalvageProbeDiagnostics_DurableBeforeTeardown(t *testing.T) {
	t.Parallel()
	scratch, evolveDir := t.TempDir(), t.TempDir()
	writeFile := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(scratch, "escalation-report.json", `{"reason":"quota wall"}`)
	writeFile(scratch, "model-classifier-launch-error.txt", "boom stderr")
	writeFile(scratch, "llm-calls.ndjson", `{"call":2}`+"\n")
	writeFile(scratch, "model-classifier-artifact.txt", "not a diagnostic — must not be salvaged")
	// Pre-seed the durable ledger: salvage must APPEND, never clobber.
	durable := filepath.Join(evolveDir, "models-probe")
	if err := os.MkdirAll(durable, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(durable, "llm-calls.ndjson", `{"call":1}`+"\n")

	var log bytes.Buffer
	salvageProbeDiagnostics(scratch, evolveDir, salvageNow, &log)

	esc, err := os.ReadFile(filepath.Join(durable, "escalation-report-20260805T230000Z.json"))
	if err != nil || string(esc) != `{"reason":"quota wall"}` {
		t.Errorf("escalation report not salvaged timestamped: %v %q", err, esc)
	}
	if _, err := os.Stat(filepath.Join(durable, "model-classifier-launch-error.txt")); err != nil {
		t.Errorf("launch-error not salvaged: %v", err)
	}
	ledger, err := os.ReadFile(filepath.Join(durable, "llm-calls.ndjson"))
	if err != nil || string(ledger) != `{"call":1}`+"\n"+`{"call":2}`+"\n" {
		t.Errorf("ledger not appended: %v %q", err, ledger)
	}
	if _, err := os.Stat(filepath.Join(durable, "model-classifier-artifact.txt")); !os.IsNotExist(err) {
		t.Error("artifact file salvaged — only diagnostics belong in the durable dir")
	}
	for _, want := range []string{"escalation-report-20260805T230000Z.json", "launch-error"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("salvage did not WARN about %q; log:\n%s", want, log.String())
		}
	}
}

// TestSalvageProbeDiagnostics_QuietWhenNothingToSalvage: a clean probe leaves
// no diagnostics; salvage must not create the durable dir or log noise.
func TestSalvageProbeDiagnostics_QuietWhenNothingToSalvage(t *testing.T) {
	t.Parallel()
	scratch, evolveDir := t.TempDir(), t.TempDir()
	var log bytes.Buffer
	salvageProbeDiagnostics(scratch, evolveDir, salvageNow, &log)
	if _, err := os.Stat(filepath.Join(evolveDir, "models-probe")); !os.IsNotExist(err) {
		t.Error("durable dir created with nothing to salvage")
	}
	if log.Len() != 0 {
		t.Errorf("unexpected log output: %s", log.String())
	}
}
