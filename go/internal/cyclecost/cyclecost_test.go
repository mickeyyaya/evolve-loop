package cyclecost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
)

func diffAbs(a, b float64) float64 { return math.Abs(a - b) }

func writeLog(t *testing.T, workspace, name, content string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func resultEnvelope(cost float64, in, out, cacheR, cacheC int64) string {
	b, _ := json.Marshal(map[string]any{
		"schema_version": "2.0",
		"seq":            1,
		"kind":           "result",
		"severity":       "INFO",
		"data": map[string]any{
			"cost_usd": cost,
			"tokens": map[string]any{
				"in":      in,
				"out":     out,
				"cache_r": cacheR,
				"cache_c": cacheC,
			},
		},
	})
	return string(b)
}

func textEnvelope(text string) string {
	b, _ := json.Marshal(map[string]any{
		"schema_version": "2.0",
		"seq":            1,
		"kind":           "assistant_text",
		"severity":       "INFO",
		"data":           map[string]any{"text": text},
	})
	return string(b)
}

func writeUsageSidecar(t *testing.T, workspace, phase string, cost float64, in, out, cacheR, cacheW int64) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	b, err := json.Marshal(map[string]any{
		"phase":    phase,
		"cost_usd": cost,
		"tokens": map[string]any{
			"input":       in,
			"output":      out,
			"cache_read":  cacheR,
			"cache_write": cacheW,
		},
	})
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, phase+"-usage.json"), b, 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
}

