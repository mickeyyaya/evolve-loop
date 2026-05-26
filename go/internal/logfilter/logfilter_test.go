package logfilter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRaw is a test helper that writes a raw <phase>-stdout.log into a
// fresh temp workspace and returns the workspace path.
func writeRaw(t *testing.T, phase, body string) string {
	t.Helper()
	ws := t.TempDir()
	raw := filepath.Join(ws, phase+"-stdout.log")
	if err := os.WriteFile(raw, []byte(body), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	return ws
}

// readClean returns <workspace>/<phase>-stdout.clean.txt or "" if absent.
func readClean(t *testing.T, ws, phase string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, phase+"-stdout.clean.txt"))
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read clean: %v", err)
	}
	return string(b)
}

func TestProcess_MissingRawFile_NoOp(t *testing.T) {
	ws := t.TempDir()
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("expected nil for missing raw, got %v", err)
	}
	if got := readClean(t, ws, "scout"); got != "" {
		t.Fatalf("expected no clean file, got %q", got)
	}
}

func TestProcess_KeepsAssistantText(t *testing.T) {
	raw := `{"type":"assistant","message":{"id":"msg_1","role":"assistant","content":[{"type":"text","text":"the report body"}]}}` + "\n"
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if !strings.Contains(got, "the report body") {
		t.Fatalf("expected assistant text in clean output, got %q", got)
	}
	if !strings.Contains(got, "[assistant]") {
		t.Fatalf("expected [assistant] prefix in output, got %q", got)
	}
}

func TestProcess_KeepsAssistantThinking(t *testing.T) {
	raw := `{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"thinking","thinking":"reasoning step","signature":"BASE64GARBAGE"}]}}` + "\n"
	ws := writeRaw(t, "build", raw)
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	if !strings.Contains(got, "reasoning step") {
		t.Fatalf("expected thinking text in output, got %q", got)
	}
	if strings.Contains(got, "BASE64GARBAGE") {
		t.Fatalf("signature must be stripped, got %q", got)
	}
	if !strings.Contains(got, "[thinking]") {
		t.Fatalf("expected [thinking] prefix, got %q", got)
	}
}

func TestProcess_KeepsAssistantToolUse(t *testing.T) {
	raw := `{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls -la","description":"list files"}}]}}` + "\n"
	ws := writeRaw(t, "build", raw)
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	if !strings.Contains(got, "[tool_use") {
		t.Fatalf("expected [tool_use prefix, got %q", got)
	}
	if !strings.Contains(got, "Bash") {
		t.Fatalf("expected tool name, got %q", got)
	}
	if !strings.Contains(got, "ls -la") {
		t.Fatalf("expected command input visible, got %q", got)
	}
}

func TestProcess_DropsStreamEvent(t *testing.T) {
	raw := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"chunk"}}}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}}
{"type":"stream_event","event":{"type":"content_block_stop","index":0}}
{"type":"stream_event","event":{"type":"message_start","message":{"usage":{}}}}
{"type":"stream_event","event":{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}}
{"type":"stream_event","event":{"type":"message_stop"}}
`
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if strings.Contains(got, "stream_event") {
		t.Fatalf("stream_event must be dropped, got %q", got)
	}
}

func TestProcess_DropsSystemInit(t *testing.T) {
	raw := `{"type":"system","subtype":"init","cwd":"/x","tools":["a","b","c"]}` + "\n"
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if got != "" {
		t.Fatalf("init must produce no output, got %q", got)
	}
}

func TestProcess_CompressesHookEvents(t *testing.T) {
	raw := `{"type":"system","subtype":"hook_started","hook_name":"SessionStart:startup","hook_event":"SessionStart","hook_id":"abc"}
{"type":"system","subtype":"hook_response","hook_name":"SessionStart:startup","hook_event":"SessionStart","exit_code":0,"outcome":"success","output":"LARGE-OUTPUT-BLOCK-DROPPED"}
`
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if !strings.Contains(got, "[hook]") {
		t.Fatalf("expected [hook] compressed prefix, got %q", got)
	}
	if !strings.Contains(got, "SessionStart") {
		t.Fatalf("expected hook name visible, got %q", got)
	}
	if strings.Contains(got, "LARGE-OUTPUT-BLOCK-DROPPED") {
		t.Fatalf("hook payload must be dropped, got %q", got)
	}
}

func TestProcess_TruncatesLargeToolResult(t *testing.T) {
	big := strings.Repeat("X", 10000)
	raw := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"START` + big + `END"}]}}` + "\n"
	ws := writeRaw(t, "build", raw)
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	if !strings.Contains(got, "[tool_result") {
		t.Fatalf("expected [tool_result prefix, got %q", got)
	}
	if !strings.Contains(got, "elided") {
		t.Fatalf("expected elision marker, got %q", got)
	}
	if !strings.Contains(got, "START") {
		t.Fatalf("expected head preserved, got %q", got)
	}
	if strings.Contains(got, strings.Repeat("X", 500)) {
		t.Fatalf("payload not truncated: too much X surviving")
	}
}

