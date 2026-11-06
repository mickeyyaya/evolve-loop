package tokenusage

import (
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

const turnWorktree = "/repo/worktrees/cycle-2175"

func turnLine(id, ts, content string, in, out, cr, cw int) string {
	return `{"type":"assistant","cwd":"` + turnWorktree + `","timestamp":"` + ts + `","message":{"id":"` + id + `","content":` + content +
		`,"usage":{"input_tokens":` + strconv.Itoa(in) + `,"output_tokens":` + strconv.Itoa(out) + `,"cache_read_input_tokens":` + strconv.Itoa(cr) + `,"cache_creation_input_tokens":` + strconv.Itoa(cw) + `}}}`
}

func threeTurnTranscriptRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "-repo-worktrees-cycle-2175")
	user := `{"type":"user","cwd":"` + turnWorktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":"start"}}`
	earlier := user + "\n" +
		turnLine("m0", "2026-07-07T09:59:00Z", `[{"type":"tool_use","name":"Glob"}]`, 999, 999, 999, 999) + "\n" +
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"thinking","thinking":"plan"}]`, 100, 5, 5, 1) + "\n" +
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","name":"Read"}]`, 100, 20, 5, 1) + "\n" +
		turnLine("m2", "2026-07-07T10:00:09Z", `[{"type":"tool_use","name":"Bash"},{"type":"text","text":"x"},{"type":"tool_use","name":"Edit"}]`, 50, 10, 200, 7) + "\n"
	later := user + "\n" +
		turnLine("m3", "2026-07-07T10:00:30Z", `[{"type":"text","text":"done"}]`, 30, 3, 300, 0) + "\n"
	writeTranscript(t, dir, "sess-b.jsonl", earlier)
	writeTranscript(t, dir, "sess-a.jsonl", later)
	return root
}

func turnWindow(t *testing.T) Window {
	t.Helper()
	return Window{Worktree: turnWorktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)}
}

func TestTurns_ClaudeTranscriptGivesOneTurnPerMessageInTimeOrder(t *testing.T) {
	t.Parallel()
	res, err := ScanConfigRoot(threeTurnTranscriptRoot(t), turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}

	want := []Turn{
		{Index: 1, Timestamp: "2026-07-07T10:00:05Z", Usage: cyclestate.TokenUsage{Input: 100, Output: 20, CacheRead: 5, CacheWrite: 1}, PromptTokens: 106, Tools: []string{"Read"}},
		{Index: 2, Timestamp: "2026-07-07T10:00:09Z", Usage: cyclestate.TokenUsage{Input: 50, Output: 10, CacheRead: 200, CacheWrite: 7}, PromptTokens: 257, Tools: []string{"Bash", "Edit"}},
		{Index: 3, Timestamp: "2026-07-07T10:00:30Z", Usage: cyclestate.TokenUsage{Input: 30, Output: 3, CacheRead: 300}, PromptTokens: 330},
	}
	if len(res.Turns) != len(want) {
		t.Fatalf("Turns = %d entries %+v, want %d: one per assistant message id inside the window, across both transcript files", len(res.Turns), res.Turns, len(want))
	}
	for i, w := range want {
		g := res.Turns[i]
		if g.Index != w.Index || g.Timestamp != w.Timestamp || g.Usage != w.Usage || g.PromptTokens != w.PromptTokens || !reflect.DeepEqual(append([]string(nil), g.Tools...), append([]string(nil), w.Tools...)) {
			t.Errorf("Turns[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestTurns_ClaudeTurnSumEqualsTheRowUsage(t *testing.T) {
	t.Parallel()
	res, err := ScanConfigRoot(threeTurnTranscriptRoot(t), turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}
	var sum cyclestate.TokenUsage
	for _, turn := range res.Turns {
		sum.Input += turn.Usage.Input
		sum.Output += turn.Usage.Output
		sum.CacheRead += turn.Usage.CacheRead
		sum.CacheWrite += turn.Usage.CacheWrite
	}
	if len(res.Turns) == 0 || sum != res.Usage {
		t.Errorf("turn sum = %+v over %d turns, row usage = %+v, want equal and non-empty", sum, len(res.Turns), res.Usage)
	}
}

func TestTurns_NoMatchingTranscriptGivesNoTurns(t *testing.T) {
	t.Parallel()
	res, err := ScanConfigRoot(t.TempDir(), turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceNone || len(res.Turns) != 0 {
		t.Errorf("result = %+v, want source none and no turns", res)
	}
}

func TestTurns_Agy14ConversationGivesOneTurnPerGenerationSummingToTheRow(t *testing.T) {
	t.Parallel()
	store := &agyStore{rows: map[string]string{
		"conversation_summaries.db": agy14BuildID + "\n",
		agy14BuildID + ".db":        agy14GenRows(t),
	}}

	got := AgyConversationCollector("/agy", agy14Window(), store.query)()

	if len(got.Turns) != 111 {
		t.Fatalf("Turns = %d, want 111: one per generation row of the cycle-1967 build conversation", len(got.Turns))
	}
	var sum cyclestate.TokenUsage
	maxPrompt := 0
	for i, turn := range got.Turns {
		if turn.Index != i+1 {
			t.Fatalf("Turns[%d].Index = %d, want %d", i, turn.Index, i+1)
		}
		if turn.PromptTokens != turn.Usage.Input+turn.Usage.CacheRead {
			t.Errorf("Turns[%d] prompt tokens = %d, want input + cache read = %d", i, turn.PromptTokens, turn.Usage.Input+turn.Usage.CacheRead)
		}
		sum.Input += turn.Usage.Input
		sum.Output += turn.Usage.Output
		sum.CacheRead += turn.Usage.CacheRead
		sum.CacheWrite += turn.Usage.CacheWrite
		maxPrompt = max(maxPrompt, turn.PromptTokens)
	}
	if sum != got.Usage {
		t.Errorf("turn sum = %+v, row usage = %+v, want equal", sum, got.Usage)
	}
	if maxPrompt != got.PeakPromptTokens {
		t.Errorf("largest turn prompt = %d, PeakPromptTokens = %d, want equal", maxPrompt, got.PeakPromptTokens)
	}
}

func TestTurns_AgyMalformedGenerationRowIsNotATurn(t *testing.T) {
	t.Parallel()
	good := agy14GenRows(t)
	store := &agyStore{rows: map[string]string{
		"conversation_summaries.db": agy14BuildID + "\n",
		agy14BuildID + ".db":        "zz\n" + good,
	}}

	got := AgyConversationCollector("/agy", agy14Window(), store.query)()

	if len(got.Turns) != 111 || got.Warn == "" {
		t.Errorf("turns = %d, warn = %q, want 111 turns (the malformed row is skipped, not counted) and a warning", len(got.Turns), got.Warn)
	}
}
