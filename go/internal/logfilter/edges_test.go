package logfilter

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failingWriter induces a synthetic write error to exercise filterStream's
// error path.
type failingWriter struct{ failAfter int }

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.failAfter <= 0 {
		return 0, errors.New("forced")
	}
	f.failAfter--
	return len(p), nil
}

func TestFilterStream_WriteErrorPropagates(t *testing.T) {
	in := strings.NewReader(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"hi"}]}}` + "\n")
	if err := filterStream(in, &failingWriter{}); err == nil {
		t.Fatal("expected write error to propagate")
	}
}

func TestProcess_FailedWriteLeavesNoCleanFile(t *testing.T) {
	// Pre-create the workspace as a regular file so os.CreateTemp fails
	// — exercises the create-temp error branch.
	ws := t.TempDir()
	rawPath := filepath.Join(ws, "scout-stdout.log")
	if err := os.WriteFile(rawPath, []byte(`{"type":"result","total_cost_usd":1}` + "\n"), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	// Make workspace unwritable to force os.CreateTemp to fail.
	if err := os.Chmod(ws, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(ws, 0o755) }()
	err := Process(ws, "scout")
	if err == nil {
		t.Fatal("expected error when workspace not writable")
	}
	// No clean file should exist.
	if _, statErr := os.Stat(filepath.Join(ws, "scout-stdout.clean.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("clean file must not exist, stat=%v", statErr)
	}
}

func TestClassifyJSON_NonJSONFallsThrough(t *testing.T) {
	handled, formatted := classifyJSON([]byte("plain text line"))
	if handled {
		t.Fatalf("plain text must not be handled as JSON; got formatted=%q", formatted)
	}
}

func TestClassifyJSON_MalformedJSONFallsThrough(t *testing.T) {
	handled, _ := classifyJSON([]byte(`{"type":"assistant"`)) // missing closing brace
	if handled {
		t.Fatal("malformed JSON must fall through to plain-text path")
	}
}

func TestClassifyJSON_BlankLineDropped(t *testing.T) {
	handled, formatted := classifyJSON([]byte("   "))
	if !handled || formatted != "" {
		t.Fatalf("blank line should be handled and dropped; got handled=%v formatted=%q", handled, formatted)
	}
}

func TestClassifyJSON_MissingTypeFallsThrough(t *testing.T) {
	handled, _ := classifyJSON([]byte(`{"no_type":"here"}`))
	if handled {
		t.Fatal("JSON without .type must fall through")
	}
}

func TestFormatSystem_UnknownSubtype(t *testing.T) {
	raw := []byte(`{"type":"system","subtype":"task_started","task_name":"x"}`)
	out := formatSystem(raw)
	if !strings.Contains(out, "[system:task_started]") {
		t.Fatalf("expected fallback prefix, got %q", out)
	}
}

func TestFormatSystem_MalformedJSON(t *testing.T) {
	raw := []byte(`{"type":"system","subtype":42}`) // wrong subtype type
	out := formatSystem(raw)
	if !strings.Contains(out, "[system]") {
		t.Fatalf("expected unparseable fallback, got %q", out)
	}
}

func TestFormatAssistant_EmptyContent(t *testing.T) {
	raw := []byte(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":""},{"type":"thinking","thinking":""}]}}`)
	out := formatAssistant(raw)
	if out != "" {
		t.Fatalf("empty text/thinking should produce empty output, got %q", out)
	}
}

