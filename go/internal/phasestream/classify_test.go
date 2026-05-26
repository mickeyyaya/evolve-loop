package phasestream

import (
	"strings"
	"testing"
	"time"
)

// fixedNow gives deterministic timestamps for envelope assertions.
func fixedNow() func() time.Time {
	t := time.Date(2026, 5, 26, 8, 41, 9, 0, time.UTC)
	return func() time.Time { return t }
}

func newTestClassifier() *Classifier {
	return NewClassifier(Source{
		Producer: "normalizer", CLI: "claude-p", Cycle: 12, Phase: "build", Agent: "build",
	}, "cycle-12-build-1748246469", fixedNow())
}

// Real claude-p result-event shape (cycle-107 build-stdout.log, redacted text).
const realResultLine = `{"type":"result","subtype":"success","is_error":false,"duration_ms":177003,"num_turns":22,"total_cost_usd":0.69127425,"usage":{"input_tokens":17,"cache_creation_input_tokens":66945,"cache_read_input_tokens":1136065,"output_tokens":6624}}`

func TestClassifier_ResultEvent_ExtractsCostAndTokens(t *testing.T) {
	c := newTestClassifier()
	out := c.Line([]byte(realResultLine))
	if len(out) != 1 {
		t.Fatalf("want 1 envelope, got %d", len(out))
	}
	e := out[0]
	if e.Kind != KindResult {
		t.Fatalf("want kind=result, got %q", e.Kind)
	}
	if e.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version: want %q got %q", SchemaVersion, e.SchemaVersion)
	}
	if e.TraceID != "cycle-12-build-1748246469" {
		t.Errorf("trace_id not propagated: %q", e.TraceID)
	}
	if got := asFloat(e.Data["cost_usd"]); got != 0.69127425 {
		t.Errorf("cost_usd: want 0.69127425 got %v", got)
	}
	tok, _ := e.Data["tokens"].(map[string]any)
	if tok == nil {
		t.Fatalf("tokens missing: %#v", e.Data)
	}
	if asInt(tok["in"]) != 17 || asInt(tok["out"]) != 6624 ||
		asInt(tok["cache_r"]) != 1136065 || asInt(tok["cache_c"]) != 66945 {
		t.Errorf("token counts wrong: %#v", tok)
	}
}

func TestClassifier_StreamEvent_CoalescesIntoProgress(t *testing.T) {
	c := newTestClassifier()
	se := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}}`
	for i := 0; i < 5; i++ {
		if out := c.Line([]byte(se)); len(out) != 0 {
			t.Fatalf("stream_event must emit nothing inline, got %d", len(out))
		}
	}
	env, ok := c.FlushProgress()
	if !ok {
		t.Fatal("FlushProgress should emit after pending deltas")
	}
	if env.Kind != KindProgress {
		t.Fatalf("want kind=progress, got %q", env.Kind)
	}
	if asInt(env.Data["delta_count"]) != 5 {
		t.Errorf("delta_count: want 5 got %v", env.Data["delta_count"])
	}
	// No pending deltas → no tick.
	if _, ok := c.FlushProgress(); ok {
		t.Error("FlushProgress should be a no-op when nothing pending")
	}
}

func TestClassifier_AssistantText_Emitted(t *testing.T) {
	c := newTestClassifier()
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello world"}]}}`
	out := c.Line([]byte(line))
	if len(out) != 1 || out[0].Kind != KindAssistantText {
		t.Fatalf("want 1 assistant_text, got %#v", out)
	}
	if out[0].Data["text"] != "hello world" {
		t.Errorf("text not preserved: %v", out[0].Data["text"])
	}
}

func TestClassifier_Interaction_AskUserQuestion_FullFidelity(t *testing.T) {
	c := newTestClassifier()
	// A long option description that the generic 200-byte tool_use clamp WOULD truncate.
	longDesc := strings.Repeat("this recommendation explains the tradeoff in detail. ", 12) // ~620 bytes
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_1","name":"AskUserQuestion","input":{"questions":[{"question":"Which approach?","header":"Approach","options":[{"label":"Full collapse (Recommended)","description":"` + longDesc + `"},{"label":"Shadow","description":"safer"}]}]}}]}}`
	out := c.Line([]byte(line))
	if len(out) != 1 || out[0].Kind != KindInteraction {
		t.Fatalf("want 1 interaction, got %#v", out)
	}
	e := out[0]
	if e.Data["mode"] != "ask_user_question" {
		t.Errorf("mode: %v", e.Data["mode"])
	}
	// Full fidelity: the long description must survive intact (not truncated).
	blob := flatten(e.Data)
	if !strings.Contains(blob, longDesc) {
		t.Errorf("interaction truncated the recommendation/options; full fidelity required")
	}
	if !strings.Contains(blob, "Recommended") {
		t.Errorf("recommended option label lost")
	}
}

func TestClassifier_Interaction_ExitPlanMode(t *testing.T) {
	c := newTestClassifier()
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_2","name":"ExitPlanMode","input":{"plan":"step 1\nstep 2"}}]}}`
	out := c.Line([]byte(line))
	if len(out) != 1 || out[0].Kind != KindInteraction {
		t.Fatalf("want 1 interaction, got %#v", out)
	}
	if out[0].Data["mode"] != "exit_plan_mode" {
		t.Errorf("mode: %v", out[0].Data["mode"])
	}
	if !strings.Contains(flatten(out[0].Data), "step 2") {
		t.Errorf("plan body lost")
	}
}

