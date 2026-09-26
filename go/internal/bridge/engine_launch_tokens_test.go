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

type llmCallRecord struct {
	SchemaVersion   int                   `json:"schema_version"`
	CallID          string                `json:"call_id"`
	TS              string                `json:"ts"`
	StartedAt       string                `json:"started_at"`
	EndedAt         string                `json:"ended_at"`
	TimingScope     string                `json:"timing_scope"`
	Agent           string                `json:"agent"`
	Phase           string                `json:"phase"`
	CLI             string                `json:"cli"`
	Model           string                `json:"model"`
	RequestedModel  string                `json:"requested_model"`
	DispatchedModel string                `json:"dispatched_model"`
	DispatchSource  string                `json:"dispatch_source"`
	Attempt         int                   `json:"attempt"`
	Tokens          cyclestate.TokenUsage `json:"tokens"`
	Source          string                `json:"source"`
	UsageStatus     string                `json:"usage_status"`
	DurationMS      int64                 `json:"duration_ms"`
	ExitCode        int                   `json:"exit_code"`
	CauseCode       string                `json:"cause_code"`
	FillPct         float64               `json:"fill_pct"`
}

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

func TestEngineLaunch_CollectorErrorNeverFailsLaunch(t *testing.T) {
	ws := t.TempDir()
	prof := writeProfile(t, ws, "eng-tokens", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	var stderr bytes.Buffer

	eng := NewEngine(Deps{
		Runner:    fr.runner(),
		LookupEnv: mapLookup(nil),
		Stderr:    &stderr, Signals: sinkDeps(&stderr),
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
	recs := readRecords(t, ws)
	if len(recs) != 1 || recs[0].UsageStatus != "resolver_error" || recs[0].Source != "none" {
		t.Fatalf("resolver error must retain the attempt with unavailable usage: %+v", recs)
	}
	if recs[0].FillPct != tokenusage.FillPctUnmeasured {
		t.Fatalf("resolver-error fill_pct = %v, want unavailable sentinel %v", recs[0].FillPct, tokenusage.FillPctUnmeasured)
	}
}
