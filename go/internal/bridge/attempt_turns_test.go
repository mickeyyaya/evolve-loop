package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

type tokenResolver = func(tokenusage.Window) (tokenusage.Result, error)

type sidecarLine struct {
	CallID       string                `json:"call_id"`
	Turn         int                   `json:"turn"`
	Tokens       cyclestate.TokenUsage `json:"tokens"`
	PromptTokens int                   `json:"prompt_tokens"`
	Tools        []string              `json:"tools"`
	Truncated    int                   `json:"truncated"`
}

func readSidecar(t *testing.T, ws string) []sidecarLine {
	t.Helper()
	f, err := os.Open(filepath.Join(ws, "llm-turns.ndjson"))
	if err != nil {
		t.Fatalf("open the turn sidecar: %v", err)
	}
	defer func() { _ = f.Close() }()
	var out []sidecarLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ln sidecarLine
		if err := json.Unmarshal(sc.Bytes(), &ln); err != nil {
			t.Fatalf("sidecar line %q is not JSON: %v", sc.Text(), err)
		}
		out = append(out, ln)
	}
	return out
}

func turnsOf(counts ...cyclestate.TokenUsage) []tokenusage.Turn {
	turns := make([]tokenusage.Turn, len(counts))
	for i, u := range counts {
		turns[i] = tokenusage.Turn{Index: i + 1, Timestamp: "2026-10-27T09:00:00Z", Usage: u, PromptTokens: tokenusage.PromptTokens(u)}
	}
	return turns
}

func sumUsage(turns []tokenusage.Turn) cyclestate.TokenUsage {
	var sum cyclestate.TokenUsage
	for _, turn := range turns {
		sum.Input += turn.Usage.Input
		sum.Output += turn.Usage.Output
		sum.CacheRead += turn.Usage.CacheRead
		sum.CacheWrite += turn.Usage.CacheWrite
	}
	return sum
}

func launchWithResolver(t *testing.T, ws string, resolver tokenResolver) (*[]signalcenter.Event, core.BridgeResponse, error) {
	t.Helper()
	signals, events := recordingSignals()
	profile := writeProfile(t, ws, "turns", "")
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil), Signals: signals, Stderr: io.Discard, TokenResolver: resolver})
	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: profile, Model: "auto", Prompt: "work",
		Workspace: ws, ArtifactPath: artifact, Agent: "build",
	})
	signals.Flush()
	return events, resp, err
}

func turnEventsWithCode(events *[]signalcenter.Event, code signalcenter.Code) []signalcenter.Event {
	var out []signalcenter.Event
	for _, ev := range *events {
		if ev.Code == code {
			out = append(out, ev)
		}
	}
	return out
}

func resolverReturning(res tokenusage.Result) tokenResolver {
	return func(tokenusage.Window) (tokenusage.Result, error) { return res, nil }
}

func TestEngineLaunch_TurnSidecarHoldsOneLinePerTurnKeyedByTheRowCallID(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	turns := turnsOf(
		cyclestate.TokenUsage{Input: 100, Output: 20, CacheRead: 5, CacheWrite: 1},
		cyclestate.TokenUsage{Input: 50, Output: 10, CacheRead: 200},
		cyclestate.TokenUsage{Input: 30, Output: 3, CacheRead: 300},
	)
	turns[0].Tools = []string{"Read"}
	turns[1].Tools = []string{"Bash", "Edit"}

	events, _, err := launchWithResolver(t, ws, resolverReturning(tokenusage.Result{Usage: sumUsage(turns), Source: tokenusage.SourceTranscript, Turns: turns}))

	if err != nil {
		t.Fatal(err)
	}
	row := readRecords(t, ws)
	lines := readSidecar(t, ws)
	if len(row) != 1 || len(lines) != 3 {
		t.Fatalf("rows = %d, sidecar lines = %d, want 1 row and 3 turn lines", len(row), len(lines))
	}
	for i, ln := range lines {
		if ln.CallID == "" || ln.CallID != row[0].CallID || ln.Turn != i+1 || ln.Tokens != turns[i].Usage || ln.PromptTokens != turns[i].PromptTokens || len(ln.Tools) != len(turns[i].Tools) {
			t.Errorf("line %d = %+v, want the row call_id %q and turn %+v", i, ln, row[0].CallID, turns[i])
		}
	}
	if n := len(turnEventsWithCode(events, CodeTurnSumMismatch)) + len(turnEventsWithCode(events, CodeTurnAppendFailed)); n != 0 {
		t.Errorf("%d turn signals fired for a sound call, want 0: %+v", n, *events)
	}
}

