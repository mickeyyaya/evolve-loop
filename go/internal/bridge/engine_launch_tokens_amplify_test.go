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

// engine_launch_tokens_amplify_test.go — adversarial amplification for
// token-telemetry S3 (cycle 598). These tests are designed black-box from the
// TDD "Builder Contract" (test-report.md) alone; the engine.go/ports.go
// implementations were NOT read. They target contract clauses the three RED
// tests in engine_launch_tokens_test.go leave unguarded:
//
//   - nil TokenResolver is the DI "off" switch: no record file at all.
//   - Attempt==0 (existing callers) must default to attempt 1, not 0.
//   - the append must not truncate a pre-existing llm-calls.ndjson.
//   - the full on-disk JSON schema S6/S7 rollups decode (phase==agent,
//     nested input/output/cache_read/cache_write, RFC3339 ts, exit_code).
//   - the record write is gated on resolver *presence*, not usage magnitude:
//     a zero-usage-but-no-error resolve still emits a record.
//
// The record schema type llmCallRecord and the harness helpers (fakeRunner,
// writeProfile, mapLookup, NewEngine, Deps, ExitOK) are reused from the
// existing package tests — this file only adds new test funcs.

// amplifyEngine builds an Engine whose fakeRunner writes a success artifact and
// whose resolver returns the given usage/source with no error. It returns the
// engine plus the resolved profile so callers can build a BridgeRequest.
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

// readRecords reads and decodes every line of <ws>/llm-calls.ndjson.
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

// TestEngineLaunch_NilResolver_NoTelemetry: the nil TokenResolver is the DI
// "off" switch (Deps.TokenResolver doc: "nil disables telemetry entirely:
// Tokens stays zero and no llm-calls.ndjson record is appended"). None of the
// three RED tests exercise this path — all inject a resolver. A regression that
// writes an empty/zero record (or panics on the nil func) when telemetry is off
// would slip past them.
func TestEngineLaunch_NilResolver_NoTelemetry(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-tokens-amp", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}

	// No TokenResolver set -> telemetry disabled.
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
		t.Fatalf("resp.Tokens = %+v, want zero when telemetry disabled", resp.Tokens)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "llm-calls.ndjson")); !os.IsNotExist(statErr) {
		t.Fatalf("llm-calls.ndjson must NOT be created when TokenResolver is nil; stat err = %v", statErr)
	}
}

// TestEngineLaunch_ZeroAttempt_DefaultsToOne: existing callers do not set
// BridgeRequest.Attempt (zero value). The contract requires the record's
// attempt to default to 1 ("Zero (unset, existing callers) is treated as
// attempt 1"). The RED append test always sets Attempt explicitly (1, 2), so
// the defaulting arithmetic (an off-by-one writing 0) is untested.
func TestEngineLaunch_ZeroAttempt_DefaultsToOne(t *testing.T) {
	ws := t.TempDir()
	eng, prof := amplifyEngine(t, ws, cyclestate.TokenUsage{Input: 7}, tokenusage.SourceTranscript)

	// Attempt left at its zero value on purpose.
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

// TestEngineLaunch_AppendPreservesExistingRecords: the write must open the file
// O_APPEND, never O_TRUNC — a Launch in cycle N must not clobber records a
// prior Launch (or prior cycle) already wrote. The RED append test starts from
// a fresh temp dir every time, so an accidental truncate-on-open would still
// leave it green (it only counts records it wrote itself).
func TestEngineLaunch_AppendPreservesExistingRecords(t *testing.T) {
	ws := t.TempDir()
	eng, prof := amplifyEngine(t, ws, cyclestate.TokenUsage{Input: 5}, tokenusage.SourceEventsResult)

	// Seed a pre-existing record line (a valid prior entry).
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

// TestEngineLaunch_ZeroUsageStillRecords: the record write is gated on the
// resolver being *present and non-erroring*, not on the usage being non-zero.
// A resolver that legitimately resolves zero usage (no error) must still append
// exactly one record — this is what separates "telemetry off" (nil resolver,
// no file) from "telemetry on, nothing spent" (a real zero-usage record). No
// RED test covers the zero-usage-success case.
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

// TestEngineLaunch_RecordSchemaConformance: pins the exact on-disk schema the
// S6/S7 rollups will decode without a migration. The RED append test only spot-
// checks agent, cli, source, tokens.input and TS!="". This asserts the full
// contract: phase mirrors agent, the four nested token fields, a parseable
// RFC3339 timestamp, exit_code carries the launch code, and every top-level
// key the contract names is physically present on disk.
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

	// (a) Typed decode: assert field VALUES (robust to omitempty on zero-ish
	//     numeric fields — a missing key decodes to the zero value we assert).
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

	// (b) Physical key presence for the contract's guaranteed-non-empty
	//     top-level fields (these cannot be dropped by omitempty on a success
	//     record) plus the nested token object's four keys.
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &top); err != nil {
		t.Fatalf("record not a JSON object: %v", err)
	}
	for _, k := range []string{"ts", "agent", "phase", "cli", "model", "source", "tokens"} {
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
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
