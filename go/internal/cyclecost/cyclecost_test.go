package cyclecost

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// diffAbs returns |a-b| for float epsilon comparisons.
func diffAbs(a, b float64) float64 { return math.Abs(a - b) }

// writeLog seeds a stdout-log file in workspace.
func writeLog(t *testing.T, workspace, name, content string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// resultLine generates a stream-json result event with the given cost
// + token counts.
func resultLine(cost float64, in, out, cacheRead, cacheCreate int64) string {
	ev := map[string]any{
		"type":           "result",
		"total_cost_usd": cost,
		"usage": map[string]any{
			"input_tokens":                 in,
			"output_tokens":                out,
			"cache_read_input_tokens":      cacheRead,
			"cache_creation_input_tokens":  cacheCreate,
		},
	}
	b, _ := json.Marshal(ev)
	return string(b)
}

func TestSummarizeCycle_Empty(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := SummarizeCycle(ws, 1)
	if !errors.Is(err, ErrNoLogs) {
		t.Fatalf("err=%v want ErrNoLogs", err)
	}
}

func TestSummarizeCycle_MissingWorkspace(t *testing.T) {
	t.Parallel()
	_, err := SummarizeCycle(filepath.Join(t.TempDir(), "nope"), 1)
	if !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("err=%v want ErrNoWorkspace", err)
	}
}

func TestSummarizeCycle_WorkspaceIsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ws")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := SummarizeCycle(path, 1)
	if !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("err=%v want ErrNoWorkspace", err)
	}
}

