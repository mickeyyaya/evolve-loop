package phasestream

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/textutil"
)

// TestClassifier_NewClassifier_DefaultsClockToNow pins the NewClassifier
// nil-now fallback: a classifier built without an injected clock must still
// stamp a non-empty RFC3339 timestamp (it falls back to time.Now).
func TestClassifier_NewClassifier_DefaultsClockToNow(t *testing.T) {
	t.Parallel()
	c := NewClassifier(Source{Producer: "normalizer", Phase: "build", Agent: "build"}, "trace-x", nil)
	out := c.Line([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`))
	if len(out) != 1 {
		t.Fatalf("want 1 envelope, got %d", len(out))
	}
	if out[0].TS == "" {
		t.Errorf("nil-now fallback must still produce a timestamp; got empty")
	}
}

// TestClassifier_Stderr_BlankLineDropped pins the empty-stderr-line branch:
// a whitespace-only stderr line carries no signal and emits nothing.
func TestClassifier_Stderr_BlankLineDropped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	if out := c.Stderr([]byte("   \t  ")); out != nil {
		t.Errorf("blank stderr line must drop to nil, got %#v", out)
	}
}

// TestClassifier_Stderr_NonMarkerDropped pins the no-marker stderr branch:
// ordinary stderr noise (no infra marker) is dropped.
func TestClassifier_Stderr_NonMarkerDropped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	if out := c.Stderr([]byte("just some benign diagnostic chatter")); out != nil {
		t.Errorf("non-marker stderr must drop to nil, got %#v", out)
	}
}

// TestClassifier_Assistant_MalformedFallsBackToText pins the
// formatAssistant unmarshal-error branch: an assistant event whose message
// shape doesn't decode is preserved verbatim as assistant_text rather than
// silently lost.
func TestClassifier_Assistant_MalformedFallsBackToText(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	// type==assistant routes here, but message is a string not an object →
	// unmarshal into assistantEvent fails.
	raw := `{"type":"assistant","message":"not-an-object"}`
	out := c.Line([]byte(raw))
	if len(out) != 1 || out[0].Kind != KindAssistantText {
		t.Fatalf("malformed assistant must fall back to 1 assistant_text, got %#v", out)
	}
	if !strings.Contains(out[0].Data["text"].(string), "not-an-object") {
		t.Errorf("fallback must preserve the raw line; got %v", out[0].Data["text"])
	}
}

// TestClassifier_Assistant_EmptyBlocksSkipped pins the empty-text and
// empty-thinking continue branches: blocks with no payload emit nothing,
// while a populated thinking block emits exactly one thinking envelope.
func TestClassifier_Assistant_EmptyBlocksSkipped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	raw := `{"type":"assistant","message":{"content":[` +
		`{"type":"text","text":""},` +
		`{"type":"thinking","thinking":""},` +
		`{"type":"thinking","thinking":"deliberating"}]}}`
	out := c.Line([]byte(raw))
	if len(out) != 1 {
		t.Fatalf("empty text+thinking blocks must be skipped, leaving 1 envelope; got %d", len(out))
	}
	if out[0].Kind != KindThinking || out[0].Data["text"] != "deliberating" {
		t.Errorf("want the single populated thinking block, got %#v", out[0])
	}
}

// TestClassifier_Assistant_ToolUse_GenericClamp pins the default tool_use
// branch (a non-interactive tool): the input is excerpted into input_excerpt
// under the generic clamp.
func TestClassifier_Assistant_ToolUse_GenericClamp(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_9","name":"Bash","input":{"command":"ls"}}]}}`
	out := c.Line([]byte(raw))
	if len(out) != 1 || out[0].Kind != KindToolUse {
		t.Fatalf("generic tool_use must yield 1 tool_use, got %#v", out)
	}
	if out[0].Data["name"] != "Bash" {
		t.Errorf("tool name: got %v want Bash", out[0].Data["name"])
	}
	if !strings.Contains(out[0].Data["input_excerpt"].(string), "ls") {
		t.Errorf("input_excerpt must carry the command, got %v", out[0].Data["input_excerpt"])
	}
}

// TestClassifier_User_MalformedDropped pins the formatUser unmarshal-error
// branch: a user event that doesn't decode emits nothing (no crash, no
// envelope).
func TestClassifier_User_MalformedDropped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	if out := c.Line([]byte(`{"type":"user","message":42}`)); out != nil {
		t.Errorf("malformed user event must drop to nil, got %#v", out)
	}
}

// TestClassifier_User_NonToolResultSkipped pins the non-tool_result continue
// branch: a user content block that isn't a tool_result contributes nothing.
func TestClassifier_User_NonToolResultSkipped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	raw := `{"type":"user","message":{"content":[{"type":"text","content":"hello"}]}}`
	if out := c.Line([]byte(raw)); out != nil {
		t.Errorf("non-tool_result user content must be skipped, got %#v", out)
	}
}

