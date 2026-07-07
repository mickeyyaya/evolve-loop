package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// engine_launch_tokens_test.go — RED contract for token-telemetry S3
// (cycle 598): Engine.Launch (engine.go:323) must populate
// core.BridgeResponse.Tokens from an injected Deps.TokenResolver and append
// one record per Launch attempt to <Workspace>/llm-calls.ndjson. See
// test-report.md "Builder Contract" for the exact seam Builder must add
// (Deps.TokenResolver, core.BridgeRequest.Attempt) — this file does not
// implement production code, only the RED tests encoding the acceptance
// criteria named in the cycle-598 inbox item
// (token-telemetry-s3-engine-launch-instrumentation).

// llmCallRecord mirrors the on-disk llm-calls.ndjson schema
// (ts, agent, phase, cli, model, attempt, tokens, source, duration_ms,
// exit_code) exactly as pinned by the S3 inbox item so a later cycle
// (S6/S7 rollups) can decode it without a migration.
type llmCallRecord struct {
	TS         string                `json:"ts"`
	Agent      string                `json:"agent"`
	Phase      string                `json:"phase"`
	CLI        string                `json:"cli"`
	Model      string                `json:"model"`
	Attempt    int                   `json:"attempt"`
	Tokens     cyclestate.TokenUsage `json:"tokens"`
	Source     string                `json:"source"`
	DurationMS int64                 `json:"duration_ms"`
	ExitCode   int                   `json:"exit_code"`
}

// TestEngineLaunch_PopulatesBridgeResponseTokens: a Launch whose
// Deps.TokenResolver resolves non-empty usage must surface that usage on
// core.BridgeResponse.Tokens (ports.go:294 — the field exists today but
// Launch never populates it, per scout finding engine.go:428).
func TestEngineLaunch_PopulatesBridgeResponseTokens(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-tokens", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}

	wantUsage := cyclestate.TokenUsage{Input: 1000, Output: 250, CacheRead: 10, CacheWrite: 5}
	eng := NewEngine(Deps{
		Runner:    fr.runner(),
		LookupEnv: mapLookup(nil),
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Usage: wantUsage, Source: tokenusage.SourceTranscript}, nil
		},
	})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "do the thing",
		Workspace: ws, ArtifactPath: artifact, Agent: "scout",
	})
	if err != nil {
		t.Fatalf("Launch err: %v", err)
	}
	if resp.Tokens != core.TokenUsage(wantUsage) {
		t.Fatalf("resp.Tokens = %+v, want %+v", resp.Tokens, wantUsage)
	}
}

// TestEngineLaunch_AppendsLLMCallRecordPerFallbackAttempt: two Launch calls
// against the same Workspace with distinct BridgeRequest.Attempt values
// (simulating a fallback retry on a different CLI, per the scout finding
// that each fallback candidate is its own Launch) must append TWO records
// to llm-calls.ndjson, not overwrite a single one — this is what makes the
// double-dispatch waste class measurable.
func TestEngineLaunch_AppendsLLMCallRecordPerFallbackAttempt(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-tokens", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}

	eng := NewEngine(Deps{
		Runner:    fr.runner(),
		LookupEnv: mapLookup(nil),
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{Usage: cyclestate.TokenUsage{Input: 10}, Source: tokenusage.SourceEventsResult}, nil
		},
	})

	base := core.BridgeRequest{
		Profile: prof, Model: "auto", Prompt: "do the thing",
		Workspace: ws, ArtifactPath: artifact, Agent: "builder",
	}
	first := base
	first.CLI = "claude-p"
	first.Attempt = 1
	if _, err := eng.Launch(context.Background(), first); err != nil {
		t.Fatalf("first Launch err: %v", err)
	}
	second := base
	second.CLI = "codex"
	second.Attempt = 2
	if _, err := eng.Launch(context.Background(), second); err != nil {
		t.Fatalf("second Launch err: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(ws, "llm-calls.ndjson"))
	if err != nil {
		t.Fatalf("read llm-calls.ndjson: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("llm-calls.ndjson has %d lines, want 2 (one per attempt); content=%q", len(lines), raw)
	}

	var recs []llmCallRecord
	for i, line := range lines {
		var rec llmCallRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d not valid JSON: %v (%q)", i, err, line)
		}
		recs = append(recs, rec)
	}

	if recs[0].Attempt != 1 || recs[0].CLI != "claude-p" {
		t.Fatalf("record 0 = %+v, want attempt=1 cli=claude-p", recs[0])
	}
	if recs[1].Attempt != 2 || recs[1].CLI != "codex" {
		t.Fatalf("record 1 = %+v, want attempt=2 cli=codex", recs[1])
	}
	for i, rec := range recs {
		if rec.Agent != "builder" {
			t.Errorf("record %d Agent = %q, want %q", i, rec.Agent, "builder")
		}
		if rec.Source != string(tokenusage.SourceEventsResult) {
			t.Errorf("record %d Source = %q, want %q", i, rec.Source, tokenusage.SourceEventsResult)
		}
		if rec.Tokens.Input != 10 {
			t.Errorf("record %d Tokens.Input = %d, want 10", i, rec.Tokens.Input)
		}
		if rec.TS == "" {
			t.Errorf("record %d TS is empty, want a timestamp", i)
		}
	}
}

// TestEngineLaunch_CollectorErrorNeverFailsLaunch: the fail-open contract
// (S3 acceptance criteria) — a TokenResolver error must WARN (to
// Deps.Stderr) and leave resp.Tokens at its zero value, but must NEVER
// turn an otherwise-successful Launch into an error.
func TestEngineLaunch_CollectorErrorNeverFailsLaunch(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-tokens", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	var stderr bytes.Buffer

	eng := NewEngine(Deps{
		Runner:    fr.runner(),
		LookupEnv: mapLookup(nil),
		Stderr:    &stderr,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
			return tokenusage.Result{}, errors.New("boom: collector unavailable")
		},
	})

	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: prof, Model: "auto", Prompt: "do the thing",
		Workspace: ws, ArtifactPath: artifact, Agent: "scout",
	})
	if err != nil {
		t.Fatalf("Launch must not fail on a resolver error, got: %v", err)
	}
	if resp.ExitCode != ExitOK {
		t.Fatalf("resp.ExitCode = %d, want %d (fail-open)", resp.ExitCode, ExitOK)
	}
	if resp.Tokens != (core.TokenUsage{}) {
		t.Fatalf("resp.Tokens = %+v, want zero value on resolver error", resp.Tokens)
	}
	if !strings.Contains(stderr.String(), "boom: collector unavailable") {
		t.Fatalf("resolver error must be WARNed to Stderr; got %q", stderr.String())
	}
}