func TestSummarizeCycle_SingleStreamJSONLog(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-1")
	// Stream-json: many intermediate events, one result at the end.
	content := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant_message","content":"thinking..."}`,
		`{"type":"tool_use","tool":"Read"}`,
		resultLine(0.0512, 1234, 567, 9000, 1500),
	}, "\n")
	writeLog(t, ws, "scout-stdout.log", content)

	s, err := SummarizeCycle(ws, 1)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Cycle != 1 {
		t.Fatalf("cycle=%d want 1", s.Cycle)
	}
	if len(s.Phases) != 1 {
		t.Fatalf("phases=%d want 1", len(s.Phases))
	}
	pc := s.Phases[0]
	if pc.Phase != "scout" {
		t.Fatalf("phase=%q want scout", pc.Phase)
	}
	if pc.CostUSD != 0.0512 {
		t.Fatalf("cost=%v want 0.0512", pc.CostUSD)
	}
	if pc.InputTokens != 1234 || pc.OutputTokens != 567 {
		t.Fatalf("tokens: in=%d out=%d want 1234/567", pc.InputTokens, pc.OutputTokens)
	}
	if s.Total.CostUSD != 0.0512 {
		t.Fatalf("total cost=%v want 0.0512", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_MultipleLogsSummed(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-2")
	writeLog(t, ws, "scout-stdout.log", resultLine(0.10, 1000, 200, 0, 0))
	writeLog(t, ws, "builder-stdout.log", resultLine(0.30, 5000, 800, 0, 0))
	writeLog(t, ws, "auditor-stdout.log", resultLine(0.15, 2000, 400, 0, 0))

	s, err := SummarizeCycle(ws, 2)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(s.Phases) != 3 {
		t.Fatalf("phases=%d want 3", len(s.Phases))
	}
	// Sorted scan → phases ordered alphabetically: auditor, builder, scout.
	wantOrder := []string{"auditor", "builder", "scout"}
	for i, want := range wantOrder {
		if s.Phases[i].Phase != want {
			t.Fatalf("phases[%d]=%q want %q", i, s.Phases[i].Phase, want)
		}
	}
	// Float-precision: 0.10+0.30+0.15 = 0.5499999... in IEEE 754.
	// Assert within 1e-9 epsilon rather than exact equality.
	if got, want := s.Total.CostUSD, 0.55; diffAbs(got, want) > 1e-9 {
		t.Fatalf("total cost=%v want %v (epsilon-equal)", got, want)
	}
	if s.Total.InputTokens != 8000 || s.Total.OutputTokens != 1400 {
		t.Fatalf("total tokens: in=%d out=%d want 8000/1400", s.Total.InputTokens, s.Total.OutputTokens)
	}
}

func TestSummarizeCycle_LastResultWins(t *testing.T) {
	t.Parallel()
	// Two result events in one log — the LAST one wins (mirrors `tail -1`).
	ws := filepath.Join(t.TempDir(), "cycle-3")
	content := strings.Join([]string{
		resultLine(0.99, 100, 100, 0, 0), // older, should be ignored
		`{"type":"assistant_message","content":"another turn"}`,
		resultLine(0.05, 200, 50, 0, 0), // newer, wins
	}, "\n")
	writeLog(t, ws, "scout-stdout.log", content)

	s, err := SummarizeCycle(ws, 3)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.05 {
		t.Fatalf("cost=%v want 0.05 (last result wins)", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_LegacySingleBlobFallback(t *testing.T) {
	t.Parallel()
	// Legacy log: only one JSON blob with `"type":"result"`. The
	// pre-filter still catches it.
	ws := filepath.Join(t.TempDir(), "cycle-4")
	content := resultLine(0.02, 50, 10, 0, 0)
	writeLog(t, ws, "tdd-stdout.log", content)

	s, err := SummarizeCycle(ws, 4)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.02 {
		t.Fatalf("cost=%v want 0.02", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_NoResultTypeFallback(t *testing.T) {
	t.Parallel()
	// Log has no result event — fallback parses the last line as
	// a legacy blob.
	ws := filepath.Join(t.TempDir(), "cycle-5")
	// Use a legacy-style blob WITHOUT `"type":"result"` substring.
	// The fallback tail-1 should still parse it for cost.
	legacy := `{"total_cost_usd":0.07,"usage":{"input_tokens":1,"output_tokens":1}}`
	writeLog(t, ws, "ship-stdout.log", legacy)

	s, err := SummarizeCycle(ws, 5)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.07 {
		t.Fatalf("fallback cost=%v want 0.07", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_EmptyLogSkipped(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-6")
	// One usable log, one empty.
	writeLog(t, ws, "scout-stdout.log", resultLine(0.01, 1, 1, 0, 0))
	writeLog(t, ws, "builder-stdout.log", "") // empty
	s, err := SummarizeCycle(ws, 6)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(s.Phases) != 1 {
		t.Fatalf("phases=%d want 1 (empty log skipped)", len(s.Phases))
	}
}

func TestSummarizeCycle_MalformedJSONSkipped(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-7")
	// First log good; second log malformed.
	writeLog(t, ws, "scout-stdout.log", resultLine(0.10, 1, 1, 0, 0))
	writeLog(t, ws, "builder-stdout.log", `{not json}`)
	s, err := SummarizeCycle(ws, 7)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	// Builder skipped (no parseable result), scout counted.
	if s.Total.CostUSD != 0.10 {
		t.Fatalf("cost=%v want 0.10 (malformed skipped)", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_NonStandardPhaseName(t *testing.T) {
	t.Parallel()
	// Phase name with dots (e.g., parallel-worker logs) — must
	// preserve the full name minus the suffix.
	ws := filepath.Join(t.TempDir(), "cycle-8")
	writeLog(t, ws, "subagent.scout.parallel-worker-1-stdout.log",
		resultLine(0.01, 1, 1, 0, 0))
	s, err := SummarizeCycle(ws, 8)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got, want := s.Phases[0].Phase, "subagent.scout.parallel-worker-1"; got != want {
		t.Fatalf("phase=%q want %q", got, want)
	}
}

// TestSummarizeCycle_LongLines verifies that scanner handles the
// large stream-json events (each can be hundreds of KB).
func TestSummarizeCycle_LongLines(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-9")
	// Build a result event with a huge embedded payload.
	pad := strings.Repeat("x", 100000)
	content := fmt.Sprintf(`{"type":"result","total_cost_usd":0.42,"usage":{"input_tokens":1,"output_tokens":1},"_pad":%q}`, pad)
	writeLog(t, ws, "scout-stdout.log", content)
	s, err := SummarizeCycle(ws, 9)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.42 {
		t.Fatalf("cost=%v want 0.42 (long-line case)", s.Total.CostUSD)
	}
}

// TestSummarizeCycle_UnreadableLog covers the os.Open error branch
// in parsePhaseLog. We create the file then make it unreadable.
func TestSummarizeCycle_UnreadableLog(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod doesn't restrict reads")
	}
	ws := filepath.Join(t.TempDir(), "cycle-10")
	writeLog(t, ws, "scout-stdout.log", resultLine(0.10, 1, 1, 0, 0))
	logPath := filepath.Join(ws, "builder-stdout.log")
	if err := os.WriteFile(logPath, []byte(resultLine(0.20, 1, 1, 0, 0)), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(logPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(logPath, 0o644)

	s, err := SummarizeCycle(ws, 10)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	// Scout counted, builder skipped due to unreadable.
	if s.Total.CostUSD != 0.10 {
		t.Fatalf("cost=%v want 0.10 (unreadable log skipped)", s.Total.CostUSD)
	}
}
