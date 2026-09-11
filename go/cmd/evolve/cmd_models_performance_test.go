package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
)

func TestModelsPerformance_JSONBuildsCanonicalAttemptIndex(t *testing.T) {
	evolveDir := t.TempDir()
	cycle := filepath.Join(evolveDir, "runs", "cycle-17")
	if err := os.MkdirAll(cycle, 0o755); err != nil {
		t.Fatal(err)
	}
	exit := 0
	d1, d2 := int64(1000), int64(3000)
	for _, rec := range []llmcalls.Record{
		{
			SchemaVersion: llmcalls.SchemaVersion, CallID: "a", TS: "2026-09-11T10:00:01Z",
			Phase: "build", Agent: "build", CLI: "codex", Model: "deep", RequestedModel: "deep",
			DispatchedModel: "gpt-5.6-sol", DispatchSource: llmcalls.DispatchArgv,
			Attempt: 1, Source: "events_result", UsageStatus: llmcalls.UsageMeasured,
			DurationMS: &d1, ExitCode: &exit,
		},
		{
			SchemaVersion: llmcalls.SchemaVersion, CallID: "b", TS: "2026-09-11T10:00:04Z",
			Phase: "build", Agent: "build", CLI: "codex", Model: "deep", RequestedModel: "deep",
			DispatchedModel: "gpt-5.6-sol", DispatchSource: llmcalls.DispatchArgv,
			Attempt: 2, Source: "none", UsageStatus: llmcalls.UsageResolverError,
			DurationMS: &d2, ExitCode: &exit,
		},
	} {
		if err := llmcalls.AppendWorkspace(cycle, rec); err != nil {
			t.Fatal(err)
		}
	}
	ledger, err := os.OpenFile(filepath.Join(cycle, llmcalls.Filename), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.WriteString("{malformed}\n"); err != nil {
		_ = ledger.Close()
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := runModels([]string{"performance", "--evolve-dir", evolveDir, "--json"}, strings.NewReader(""), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runModels rc=%d stderr=%s", rc, stderr.String())
	}
	var report modelPerformanceReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("performance JSON: %v\n%s", err, stdout.String())
	}
	if report.Attempts != 2 || report.MalformedSkipped != 1 || len(report.Groups) != 2 {
		t.Fatalf("performance report = %+v", report)
	}
	var measured, resolverError llmcalls.Performance
	for _, row := range report.Groups {
		switch row.MeasurementSource {
		case "events_result":
			measured = row
		case "none":
			resolverError = row
		}
	}
	if measured.CLI != "codex" || measured.Model != "gpt-5.6-sol" || measured.Attempts != 1 || measured.UsageMeasured != 1 {
		t.Fatalf("measured performance row = %+v", measured)
	}
	if resolverError.Attempts != 1 || resolverError.ResolverErrors != 1 {
		t.Fatalf("resolver-error performance row = %+v", resolverError)
	}
}

func TestModelsPerformance_HumanOutputNeutralizesTerminalControls(t *testing.T) {
	evolveDir := t.TempDir()
	cycle := filepath.Join(evolveDir, "runs", "cycle-18")
	if err := os.MkdirAll(cycle, 0o755); err != nil {
		t.Fatal(err)
	}
	exit, duration := 0, int64(100)
	rec := llmcalls.Record{
		SchemaVersion: llmcalls.SchemaVersion, CallID: "control", TS: "2026-09-11T10:00:01Z",
		Phase: "audit", CLI: "claude-tmux", Model: "deep", RequestedModel: "deep",
		DispatchedModel: "opus\x1b[31m\nforged", DispatchSource: llmcalls.DispatchArgv,
		UsageStatus: llmcalls.UsageUnavailable, Source: "none", DurationMS: &duration, ExitCode: &exit,
	}
	if err := llmcalls.AppendWorkspace(cycle, rec); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if rc := runModels([]string{"performance", "--evolve-dir", evolveDir}, strings.NewReader(""), &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	if strings.ContainsRune(stdout.String(), '\x1b') || strings.Contains(stdout.String(), "\nforged") {
		t.Fatalf("terminal control escaped into human report: %q", stdout.String())
	}
}

func TestModelsPerformance_ReadWarningCannotInjectDiagnosticLines(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), "bad\n[bridge] forged\u202e")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "runs"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if rc := runModels([]string{"performance", "--evolve-dir", evolveDir}, strings.NewReader(""), &stdout, &stderr); rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr.String())
	}
	log := stderr.String()
	if strings.Count(log, "\n") != 1 || strings.ContainsRune(log, '\u202e') {
		t.Fatalf("read warning injected a diagnostic line or bidi control: %q", log)
	}
	if !strings.Contains(log, `detail="`) || !strings.Contains(log, `bad\n[bridge] forged\u202e`) {
		t.Fatalf("read warning lacks escaped context: %q", log)
	}
}

func TestPrintModelPerformance_DistinguishesMissingAndMeasuredZeroLatency(t *testing.T) {
	zeroRate := 0.0
	rows := []llmcalls.Performance{
		{CLI: "codex", Model: "missing", Attempts: 1, LatencyUnavailable: 1},
		{CLI: "codex", Model: "zero", Attempts: 1, LatencySamples: 1, P50LatencyMS: 0, P95LatencyMS: 0,
			ThroughputSamples: 1, OutputTokensPerSecond: &zeroRate},
	}
	var out bytes.Buffer
	printModelPerformance(&out, rows, 2, 0)
	text := out.String()
	if !strings.Contains(text, "-/- (0 samples, 1 unavailable)") {
		t.Fatalf("missing latency rendered as measured zero: %q", text)
	}
	if !strings.Contains(text, "0s/0s (1 samples, 0 unavailable)") {
		t.Fatalf("measured zero latency lost: %q", text)
	}
}

func TestPrintModelPerformance_HeaderDoesNotClaimOneTimingScope(t *testing.T) {
	rows := []llmcalls.Performance{{
		CLI: "claude-tmux", Model: llmcalls.UnknownModel, TimingScope: llmcalls.TimingLegacyUnknown,
		Attempts: 1, LatencyUnavailable: 1,
	}}
	var out bytes.Buffer
	printModelPerformance(&out, rows, 1, 0)
	firstLine, _, _ := strings.Cut(out.String(), "\n")
	if strings.Contains(firstLine, "scope=") {
		t.Fatalf("mixed-scope report claimed one global timing scope: %q", firstLine)
	}
}