func TestClassifier_ToolResult_ErrorIsWarn(t *testing.T) {
	c := newTestClassifier()
	line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu_1","is_error":true,"content":"boom"}]}}`
	out := c.Line([]byte(line))
	if len(out) != 1 || out[0].Kind != KindToolResult {
		t.Fatalf("want 1 tool_result, got %#v", out)
	}
	if out[0].Severity != SeverityWarn {
		t.Errorf("errored tool_result should be WARN, got %q", out[0].Severity)
	}
}

func TestClassifier_RateLimit_Structured(t *testing.T) {
	c := newTestClassifier()
	line := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","rateLimitType":"five_hour"}}`
	out := c.Line([]byte(line))
	if len(out) != 1 || out[0].Kind != KindRateLimit {
		t.Fatalf("want 1 rate_limit, got %#v", out)
	}
	if out[0].Severity != SeverityWarn {
		t.Errorf("rate_limit should be WARN")
	}
}

func TestClassifier_Stderr_InfraFailureMarkers(t *testing.T) {
	cases := []struct {
		name, line, marker string
	}{
		{"eperm", "sandbox-exec: sandbox_apply: Operation not permitted", "eperm"},
		{"api_529", "API Error 529 Overloaded", "api_529"},
		{"timeout", "dial tcp: operation timed out", "timeout"},
		{"conn_refused", "connect: connection refused", "conn_refused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClassifier()
			out := c.Stderr([]byte(tc.line))
			if len(out) != 1 || out[0].Kind != KindInfraFailure {
				t.Fatalf("want 1 infra_failure, got %#v", out)
			}
			if out[0].Data["marker"] != tc.marker {
				t.Errorf("marker: want %q got %v", tc.marker, out[0].Data["marker"])
			}
			if out[0].Data["source"] != "stderr" {
				t.Errorf("source: want stderr got %v", out[0].Data["source"])
			}
			if out[0].Severity != SeverityIncident {
				t.Errorf("infra_failure must be INCIDENT")
			}
		})
	}
}

// TestClassifier_Plaintext_InfraMarkerSurfaces guards the parity contract
// for cycleclassify's hard-switch to the events stream (ADR-0020 task 4):
// the legacy classifier scanned *-stdout.log too, so infra markers that
// land on STDOUT (the cycle-61 memo-stdout.log 529) must surface as
// infra_failure, symmetric with Stderr(). Without this, the events-only
// filter would silently miss stdout-borne infra.
func TestClassifier_Plaintext_InfraMarkerSurfaces(t *testing.T) {
	cases := []struct {
		name, line, marker string
	}{
		{"api_529", "API Error 529 Overloaded — retrying", "api_529"},
		{"api_429", "got 429 Too Many Requests from anthropic", "api_429"},
		{"eperm", "sandbox-exec: deny Operation not permitted", "eperm"},
		{"timeout", "phase exit: operation timed out", "timeout"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClassifier()
			out := c.Line([]byte(tc.line))
			if len(out) != 1 || out[0].Kind != KindInfraFailure {
				t.Fatalf("want 1 infra_failure from stdout, got %#v", out)
			}
			if out[0].Data["marker"] != tc.marker {
				t.Errorf("marker: want %q got %v", tc.marker, out[0].Data["marker"])
			}
			if out[0].Data["source"] != "stdout" {
				t.Errorf("source: want stdout got %v", out[0].Data["source"])
			}
			if out[0].Severity != SeverityIncident {
				t.Errorf("infra_failure must be INCIDENT")
			}
		})
	}
}

func TestClassifier_UnknownType_NotDropped(t *testing.T) {
	c := newTestClassifier()
	out := c.Line([]byte(`{"type":"brand_new_event","payload":42}`))
	if len(out) != 1 || out[0].Kind != KindUnknown {
		t.Fatalf("unknown type must be kept as unknown, got %#v", out)
	}
}

func TestClassifier_Plaintext_NoiseDroppedTextKept(t *testing.T) {
	c := newTestClassifier()
	// tmux spinner / border lines are noise.
	for _, noise := range []string{"⠋", "╭───────────╮", "   "} {
		if out := c.Line([]byte(noise)); len(out) != 0 {
			t.Errorf("noise %q should be dropped, got %#v", noise, out)
		}
	}
	// real prose is signal.
	out := c.Line([]byte("Auditor is alive and working."))
	if len(out) != 1 || out[0].Kind != KindAssistantText {
		t.Fatalf("plaintext prose should be assistant_text, got %#v", out)
	}
}

func TestClassifier_SeqMonotonic(t *testing.T) {
	c := newTestClassifier()
	a := c.Line([]byte(realResultLine))[0]
	b := c.Line([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"y"}]}}`))[0]
	if b.Seq <= a.Seq {
		t.Errorf("seq must be monotonic: a=%d b=%d", a.Seq, b.Seq)
	}
}

// ---- tiny test helpers (assert on map[string]any wire shape) ----

func asFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return -1
}

func asInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return -1
}

func flatten(m map[string]any) string {
	var sb strings.Builder
	var rec func(v any)
	rec = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for _, vv := range t {
				rec(vv)
			}
		case []any:
			for _, vv := range t {
				rec(vv)
			}
		case string:
			sb.WriteString(t)
			sb.WriteByte('\n')
		}
	}
	rec(m)
	return sb.String()
}
