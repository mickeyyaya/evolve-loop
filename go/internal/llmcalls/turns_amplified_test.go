package llmcalls

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestAppendTurnsAmplified_LineCarriesTheContractJSONKeys(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	turns := []Turn{{Index: 7, TS: "2026-10-27T09:00:05Z", Tokens: cyclestate.TokenUsage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}, PromptTokens: 8, Tools: []string{"Read"}}}
	if err := AppendTurns(ws, "call-keys", turns); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(TurnsPath(ws))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"call_id", "turn", "ts", "tokens", "prompt_tokens", "tools"} {
		if _, ok := m[key]; !ok {
			t.Errorf("line %s lacks key %q", raw, key)
		}
	}
	var tok map[string]int
	if err := json.Unmarshal(m["tokens"], &tok); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"input": 1, "output": 2, "cache_read": 3, "cache_write": 4}
	for k, v := range want {
		if tok[k] != v {
			t.Errorf("tokens[%q] = %d, want %d", k, tok[k], v)
		}
	}
}

func TestAppendTurnsAmplified_CapPlusOneDropsExactlyOne(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := AppendTurns(ws, "call-cap1", numberedTurns(MaxTurnLines+1)); err != nil {
		t.Fatal(err)
	}
	got := readTurnLines(t, ws)
	if len(got) != MaxTurnLines+1 {
		t.Fatalf("lines = %d, want %d", len(got), MaxTurnLines+1)
	}
	last := got[len(got)-1]
	if last.CallID != "call-cap1" || last.Truncated != 1 {
		t.Errorf("last line = %+v, want call-cap1 with truncated 1", last)
	}
	if got[MaxTurnLines-1].Turn != MaxTurnLines {
		t.Errorf("kept turn %d, want the first %d turns", got[MaxTurnLines-1].Turn, MaxTurnLines)
	}
}

func TestAppendTurnsAmplified_SecondCallAppendsAndKeepsTheFirst(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := AppendTurns(ws, "call-a", numberedTurns(2)); err != nil {
		t.Fatal(err)
	}
	if err := AppendTurns(ws, "call-b", numberedTurns(3)); err != nil {
		t.Fatal(err)
	}
	got := readTurnLines(t, ws)
	if len(got) != 5 {
		t.Fatalf("lines = %d, want 5", len(got))
	}
	for i, id := range []string{"call-a", "call-a", "call-b", "call-b", "call-b"} {
		if got[i].CallID != id {
			t.Errorf("line %d call_id = %q, want %q", i, got[i].CallID, id)
		}
	}
}

func TestAppendTurnsAmplified_EmptySliceCreatesNoFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := AppendTurns(ws, "call-empty", []Turn{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(TurnsPath(ws)); !os.IsNotExist(err) {
		t.Errorf("stat err = %v, want not-exist", err)
	}
}

func TestAppendTurnsAmplified_UnicodeAndQuotedToolNamesSurviveRoundTrip(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	tools := []string{"mcp__tëst__\"quoted\"", "工具", "line\nbreak"}
	if err := AppendTurns(ws, "call-uni", []Turn{{Index: 1, Tools: tools}}); err != nil {
		t.Fatal(err)
	}
	got := readTurnLines(t, ws)
	if len(got) != 1 || len(got[0].Tools) != len(tools) {
		t.Fatalf("got %+v, want one line with %d tools", got, len(tools))
	}
	for i, name := range tools {
		if got[0].Tools[i] != name {
			t.Errorf("tool %d = %q, want %q", i, got[0].Tools[i], name)
		}
	}
}

func TestAppendTurnsAmplified_ConcurrentCallsNeverInterleaveLines(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	const calls, per = 8, 20
	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for c := 0; c < calls; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			errs <- AppendTurns(ws, fmt.Sprintf("call-%d", c), numberedTurns(per))
		}(c)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := readTurnLines(t, ws)
	if len(got) != calls*per {
		t.Fatalf("lines = %d, want %d", len(got), calls*per)
	}
	perCall := map[string]int{}
	for _, ln := range got {
		perCall[ln.CallID]++
	}
	for c := 0; c < calls; c++ {
		if n := perCall[fmt.Sprintf("call-%d", c)]; n != per {
			t.Errorf("call-%d has %d lines, want %d", c, n, per)
		}
	}
}
