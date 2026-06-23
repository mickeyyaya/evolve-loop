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

	"github.com/mickeyyaya/evolveloop/go/internal/textutil"
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
	if err := os.WriteFile(rawPath, []byte(`{"type":"result","total_cost_usd":1}`+"\n"), 0o644); err != nil {
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
	if got := textutil.TruncateInline("short", 100); got != "short" {
		t.Fatalf("short string must pass through unchanged, got %q", got)
	}
}

func TestTruncateInline_Shortened(t *testing.T) {
	// 10-byte string truncated to 4 bytes: keep head + elision marker
	// reporting the elided byte count (10-4=6).
	got := textutil.TruncateInline("0123456789", 4)
	if !strings.HasPrefix(got, "0123") {
		t.Fatalf("expected head %q preserved, got %q", "0123", got)
	}
	if !strings.Contains(got, "(6 bytes elided)") {
		t.Fatalf("expected elision count, got %q", got)
	}
}

func TestTruncateMiddle_NotShortened(t *testing.T) {
	if got := textutil.TruncateMiddle("short", 100, 100); got != "short" {
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

// TestPlainTextState_IsNoiseBlankFlushesPendingRun pins that a blank line
// (handled by isNoise) breaks an in-progress dedup streak and emits the
// pending collapsed run. classifyJSON intercepts whitespace lines before they
// reach plaintext in the full pipeline, so the blank→isNoise→flush path is
// covered at the unit level where the behavior actually lives.
func TestPlainTextState_IsNoiseBlankFlushesPendingRun(t *testing.T) {
	p := newPlainTextState()
	// Start a dedup run of two identical content lines.
	if _, emit := p.next("content"); emit {
		t.Fatalf("first content line should be absorbed, not emitted")
	}
	if _, emit := p.next("content"); emit {
		t.Fatalf("second identical line should be absorbed into the run")
	}
	// A blank line is noise; it must flush the pending "content (× 2 times)".
	flushed, emit := p.next("")
	if !emit {
		t.Fatalf("blank line should flush the pending dedup run")
	}
	if !strings.Contains(flushed, "content") || !strings.Contains(flushed, "× 2 times") {
		t.Fatalf("expected collapsed run flushed, got %q", flushed)
	}
}

func TestIsNoise_BlankIsNoise(t *testing.T) {
	for _, s := range []string{"", "   ", "\t  \t"} {
		if !isNoise(s) {
			t.Errorf("whitespace-only %q must be noise", s)
		}
	}
}

func TestPlainTextState_NoiseWithNoPendingRunEmitsNothing(t *testing.T) {
	p := newPlainTextState()
	formatted, emit := p.next("   ") // noise, nothing pending
	if emit || formatted != "" {
		t.Fatalf("noise with empty buffer should drop silently, got emit=%v %q", emit, formatted)
	}
}

// TestProcess_OpenRawNonNotExistError pins that a non-IsNotExist open error on
// the raw log surfaces a wrapped "open raw" error. We make the workspace dir
// non-executable so opening any child path fails with EACCES (not ENOENT).
func TestProcess_OpenRawNonNotExistError(t *testing.T) {
	parent := t.TempDir()
	ws := filepath.Join(parent, "ws")
	if err := os.Mkdir(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "scout-stdout.log"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	// Remove search permission on the workspace dir → open child fails EACCES.
	if err := os.Chmod(ws, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	err := Process(ws, "scout")
	if err == nil {
		t.Fatalf("expected open error on unreadable workspace")
	}
	if os.IsNotExist(err) || !strings.Contains(err.Error(), "open raw") {
		t.Fatalf("got %v, want a wrapped 'open raw' (non-NotExist) error", err)
	}
}

// TestProcess_RenameErrorWhenCleanPathIsDirectory pins that a rename failure
// (clean path already exists as a directory) surfaces a "rename" error and
// removes the temp file rather than leaving a stray .tmp behind.
func TestProcess_RenameErrorWhenCleanPathIsDirectory(t *testing.T) {
	ws := t.TempDir()
	raw := `{"type":"result","subtype":"success","total_cost_usd":1,"result":"done"}` + "\n"
	if err := os.WriteFile(filepath.Join(ws, "scout-stdout.log"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	// Pre-create the clean path as a directory so os.Rename(tmp, clean) fails.
	if err := os.Mkdir(filepath.Join(ws, "scout-stdout.clean.txt"), 0o755); err != nil {
		t.Fatalf("mkdir clean: %v", err)
	}
	err := Process(ws, "scout")
	if err == nil || !strings.Contains(err.Error(), "rename") {
		t.Fatalf("got %v, want a rename error", err)
	}
	// No stray temp file should survive.
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".clean.tmp.") {
			t.Errorf("stray temp file left behind: %s", e.Name())
		}
	}
}

// TestProcess_FilterErrorCleansUp pins that when the filter step fails, the
// temp file is removed and a wrapped "filter" error is returned. We force the
// failure with an oversize line (exceeds the scanner's max buffer).
func TestProcess_FilterErrorCleansUp(t *testing.T) {
	ws := t.TempDir()
	huge := strings.Repeat("a", maxScannerBufBytes+10)
	if err := os.WriteFile(filepath.Join(ws, "scout-stdout.log"), []byte(huge), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	err := Process(ws, "scout")
	if err == nil || !strings.Contains(err.Error(), "filter") {
		t.Fatalf("got %v, want a wrapped filter error", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws, "scout-stdout.clean.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("clean file must not exist after filter failure, stat=%v", statErr)
	}
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".clean.tmp.") {
			t.Errorf("stray temp file left behind: %s", e.Name())
		}
	}
}

// bigLine is a single plain-text line larger than bufio.Writer's 4096-byte
// buffer, so writing it forces an immediate underlying Write (where our
// failingWriter can inject an error) rather than being silently buffered.
var bigLine = strings.Repeat("z", 8192)

// TestFilterStream_WriteErrorInJSONFlushPath pins propagation of a write error
// that occurs while flushing a pending plain-text dedup run at a JSON-line
// boundary. A large plain line buffers a pending run; the following JSON line
// triggers pt.flush() inside the JSON branch, and writing that >4 KB flush
// forces a real underlying Write that fails.
func TestFilterStream_WriteErrorInJSONFlushPath(t *testing.T) {
	in := strings.NewReader(bigLine + "\n" +
		`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"hi"}]}}` + "\n")
	if err := filterStream(in, &failingWriter{failAfter: 0}); err == nil {
		t.Fatal("expected write error during pending-run flush at JSON boundary")
	}
}

// TestFilterStream_WriteErrorWritingFormattedJSON pins the WriteString error
// branch on the JSON-formatted path: an assistant text whose formatted line
// exceeds the bufio buffer forces a direct underlying Write (bufio writes
// oversize payloads straight through) that fails.
func TestFilterStream_WriteErrorWritingFormattedJSON(t *testing.T) {
	// assistant text is emitted untruncated as "[assistant] " + text, so an
	// 8 KB text yields a >4 KB formatted line → direct write.
	text := strings.Repeat("q", 8192)
	in := strings.NewReader(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"` + text + `"}]}}` + "\n")
	if err := filterStream(in, &failingWriter{failAfter: 0}); err == nil {
		t.Fatal("expected write error writing oversized formatted JSON line")
	}
}

// TestFilterStream_WriteErrorWritingPlainText pins the plain-text emit branch:
// two distinct large plain lines — emitting the first (flushed when the second
// arrives) forces a real underlying Write that fails.
func TestFilterStream_WriteErrorWritingPlainText(t *testing.T) {
	in := strings.NewReader(bigLine + "A\n" + bigLine + "B\n")
	if err := filterStream(in, &failingWriter{failAfter: 0}); err == nil {
		t.Fatal("expected write error emitting oversized plain-text line")
	}
}

// bufWriterSize is bufio.Writer's default buffer capacity. A WriteString of
// exactly this many bytes fills the buffer without an underlying Write; the
// subsequent WriteByte('\n') then overflows and triggers a real Write — the
// only deterministic way to make a single-byte WriteByte hit the failing path.
const bufWriterSize = 4096

// TestFilterStream_WriteByteErrorOnFormattedJSON pins the newline-WriteByte
// error branch on the JSON-formatted path: an assistant text formatted to
// exactly the buffer size fills it, so writing the trailing '\n' fails.
func TestFilterStream_WriteByteErrorOnFormattedJSON(t *testing.T) {
	// formatted = "[assistant] " + text; size it to exactly bufWriterSize.
	textLen := bufWriterSize - len("[assistant] ")
	text := strings.Repeat("t", textLen)
	in := strings.NewReader(`{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"` + text + `"}]}}` + "\n")
	// WriteString of the exactly-4096-byte formatted line buffers without an
	// underlying Write; the trailing WriteByte('\n') is the first real Write.
	if err := filterStream(in, &failingWriter{failAfter: 0}); err == nil {
		t.Fatal("expected WriteByte('\\n') error after a buffer-filling formatted JSON line")
	}
}

// TestFilterStream_WriteByteErrorOnPlainText pins the newline-WriteByte error
// branch on the plain-text emit path. pt.next returns the flushed line WITH a
// trailing newline already, so a 4095-char line yields a 4096-byte flushed
// string that fills the buffer; filterStream's own WriteByte('\n') then fails.
func TestFilterStream_WriteByteErrorOnPlainText(t *testing.T) {
	line1 := strings.Repeat("p", bufWriterSize-1) // flushed = line + "\n" = 4096
	// A second distinct line forces line1 to be flushed/emitted mid-stream.
	in := strings.NewReader(line1 + "\n" + "second\n")
	// WriteString of the flushed 4096-byte string buffers cleanly; the
	// subsequent WriteByte('\n') is the first real Write and fails.
	if err := filterStream(in, &failingWriter{failAfter: 0}); err == nil {
		t.Fatal("expected WriteByte('\\n') error after a buffer-filling plain-text line")
	}
}

// TestFilterStream_WriteErrorOnEndOfStreamFlush pins that a write error during
// the final end-of-stream flush of a pending dedup run propagates. A single
// large plain line leaves a pending run flushed only after the scan loop; a
// writer that fails on that write exercises the tail flush path (logfilter.go
// lines 105-107) distinctly from the out.Flush() at 109.
func TestFilterStream_WriteErrorOnEndOfStreamFlush(t *testing.T) {
	in := strings.NewReader(bigLine + "\n")
	if err := filterStream(in, &failingWriter{failAfter: 0}); err == nil {
		t.Fatal("expected write error during end-of-stream flush")
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
