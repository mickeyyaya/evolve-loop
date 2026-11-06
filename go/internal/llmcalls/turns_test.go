package llmcalls

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

type turnLine struct {
	CallID       string                `json:"call_id"`
	Turn         int                   `json:"turn"`
	TS           string                `json:"ts"`
	Tokens       cyclestate.TokenUsage `json:"tokens"`
	PromptTokens int                   `json:"prompt_tokens"`
	Tools        []string              `json:"tools"`
	Truncated    int                   `json:"truncated"`
}

func readTurnLines(t *testing.T, workspace string) []turnLine {
	t.Helper()
	f, err := os.Open(filepath.Join(workspace, "llm-turns.ndjson"))
	if err != nil {
		t.Fatalf("open the turn sidecar: %v", err)
	}
	defer func() { _ = f.Close() }()
	var out []turnLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ln turnLine
		if err := json.Unmarshal(sc.Bytes(), &ln); err != nil {
			t.Fatalf("sidecar line %q is not JSON: %v", sc.Text(), err)
		}
		out = append(out, ln)
	}
	return out
}

func numberedTurns(n int) []Turn {
	turns := make([]Turn, n)
	for i := range turns {
		turns[i] = Turn{Index: i + 1, TS: "2026-10-27T09:00:00Z", Tokens: cyclestate.TokenUsage{Input: i + 1, Output: 2, CacheRead: 3}, PromptTokens: i + 4}
	}
	return turns
}

func TestAppendTurns_OneLinePerTurnInOrderKeyedByCallID(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	turns := []Turn{
		{Index: 1, TS: "2026-10-27T09:00:05Z", Tokens: cyclestate.TokenUsage{Input: 100, Output: 20, CacheRead: 5, CacheWrite: 1}, PromptTokens: 106, Tools: []string{"Read"}},
		{Index: 2, TS: "2026-10-27T09:00:09Z", Tokens: cyclestate.TokenUsage{Input: 50, Output: 10, CacheRead: 200}, PromptTokens: 250, Tools: []string{"Bash", "Edit"}},
		{Index: 3, TS: "2026-10-27T09:00:30Z", Tokens: cyclestate.TokenUsage{Input: 30, Output: 3, CacheRead: 300}, PromptTokens: 330},
	}

	if err := AppendTurns(ws, "call-1", turns); err != nil {
		t.Fatal(err)
	}

	got := readTurnLines(t, ws)
	if len(got) != 3 {
		t.Fatalf("sidecar has %d lines, want 3", len(got))
	}
	for i, ln := range got {
		w := turns[i]
		if ln.CallID != "call-1" || ln.Turn != w.Index || ln.TS != w.TS || ln.Tokens != w.Tokens || ln.PromptTokens != w.PromptTokens || len(ln.Tools) != len(w.Tools) || ln.Truncated != 0 {
			t.Errorf("line %d = %+v, want call-1 and %+v", i, ln, w)
		}
	}
	if got[0].Tools[0] != "Read" || got[1].Tools[1] != "Edit" {
		t.Errorf("tool names = %v, %v, want them kept in order", got[0].Tools, got[1].Tools)
	}
}

func TestAppendTurns_CapIsWideEnoughForTheLargestKnownCall(t *testing.T) {
	t.Parallel()
	if MaxTurnLines < 111 {
		t.Errorf("MaxTurnLines = %d, want at least 111, the generations of the cycle-1967 agy build call", MaxTurnLines)
	}
}

func TestAppendTurns_CallAboveTheCapWritesTheCapAndATruncatedCount(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	if err := AppendTurns(ws, "call-big", numberedTurns(MaxTurnLines+7)); err != nil {
		t.Fatal(err)
	}

	got := readTurnLines(t, ws)
	if len(got) != MaxTurnLines+1 {
		t.Fatalf("sidecar has %d lines, want the cap %d plus one truncated line", len(got), MaxTurnLines)
	}
	for i := 0; i < MaxTurnLines; i++ {
		if got[i].Turn != i+1 || got[i].Truncated != 0 {
			t.Fatalf("line %d = %+v, want turn %d: the first turns are kept", i, got[i], i+1)
		}
	}
	last := got[MaxTurnLines]
	if last.CallID != "call-big" || last.Truncated != 7 || last.Turn != 0 {
		t.Errorf("last line = %+v, want call-big with truncated 7 and no turn", last)
	}
}

func TestAppendTurns_CallAtTheCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	if err := AppendTurns(ws, "call-edge", numberedTurns(MaxTurnLines)); err != nil {
		t.Fatal(err)
	}

	got := readTurnLines(t, ws)
	if len(got) != MaxTurnLines {
		t.Fatalf("sidecar has %d lines, want exactly the cap %d and no truncated line", len(got), MaxTurnLines)
	}
	for _, ln := range got {
		if ln.Truncated != 0 {
			t.Fatalf("line %+v carries a truncated count at the cap", ln)
		}
	}
}

func TestAppendTurns_TheCapAppliesToEachCallAlone(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := AppendTurns(ws, "call-a", numberedTurns(MaxTurnLines)); err != nil {
		t.Fatal(err)
	}

	if err := AppendTurns(ws, "call-b", numberedTurns(3)); err != nil {
		t.Fatal(err)
	}

	got := readTurnLines(t, ws)
	if len(got) != MaxTurnLines+3 || got[MaxTurnLines].CallID != "call-b" || got[len(got)-1].Turn != 3 {
		t.Errorf("sidecar has %d lines, want %d: the second call appends 3 lines keyed call-b", len(got), MaxTurnLines+3)
	}
}

func TestAppendTurns_NoTurnsCreatesNoFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()

	if err := AppendTurns(ws, "call-none", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(TurnsPath(ws)); !os.IsNotExist(err) {
		t.Errorf("stat %s: %v, want the file absent", TurnsPath(ws), err)
	}
}

func TestAppendTurns_UnwritableSidecarReturnsAnErrorNamingThePath(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.Mkdir(TurnsPath(ws), 0o755); err != nil {
		t.Fatal(err)
	}

	err := AppendTurns(ws, "call-x", numberedTurns(2))

	if err == nil {
		t.Fatal("AppendTurns returned nil for a sidecar path that is a directory, want an error")
	}
}

func TestTurnsPath_IsTheSidecarInTheWorkspace(t *testing.T) {
	t.Parallel()
	if got, want := TurnsPath("/ws"), filepath.Join("/ws", "llm-turns.ndjson"); got != want || TurnsFilename != "llm-turns.ndjson" {
		t.Errorf("TurnsPath = %q, TurnsFilename = %q, want %q", got, TurnsFilename, want)
	}
}