// TestClassifier_User_ToolResultNonStringContent pins the content-not-string
// fallback in formatUser: when tool_result content is a JSON object (not a
// bare string), the raw bytes are used as the excerpt.
func TestClassifier_User_ToolResultNonStringContent(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	raw := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu_3","is_error":false,"content":{"nested":"obj"}}]}}`
	out := c.Line([]byte(raw))
	if len(out) != 1 || out[0].Kind != KindToolResult {
		t.Fatalf("want 1 tool_result, got %#v", out)
	}
	if out[0].Severity != SeverityInfo {
		t.Errorf("non-error tool_result should be INFO, got %q", out[0].Severity)
	}
	if !strings.Contains(out[0].Data["excerpt"].(string), "nested") {
		t.Errorf("object content must fall back to raw bytes in excerpt, got %v", out[0].Data["excerpt"])
	}
}

// TestClassifier_System_MalformedDropped pins the formatSystem unmarshal
// error branch: an undecodable system event emits nothing.
func TestClassifier_System_MalformedDropped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	if out := c.Line([]byte(`{"type":"system","subtype":[1,2,3]}`)); out != nil {
		t.Errorf("malformed system event must drop to nil, got %#v", out)
	}
}

// TestClassifier_System_HookWithExitCode pins the hook-name and exit-code
// branches of formatSystem: a hook system event surfaces the hook fields and
// the exit code.
func TestClassifier_System_HookWithExitCode(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	raw := `{"type":"system","subtype":"hook_result","hook_name":"phase-gate","hook_event":"PreToolUse","exit_code":2,"outcome":"blocked"}`
	out := c.Line([]byte(raw))
	if len(out) != 1 || out[0].Kind != KindSystemHook {
		t.Fatalf("want 1 system_hook, got %#v", out)
	}
	d := out[0].Data
	if d["hook_name"] != "phase-gate" || d["hook_event"] != "PreToolUse" || d["outcome"] != "blocked" {
		t.Errorf("hook fields not surfaced: %#v", d)
	}
	if asInt(d["exit_code"]) != 2 {
		t.Errorf("exit_code: got %v want 2", d["exit_code"])
	}
}

// TestClassifier_System_InitDropped pins the system:init noise branch.
func TestClassifier_System_InitDropped(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	if out := c.Line([]byte(`{"type":"system","subtype":"init"}`)); out != nil {
		t.Errorf("system:init is bootstrap noise and must drop, got %#v", out)
	}
}

// TestDecodeFull_InvalidJSONFallsBackToRawString pins decodeFull's
// invalid-JSON branch directly: a blob that isn't valid JSON is returned as
// the raw string rather than dropped.
func TestDecodeFull_InvalidJSONFallsBackToRawString(t *testing.T) {
	t.Parallel()
	if got := decodeFull([]byte(`{not valid json`)); got != `{not valid json` {
		t.Errorf("invalid JSON must fall back to raw string, got %#v", got)
	}
	// Valid JSON decodes into a structured value.
	got := decodeFull([]byte(`{"k":"v"}`))
	m, ok := got.(map[string]any)
	if !ok || m["k"] != "v" {
		t.Errorf("valid JSON must decode to a map, got %#v", got)
	}
}

// TestClassifier_Interaction_StringInput pins the AskUserQuestion path where
// the input is a JSON string literal: decodeFull returns that string as-is.
func TestClassifier_Interaction_StringInput(t *testing.T) {
	t.Parallel()
	c := newTestClassifier()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_x","name":"AskUserQuestion","input":"raw-string-input"}]}}`
	out := c.Line([]byte(raw))
	if len(out) != 1 || out[0].Kind != KindInteraction {
		t.Fatalf("want 1 interaction, got %#v", out)
	}
	if out[0].Data["input"] != "raw-string-input" {
		t.Errorf("string input should decode to that string, got %#v", out[0].Data["input"])
	}
}

// TestTruncateInline_TruncatesOversize pins the len>n truncation branch:
// a string longer than the limit is clipped and annotated with the elided
// byte count.
func TestTruncateInline_TruncatesOversize(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("a", 50)
	got := textutil.TruncateInline(in, 10)
	if !strings.HasPrefix(got, strings.Repeat("a", 10)) {
		t.Errorf("must keep the first 10 bytes, got %q", got)
	}
	if !strings.Contains(got, "40 bytes elided") {
		t.Errorf("must annotate elided count, got %q", got)
	}
	// Under-limit input is returned unchanged.
	if textutil.TruncateInline("short", 10) != "short" {
		t.Errorf("under-limit input must be unchanged")
	}
}

// TestIsSpinnerRune_NonSpinnerRune pins the default (non-spinner) branch.
func TestIsSpinnerRune_NonSpinnerRune(t *testing.T) {
	t.Parallel()
	if isSpinnerRune('A') {
		t.Errorf("'A' is not a spinner rune")
	}
	if !isSpinnerRune('|') {
		t.Errorf("'|' is a spinner rune")
	}
}

// TestIsNoise_MixedLineIsSignal pins the early-return branch: a line mixing a
// border rune with prose is NOT noise (returns false before scanning ends).
func TestIsNoise_MixedLineIsSignal(t *testing.T) {
	t.Parallel()
	if isNoise("│ real content here") {
		t.Errorf("a border rune mixed with prose is signal, not noise")
	}
	if !isNoise("") {
		t.Errorf("empty line is noise")
	}
}