func TestCycleCost_PrefersSidecarOverEvents(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-11")
	writeLog(t, ws, "scout-events.ndjson", resultEnvelope(0.99, 900, 90, 9, 9))
	writeUsageSidecar(t, ws, "scout", 0.05, 100, 20, 3, 4)
	writeUsageSidecar(t, ws, "build", 0.30, 500, 60, 7, 8)
	if err := os.WriteFile(filepath.Join(ws, "token-usage.json"), []byte(`{"peak_tokens":12345}`), 0o644); err != nil {
		t.Fatalf("write token-usage.json: %v", err)
	}

	s, err := SummarizeCycle(ws, 11)
	if err != nil {
		t.Fatalf("SummarizeCycle: %v", err)
	}
	if len(s.Phases) != 2 {
		t.Fatalf("phases=%d want 2 (token-usage.json must not appear as a phase): %+v", len(s.Phases), s.Phases)
	}
	wantScout := PhaseCost{Phase: "scout", CostUSD: 0.99, InputTokens: 900, OutputTokens: 90, CacheReadInputTokens: 9, CacheCreationInputTokens: 9}
	wantBuild := PhaseCost{Phase: "build", CostUSD: 0.30, InputTokens: 500, OutputTokens: 60, CacheReadInputTokens: 7, CacheCreationInputTokens: 8}
	byPhase := map[string]PhaseCost{}
	for _, pc := range s.Phases {
		byPhase[pc.Phase] = pc
	}
	if byPhase["scout"] != wantScout {
		t.Fatalf("scout: got %+v want %+v (events.ndjson must win when both sources exist)", byPhase["scout"], wantScout)
	}
	if byPhase["build"] != wantBuild {
		t.Fatalf("build: got %+v want %+v (sidecar recovers a phase with no events.ndjson)", byPhase["build"], wantBuild)
	}
	if got, want := s.Total.CostUSD, 1.29; diffAbs(got, want) > 1e-9 {
		t.Fatalf("total cost=%v want %v", got, want)
	}
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

func TestSummarizeCycle_SingleEventsLog(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-1")
	content := strings.Join([]string{
		textEnvelope("thinking..."),
		`{"schema_version":"2.0","seq":2,"kind":"tool_use","severity":"INFO","data":{"name":"Read"}}`,
		resultEnvelope(0.0512, 1234, 567, 9000, 1500),
	}, "\n")
	writeLog(t, ws, "scout-events.ndjson", content)

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
	if pc.CacheReadInputTokens != 9000 || pc.CacheCreationInputTokens != 1500 {
		t.Fatalf("cache tokens: r=%d c=%d want 9000/1500", pc.CacheReadInputTokens, pc.CacheCreationInputTokens)
	}
	if s.Total.CostUSD != 0.0512 {
		t.Fatalf("total cost=%v want 0.0512", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_MultipleLogsSummed(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-2")
	writeLog(t, ws, "scout-events.ndjson", resultEnvelope(0.10, 1000, 200, 0, 0))
	writeLog(t, ws, "builder-events.ndjson", resultEnvelope(0.30, 5000, 800, 0, 0))
	writeLog(t, ws, "auditor-events.ndjson", resultEnvelope(0.15, 2000, 400, 0, 0))

	s, err := SummarizeCycle(ws, 2)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(s.Phases) != 3 {
		t.Fatalf("phases=%d want 3", len(s.Phases))
	}
	wantOrder := []string{"auditor", "builder", "scout"}
	for i, want := range wantOrder {
		if s.Phases[i].Phase != want {
			t.Fatalf("phases[%d]=%q want %q", i, s.Phases[i].Phase, want)
		}
	}
	if got, want := s.Total.CostUSD, 0.55; diffAbs(got, want) > 1e-9 {
		t.Fatalf("total cost=%v want %v (epsilon-equal)", got, want)
	}
	if s.Total.InputTokens != 8000 || s.Total.OutputTokens != 1400 {
		t.Fatalf("total tokens: in=%d out=%d want 8000/1400", s.Total.InputTokens, s.Total.OutputTokens)
	}
}

func TestSummarizeCycle_LastResultWins(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-3")
	content := strings.Join([]string{
		resultEnvelope(0.99, 100, 100, 0, 0),
		textEnvelope("another turn"),
		resultEnvelope(0.05, 200, 50, 0, 0),
	}, "\n")
	writeLog(t, ws, "scout-events.ndjson", content)

	s, err := SummarizeCycle(ws, 3)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.05 {
		t.Fatalf("cost=%v want 0.05 (last result wins)", s.Total.CostUSD)
	}
	if s.Total.InputTokens != 200 || s.Total.OutputTokens != 50 {
		t.Fatalf("tokens: in=%d out=%d want 200/50 (last result)", s.Total.InputTokens, s.Total.OutputTokens)
	}
}

func TestSummarizeCycle_NoResultEnvelopeSkipped(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-5")
	content := strings.Join([]string{
		textEnvelope("did some work"),
		`{"schema_version":"2.0","seq":2,"kind":"tool_use","severity":"INFO","data":{"name":"Edit"}}`,
	}, "\n")
	writeLog(t, ws, "ship-events.ndjson", content)

	s, err := SummarizeCycle(ws, 5)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(s.Phases) != 0 {
		t.Fatalf("phases=%d want 0 (no result envelope → skipped)", len(s.Phases))
	}
	if s.Total.CostUSD != 0 {
		t.Fatalf("total cost=%v want 0", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_EmptyLogSkipped(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-6")
	writeLog(t, ws, "scout-events.ndjson", resultEnvelope(0.01, 1, 1, 0, 0))
	writeLog(t, ws, "builder-events.ndjson", "")
	s, err := SummarizeCycle(ws, 6)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if len(s.Phases) != 1 {
		t.Fatalf("phases=%d want 1 (empty log skipped)", len(s.Phases))
	}
}

func TestSummarizeCycle_MalformedLineSkipped(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-7")
	writeLog(t, ws, "scout-events.ndjson", resultEnvelope(0.10, 1, 1, 0, 0))
	writeLog(t, ws, "builder-events.ndjson", `{not json}`)
	s, err := SummarizeCycle(ws, 7)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.10 {
		t.Fatalf("cost=%v want 0.10 (malformed skipped)", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_NonStandardPhaseName(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-8")
	writeLog(t, ws, "subagent.scout.parallel-worker-1-events.ndjson",
		resultEnvelope(0.01, 1, 1, 0, 0))
	s, err := SummarizeCycle(ws, 8)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got, want := s.Phases[0].Phase, "subagent.scout.parallel-worker-1"; got != want {
		t.Fatalf("phase=%q want %q", got, want)
	}
}

func TestSummarizeCycle_LargeResultEnvelopeParsed(t *testing.T) {
	t.Parallel()
	ws := filepath.Join(t.TempDir(), "cycle-9")
	pad := strings.Repeat("x", 100000)
	content := fmt.Sprintf(`{"schema_version":"2.0","seq":1,"kind":"result","severity":"INFO","data":{"cost_usd":0.42,"tokens":{"in":1,"out":1,"cache_r":0,"cache_c":0},"_pad":%q}}`, pad)
	writeLog(t, ws, "scout-events.ndjson", content)
	s, err := SummarizeCycle(ws, 9)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != 0.42 {
		t.Fatalf("cost=%v want 0.42 (long-line case)", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_UnreadableLog(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod doesn't restrict reads")
	}
	ws := filepath.Join(t.TempDir(), "cycle-10")
	writeLog(t, ws, "scout-events.ndjson", resultEnvelope(0.10, 1, 1, 0, 0))
	logPath := filepath.Join(ws, "builder-events.ndjson")
	if err := os.WriteFile(logPath, []byte(resultEnvelope(0.20, 1, 1, 0, 0)), 0o644); err != nil {
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
	if s.Total.CostUSD != 0.10 {
		t.Fatalf("cost=%v want 0.10 (unreadable log skipped)", s.Total.CostUSD)
	}
}

func TestSummarizeCycle_ParityViaProduce(t *testing.T) {
	t.Parallel()
	const (
		rawCost = 0.69127425
		rawIn   = int64(17)
		rawOut  = int64(6624)
		rawCR   = int64(1136065)
		rawCC   = int64(66945)
	)
	ws := filepath.Join(t.TempDir(), "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	raw := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"building"}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta"}}`,
		fmt.Sprintf(`{"type":"result","total_cost_usd":%v,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}`,
			rawCost, rawIn, rawOut, rawCR, rawCC),
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write raw log: %v", err)
	}

	if err := phasestream.Produce(phasestream.ProduceConfig{Workspace: ws, Phase: "build", CLI: "claude-p", Cycle: 7}); err != nil {
		t.Fatalf("Produce: %v", err)
	}

	s, err := SummarizeCycle(ws, 7)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != rawCost {
		t.Fatalf("parity cost: got %v want %v", s.Total.CostUSD, rawCost)
	}
	if s.Total.InputTokens != rawIn || s.Total.OutputTokens != rawOut ||
		s.Total.CacheReadInputTokens != rawCR || s.Total.CacheCreationInputTokens != rawCC {
		t.Fatalf("parity tokens: %+v", s.Total)
	}
}

func TestSummarizeCycle_ParityRawVsEvents(t *testing.T) {
	t.Parallel()
	const (
		rawCost = 0.4242
		rawIn   = int64(12000)
		rawOut  = int64(800)
		rawCR   = int64(5000)
		rawCC   = int64(1500)
	)
	rawLines := []string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta"}}`,
		fmt.Sprintf(`{"type":"result","total_cost_usd":%v,"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}`,
			rawCost, rawIn, rawOut, rawCR, rawCC),
	}

	clf := phasestream.NewClassifier(
		phasestream.Source{Producer: "normalizer", CLI: "claude-p", Cycle: 1, Phase: "scout", Agent: "scout"},
		"parity-trace", nil)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, ln := range rawLines {
		for _, e := range clf.Line([]byte(ln)) {
			if err := enc.Encode(e); err != nil {
				t.Fatalf("encode envelope: %v", err)
			}
		}
	}

	ws := filepath.Join(t.TempDir(), "cycle-1")
	writeLog(t, ws, "scout-events.ndjson", buf.String())

	s, err := SummarizeCycle(ws, 1)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if s.Total.CostUSD != rawCost {
		t.Fatalf("parity cost: got %v want %v", s.Total.CostUSD, rawCost)
	}
	if s.Total.InputTokens != rawIn || s.Total.OutputTokens != rawOut {
		t.Fatalf("parity tokens: in=%d out=%d want %d/%d", s.Total.InputTokens, s.Total.OutputTokens, rawIn, rawOut)
	}
	if s.Total.CacheReadInputTokens != rawCR || s.Total.CacheCreationInputTokens != rawCC {
		t.Fatalf("parity cache: r=%d c=%d want %d/%d", s.Total.CacheReadInputTokens, s.Total.CacheCreationInputTokens, rawCR, rawCC)
	}
}