func TestFormatAssistant_UnknownContentType(t *testing.T) {
	raw := []byte(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"future_content_type"}]}}`)
	out := formatAssistant(raw)
	if !strings.Contains(out, "[assistant:future_content_type]") {
		t.Fatalf("expected unknown-content fallback, got %q", out)
	}
}

func TestFormatAssistant_MalformedJSON(t *testing.T) {
	out := formatAssistant([]byte(`{"type":"assistant","message":bad`))
	if !strings.HasPrefix(out, "[assistant] ") {
		t.Fatalf("expected fallback prefix, got %q", out)
	}
}

func TestFormatUser_ToolResultNonStringContent(t *testing.T) {
	// content is an array, not a string — exercises the json.Unmarshal
	// fallback that renders the raw JSON.
	raw := []byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"part"}]}]}}`)
	out := formatUser(raw)
	if !strings.Contains(out, "[tool_result id=toolu_1]") {
		t.Fatalf("expected tool_result prefix, got %q", out)
	}
}

func TestFormatUser_UnknownContentType(t *testing.T) {
	raw := []byte(`{"type":"user","message":{"content":[{"type":"future_user_type"}]}}`)
	out := formatUser(raw)
	if !strings.Contains(out, "[user:future_user_type]") {
		t.Fatalf("expected unknown-content fallback, got %q", out)
	}
}

func TestFormatUser_MalformedJSON(t *testing.T) {
	out := formatUser([]byte(`{"type":"user","message":nope`))
	if !strings.HasPrefix(out, "[user] ") {
		t.Fatalf("expected fallback prefix, got %q", out)
	}
}

func TestFormatResult_NoResultField(t *testing.T) {
	raw := []byte(`{"type":"result","subtype":"success","total_cost_usd":1.5,"usage":{}}`)
	out := formatResult(raw)
	if strings.Contains(out, "\n") {
		t.Fatalf("expected single header line when result text empty, got %q", out)
	}
	if !strings.Contains(out, "[result]") {
		t.Fatalf("expected [result] prefix, got %q", out)
	}
}

func TestFormatResult_MalformedJSON(t *testing.T) {
	out := formatResult([]byte(`{"type":"result","subtype":42}`)) // wrong field type
	if !strings.HasPrefix(out, "[result] ") {
		t.Fatalf("expected fallback prefix, got %q", out)
	}
}

func TestTruncateInline_NotShortened(t *testing.T) {
	if got := truncateInline("short", 100); got != "short" {
		t.Fatalf("short string must pass through unchanged, got %q", got)
	}
}

func TestTruncateMiddle_NotShortened(t *testing.T) {
	if got := truncateMiddle("short", 100, 100); got != "short" {
		t.Fatalf("short string must pass through unchanged, got %q", got)
	}
}

func TestPlainTextState_ASCIISpinnersDropped(t *testing.T) {
	// Cover the ASCII rotator branches of isSpinnerRune.
	in := strings.NewReader("|\n/\n-\n\\\nreal\n")
	var buf bytes.Buffer
	if err := filterStream(in, &buf); err != nil {
		t.Fatalf("filterStream: %v", err)
	}
	got := buf.String()
	for _, glyph := range []string{"|\n", "/\n", "-\n", "\\\n"} {
		if strings.Contains(got, glyph) {
			t.Fatalf("ASCII spinner %q must be dropped, got %q", glyph, got)
		}
	}
	if !strings.Contains(got, "real") {
		t.Fatalf("real content must survive, got %q", got)
	}
}

func TestFilterStream_ScannerErrorOnLongLine(t *testing.T) {
	// A line longer than the scanner's max buffer triggers bufio.ErrTooLong.
	huge := strings.Repeat("a", maxScannerBufBytes+10)
	if err := filterStream(strings.NewReader(huge), io.Discard); err == nil {
		t.Fatal("expected scanner error on oversize line")
	}
}

// Sanity check our JSON shape assumptions don't drift silently.
func TestAssistantContent_JSONRoundTrip(t *testing.T) {
	in := assistantContent{Type: "tool_use", ID: "x", Name: "Bash", Input: json.RawMessage(`{"cmd":"ls"}`)}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out assistantContent
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(out.Input) != `{"cmd":"ls"}` {
		t.Fatalf("RawMessage round-trip lost data: %s", out.Input)
	}
}