func TestEngineLaunch_Agy14TurnsFillTheSidecarAndSumToTheRow(t *testing.T) {
	t.Parallel()
	hexRows, err := os.ReadFile(filepath.Join("..", "tokenusage", "testdata", "agy14", "gen-metadata-cycle1967-build.hex"))
	if err != nil {
		t.Fatal(err)
	}
	const conversation = "587d3641-24fd-443d-8989-c539a294e080"
	query := func(db, _ string) (string, error) {
		if filepath.Base(db) == "conversation_summaries.db" {
			return conversation + "\n", nil
		}
		return string(hexRows), nil
	}
	ws := t.TempDir()
	resolver := func(w tokenusage.Window) (tokenusage.Result, error) {
		w.Worktree = "/agy/worktree"
		w.Start, w.End = time.Date(2026, 10, 19, 7, 7, 30, 0, time.UTC), time.Date(2026, 10, 19, 7, 23, 34, 0, time.UTC)
		return tokenusage.AgyConversationCollector("/agy", w, query)(), nil
	}

	events, _, err := launchWithResolver(t, ws, resolver)

	if err != nil {
		t.Fatal(err)
	}
	row := readRecords(t, ws)[0]
	lines := readSidecar(t, ws)
	if row.Source != string(tokenusage.SourceAgyConversation) || len(lines) != 111 {
		t.Fatalf("row source %q, sidecar lines %d, want the agy conversation source and 111 lines", row.Source, len(lines))
	}
	var sum cyclestate.TokenUsage
	for _, ln := range lines {
		if ln.CallID != row.CallID {
			t.Fatalf("line %+v is not keyed by the row call_id %q", ln, row.CallID)
		}
		sum.Input += ln.Tokens.Input
		sum.Output += ln.Tokens.Output
		sum.CacheRead += ln.Tokens.CacheRead
		sum.CacheWrite += ln.Tokens.CacheWrite
	}
	if sum != row.Tokens {
		t.Errorf("sidecar sum = %+v, row tokens = %+v, want equal", sum, row.Tokens)
	}
	if n := len(turnEventsWithCode(events, CodeTurnSumMismatch)); n != 0 {
		t.Errorf("%d sum-mismatch warnings for a call whose turns sum to the row", n)
	}
}

func TestEngineLaunch_CallAboveTheLineCapWritesTheCapAndATruncatedCount(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	counts := make([]cyclestate.TokenUsage, llmcalls.MaxTurnLines+3)
	for i := range counts {
		counts[i] = cyclestate.TokenUsage{Input: 1, Output: 2, CacheRead: 3}
	}
	turns := turnsOf(counts...)

	events, _, err := launchWithResolver(t, ws, resolverReturning(tokenusage.Result{Usage: sumUsage(turns), Source: tokenusage.SourceAgyConversation, Turns: turns}))

	if err != nil {
		t.Fatal(err)
	}
	lines := readSidecar(t, ws)
	if len(lines) != llmcalls.MaxTurnLines+1 {
		t.Fatalf("sidecar has %d lines, want the cap %d plus a truncated line", len(lines), llmcalls.MaxTurnLines)
	}
	if last := lines[len(lines)-1]; last.Truncated != 3 || last.CallID != readRecords(t, ws)[0].CallID {
		t.Errorf("last line = %+v, want truncated 3 keyed by the row call_id", last)
	}
	if n := len(turnEventsWithCode(events, CodeTurnSumMismatch)); n != 0 {
		t.Errorf("%d sum-mismatch warnings: the check must run on every turn of the call, not only the written ones", n)
	}
}

