package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func amplifyEngine(t *testing.T, ws string, usage cyclestate.TokenUsage, source tokenusage.Source) (*Engine, string) {
	t.Helper()
	prof := writeProfile(t, ws, "eng-tokens-amp", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{
		Runner:    fr.runner(),
		LookupEnv: mapLookup(nil),
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Usage: usage, Source: source}, nil
		},
	})
	return eng, prof
}

func readRecords(t *testing.T, ws string) []llmCallRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(ws, "llm-calls.ndjson"))
	if err != nil {
		t.Fatalf("read llm-calls.ndjson: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	recs := make([]llmCallRecord, 0, len(lines))
	for i, line := range lines {
		var rec llmCallRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d not valid JSON: %v (%q)", i, err, line)
		}
		recs = append(recs, rec)
	}
	return recs
}

func TestEngineLaunch_NilResolver_RecordsUnavailableAttempt(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-tokens-amp", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}

	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil)})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "do the thing",
		Workspace: ws, ArtifactPath: artifact, Agent: "scout",
	})
	if err != nil {
		t.Fatalf("Launch err with nil resolver: %v", err)
	}
	if resp.ExitCode != ExitOK {
		t.Fatalf("resp.ExitCode = %d, want %d", resp.ExitCode, ExitOK)
	}
	if resp.Tokens != (core.TokenUsage{}) {
		t.Fatalf("resp.Tokens = %+v, want zero when usage enrichment is unavailable", resp.Tokens)
	}
	recs := readRecords(t, ws)
	if len(recs) != 1 {
		t.Fatalf("nil resolver must still emit one attempt record, got %d", len(recs))
	}
	rec := recs[0]
	if rec.UsageStatus != "unavailable" || rec.Source != "none" || rec.ExitCode != ExitOK {
		t.Fatalf("nil-resolver attempt = %+v", rec)
	}
	if rec.FillPct != tokenusage.FillPctUnmeasured {
		t.Fatalf("nil-resolver fill_pct = %v, want unavailable sentinel %v", rec.FillPct, tokenusage.FillPctUnmeasured)
	}
	if rec.CallID == "" || rec.StartedAt == "" || rec.EndedAt == "" || rec.TimingScope != "bridge_dispatch" {
		t.Fatalf("nil-resolver attempt lacks lifecycle identity: %+v", rec)
	}
}

// Attempt is left at its zero value, unlike the append test's explicit 1/2,
// to catch an off-by-one that would write 0.
func TestEngineLaunch_ZeroAttempt_DefaultsToOne(t *testing.T) {
	ws := t.TempDir()
	eng, prof := amplifyEngine(t, ws, cyclestate.TokenUsage{Input: 7}, tokenusage.SourceTranscript)

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "artifact.md"), Agent: "scout",
	}); err != nil {
		t.Fatalf("Launch err: %v", err)
	}

	recs := readRecords(t, ws)
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	if recs[0].Attempt != 1 {
		t.Fatalf("record.Attempt = %d, want 1 (zero must default to attempt 1)", recs[0].Attempt)
	}
}

// The append test alone starts from a fresh temp dir, so a truncate-on-open
// regression would still pass it; this seeds a pre-existing record to catch
// that.
func TestEngineLaunch_AppendPreservesExistingRecords(t *testing.T) {
	ws := t.TempDir()
	eng, prof := amplifyEngine(t, ws, cyclestate.TokenUsage{Input: 5}, tokenusage.SourceEventsResult)

	sentinel := `{"ts":"PRIOR","agent":"prior-agent","attempt":9}`
	if err := os.WriteFile(filepath.Join(ws, "llm-calls.ndjson"), []byte(sentinel+"\n"), 0o644); err != nil {
		t.Fatalf("seed llm-calls.ndjson: %v", err)
	}

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "artifact.md"),
		Agent: "builder", Attempt: 1,
	}); err != nil {
		t.Fatalf("Launch err: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(ws, "llm-calls.ndjson"))
	if err != nil {
		t.Fatalf("read llm-calls.ndjson: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines (sentinel preserved + 1 appended), got %d; content=%q", len(lines), raw)
	}
	if !strings.Contains(lines[0], "PRIOR") {
		t.Fatalf("pre-existing sentinel record was clobbered; line[0]=%q", lines[0])
	}
	var appended llmCallRecord
	if err := json.Unmarshal([]byte(lines[1]), &appended); err != nil {
		t.Fatalf("appended line not valid JSON: %v (%q)", err, lines[1])
	}
	if appended.Agent != "builder" || appended.Attempt != 1 {
		t.Fatalf("appended record = %+v, want agent=builder attempt=1", appended)
	}
}

func TestEngineLaunch_ZeroUsageStillRecords(t *testing.T) {
	ws := t.TempDir()
	eng, prof := amplifyEngine(t, ws, cyclestate.TokenUsage{}, tokenusage.SourceTranscript)

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "artifact.md"),
		Agent: "auditor", Attempt: 1,
	}); err != nil {
		t.Fatalf("Launch err: %v", err)
	}

	recs := readRecords(t, ws)
	if len(recs) != 1 {
		t.Fatalf("zero-usage success must still emit exactly 1 record, got %d", len(recs))
	}
	if recs[0].Source != string(tokenusage.SourceTranscript) {
		t.Fatalf("record.Source = %q, want %q even at zero usage", recs[0].Source, tokenusage.SourceTranscript)
	}
	if recs[0].Tokens != (cyclestate.TokenUsage{}) {
		t.Fatalf("record.Tokens = %+v, want zero", recs[0].Tokens)
	}
	if recs[0].Agent != "auditor" {
		t.Fatalf("record.Agent = %q, want auditor", recs[0].Agent)
	}
}