func TestProcess_KeepsResult(t *testing.T) {
	raw := `{"type":"result","subtype":"success","is_error":false,"total_cost_usd":3.2,"result":"Cycle complete.","usage":{"input_tokens":100}}` + "\n"
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if !strings.Contains(got, "[result]") {
		t.Fatalf("expected [result] prefix, got %q", got)
	}
	if !strings.Contains(got, "Cycle complete") {
		t.Fatalf("expected result text, got %q", got)
	}
	if !strings.Contains(got, "3.2") {
		t.Fatalf("expected cost visible, got %q", got)
	}
}

func TestProcess_KeepsStatusAndRateLimit(t *testing.T) {
	raw := `{"type":"system","subtype":"status","status":"requesting"}
{"type":"rate_limit_event","limit":50,"remaining":42}
`
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if !strings.Contains(got, "[status]") {
		t.Fatalf("expected [status] prefix, got %q", got)
	}
	if !strings.Contains(got, "rate_limit") {
		t.Fatalf("expected rate_limit visible, got %q", got)
	}
}

func TestProcess_UnknownJSONKeptRaw(t *testing.T) {
	raw := `{"type":"future_event_we_dont_know","payload":"keep me"}` + "\n"
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if !strings.Contains(got, "future_event_we_dont_know") {
		t.Fatalf("unknown type must be preserved, got %q", got)
	}
}

func TestProcess_PlainTextSpinnerDropped(t *testing.T) {
	raw := "⠋\n⠙ ⠹\n  ⠸\nactual content here\n"
	ws := writeRaw(t, "build", raw)
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	for _, glyph := range []string{"⠋", "⠙", "⠹", "⠸"} {
		if strings.Contains(got, glyph) {
			t.Fatalf("spinner glyph %q must be dropped, got %q", glyph, got)
		}
	}
	if !strings.Contains(got, "actual content here") {
		t.Fatalf("real content must survive, got %q", got)
	}
}

func TestProcess_PlainTextDuplicatesCollapsed(t *testing.T) {
	raw := "repeated line\nrepeated line\nrepeated line\nrepeated line\nrepeated line\nunique\n"
	ws := writeRaw(t, "build", raw)
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	if !strings.Contains(got, "× 5") && !strings.Contains(got, "x 5") {
		t.Fatalf("expected collapse marker like '× 5 times', got %q", got)
	}
	if !strings.Contains(got, "unique") {
		t.Fatalf("trailing unique line must survive, got %q", got)
	}
	count := strings.Count(got, "repeated line")
	if count > 1 {
		t.Fatalf("expected collapsed (1 occurrence), got %d copies", count)
	}
}

func TestProcess_PlainTextDropsEmptyBoxBorders(t *testing.T) {
	raw := "╭──╮\n│  │\n╰──╯\nreal text\n"
	ws := writeRaw(t, "build", raw)
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	if !strings.Contains(got, "real text") {
		t.Fatalf("real text must survive, got %q", got)
	}
	for _, glyph := range []string{"╭", "╰"} {
		if strings.Contains(got, glyph) {
			t.Fatalf("empty box border %q must be dropped, got %q", glyph, got)
		}
	}
}

func TestProcess_MixedContent(t *testing.T) {
	raw := `{"type":"system","subtype":"hook_started","hook_name":"SessionStart","hook_id":"x"}
[SessionStart] No CLAUDE_SESSION_ID available; skipping observer lease registration
[SessionStart] Found 86 recent session(s)
{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"Hello from the agent"}]}}
`
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "scout")
	if !strings.Contains(got, "[hook]") {
		t.Fatalf("expected hook compressed line, got %q", got)
	}
	if !strings.Contains(got, "Hello from the agent") {
		t.Fatalf("expected assistant text, got %q", got)
	}
	if !strings.Contains(got, "[SessionStart] Found 86") {
		t.Fatalf("expected plain-text hook output preserved, got %q", got)
	}
}

func TestProcess_PreservesRawFile(t *testing.T) {
	raw := `{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"text","text":"hi"}]}}` + "\n"
	ws := writeRaw(t, "scout", raw)
	if err := Process(ws, "scout"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	gotRaw, err := os.ReadFile(filepath.Join(ws, "scout-stdout.log"))
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if string(gotRaw) != raw {
		t.Fatalf("raw file mutated: got %q want %q", string(gotRaw), raw)
	}
}

func TestProcess_CompressionRatioOnRealFixture(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "streamjson-input.log"))
	if err != nil {
		t.Skipf("fixture missing (build with `head -c 200000 cycle-106/build-stdout.log > testdata/streamjson-input.log`): %v", err)
	}
	ws := writeRaw(t, "build", string(in))
	if err := Process(ws, "build"); err != nil {
		t.Fatalf("Process: %v", err)
	}
	got := readClean(t, ws, "build")
	rawSize := len(in)
	cleanSize := len(got)
	if rawSize == 0 {
		t.Fatal("raw fixture empty")
	}
	ratio := float64(cleanSize) / float64(rawSize) * 100
	t.Logf("compression: raw=%d clean=%d retention=%.1f%%", rawSize, cleanSize, ratio)
	if ratio > 30.0 {
		t.Fatalf("retention %.1f%% > 30%% target", ratio)
	}
}