func TestEngineLaunch_TurnSumMismatchWarnsAndLeavesTheRowAlone(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	turns := turnsOf(cyclestate.TokenUsage{Input: 10, Output: 1}, cyclestate.TokenUsage{Input: 20, Output: 2})
	rowUsage := cyclestate.TokenUsage{Input: 999, Output: 3, CacheRead: 7}

	events, resp, err := launchWithResolver(t, ws, resolverReturning(tokenusage.Result{Usage: rowUsage, Source: tokenusage.SourceTranscript, Turns: turns}))

	if err != nil {
		t.Fatal(err)
	}
	mismatch := turnEventsWithCode(events, CodeTurnSumMismatch)
	if len(mismatch) != 1 || mismatch[0].Kind != signalcenter.KindBridgeWarning || mismatch[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("mismatch events = %+v, want one bridge warning", mismatch)
	}
	row := readRecords(t, ws)[0]
	if row.Tokens != rowUsage || row.UsageStatus != "measured" || resp.Tokens != core.TokenUsage(rowUsage) {
		t.Errorf("row tokens %+v, status %q, response tokens %+v, want the resolver usage %+v kept and status measured", row.Tokens, row.UsageStatus, resp.Tokens, rowUsage)
	}
	if mismatch[0].Fields["call_id"] != row.CallID {
		t.Errorf("mismatch call_id = %q, want the row call_id %q", mismatch[0].Fields["call_id"], row.CallID)
	}
	if got := readSidecar(t, ws); len(got) != 2 {
		t.Errorf("sidecar has %d lines, want the 2 turns kept for diagnosis", len(got))
	}
}

func TestEngineLaunch_TurnWriteFailureWarnsAndNeverFailsTheLaunch(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "llm-turns.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	turns := turnsOf(cyclestate.TokenUsage{Input: 10, Output: 1}, cyclestate.TokenUsage{Input: 20, Output: 2})
	usage := sumUsage(turns)

	events, resp, err := launchWithResolver(t, ws, resolverReturning(tokenusage.Result{Usage: usage, Source: tokenusage.SourceTranscript, Turns: turns}))

	if err != nil || resp.Tokens != core.TokenUsage(usage) {
		t.Fatalf("Launch err = %v, tokens = %+v, want no error and the measured tokens", err, resp.Tokens)
	}
	failed := turnEventsWithCode(events, CodeTurnAppendFailed)
	if len(failed) != 1 || failed[0].Kind != signalcenter.KindBridgeWarning || failed[0].Reason == "" {
		t.Fatalf("append-failed events = %+v, want one bridge warning with a reason", failed)
	}
	if rows := readRecords(t, ws); len(rows) != 1 || rows[0].Tokens != usage {
		t.Errorf("rows = %+v, want the one row written whole despite the sidecar failure", rows)
	}
	if n := len(turnEventsWithCode(events, CodeTelemetryAppendFailed)); n != 0 {
		t.Errorf("%d row-append warnings, want 0: the row ledger was writable", n)
	}
}

func TestEngineLaunch_ResultWithoutTurnsWritesNoSidecarAndNoTurnSignal(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	events, _, err := launchWithResolver(t, ws, resolverReturning(tokenusage.Result{Usage: cyclestate.TokenUsage{Input: 5, Output: 1}, Source: tokenusage.SourceEventsResult}))

	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "llm-turns.ndjson")); !os.IsNotExist(statErr) {
		t.Errorf("stat llm-turns.ndjson: %v, want the file absent", statErr)
	}
	if n := len(turnEventsWithCode(events, CodeTurnSumMismatch)) + len(turnEventsWithCode(events, CodeTurnAppendFailed)); n != 0 {
		t.Errorf("%d turn signals for a result without turns, want 0", n)
	}
}