func TestEngineLaunch_RecordSchemaConformance(t *testing.T) {
	ws := t.TempDir()
	usage := cyclestate.TokenUsage{Input: 1234, Output: 567, CacheRead: 89, CacheWrite: 42}
	eng, prof := amplifyEngine(t, ws, usage, tokenusage.SourceTranscript)

	if _, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "artifact.md"),
		Agent: "scout", Attempt: 1,
	}); err != nil {
		t.Fatalf("Launch err: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(ws, "llm-calls.ndjson"))
	if err != nil {
		t.Fatalf("read llm-calls.ndjson: %v", err)
	}
	line := strings.TrimRight(string(raw), "\n")

	// Typed decode: omitempty on a zero-ish field still decodes to the zero
	// value asserted below.
	var rec llmCallRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("record not valid JSON: %v (%q)", err, line)
	}
	if rec.Agent != "scout" {
		t.Errorf("agent = %q, want scout", rec.Agent)
	}
	if rec.Phase != rec.Agent {
		t.Errorf("phase = %q, want it to mirror agent %q", rec.Phase, rec.Agent)
	}
	if rec.CLI != "claude-p" {
		t.Errorf("cli = %q, want claude-p", rec.CLI)
	}
	if rec.Model == "" {
		t.Errorf("model is empty, want the resolved model (e.g. auto)")
	}
	if rec.Attempt != 1 {
		t.Errorf("attempt = %d, want 1", rec.Attempt)
	}
	if rec.Source != string(tokenusage.SourceTranscript) {
		t.Errorf("source = %q, want %q", rec.Source, tokenusage.SourceTranscript)
	}
	if rec.ExitCode != ExitOK {
		t.Errorf("exit_code = %d, want %d (success launch)", rec.ExitCode, ExitOK)
	}
	if rec.DurationMS < 0 {
		t.Errorf("duration_ms = %d, want >= 0", rec.DurationMS)
	}
	if rec.Tokens != usage {
		t.Errorf("tokens = %+v, want %+v (all four nested fields must round-trip)", rec.Tokens, usage)
	}
	if rec.TS == "" {
		t.Fatalf("ts is empty")
	}
	if _, perr := time.Parse(time.RFC3339, rec.TS); perr != nil {
		t.Errorf("ts %q is not RFC3339: %v", rec.TS, perr)
	}
	if rec.SchemaVersion != 2 || rec.CallID == "" || rec.StartedAt == "" || rec.EndedAt == "" || rec.TimingScope != "bridge_dispatch" {
		t.Errorf("canonical lifecycle fields missing: %+v", rec)
	}
	if rec.RequestedModel != "auto" || rec.DispatchedModel != "haiku" || rec.DispatchSource != "argv" || rec.UsageStatus != "measured" {
		t.Errorf("model/measurement provenance = %+v", rec)
	}

	// Physical key presence: these top-level keys cannot be dropped by
	// omitempty on a success record.
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &top); err != nil {
		t.Fatalf("record not a JSON object: %v", err)
	}
	for _, k := range []string{"schema_version", "call_id", "ts", "started_at", "ended_at", "timing_scope", "agent", "phase", "cli", "model", "requested_model", "dispatched_model", "dispatch_source", "usage_status", "source", "tokens"} {
		if _, ok := top[k]; !ok {
			t.Errorf("record is missing required top-level key %q; keys=%v", k, keysOf(top))
		}
	}
	var toks map[string]json.RawMessage
	if err := json.Unmarshal(top["tokens"], &toks); err != nil {
		t.Fatalf("tokens is not a nested object: %v", err)
	}
	for _, k := range []string{"input", "output", "cache_read", "cache_write"} {
		if _, ok := toks[k]; !ok {
			t.Errorf("tokens object missing nested key %q; keys=%v", k, keysOf(toks))
		}
	}
	for _, absent := range []string{"first_output_ms", "first_output_source"} {
		if _, ok := top[absent]; ok {
			t.Errorf("record fabricated unsupported %q measurement", absent)
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
