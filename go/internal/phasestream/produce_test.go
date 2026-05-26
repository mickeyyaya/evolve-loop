package phasestream

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readEvents decodes every envelope in <ws>/<phase>-events.ndjson.
func readEvents(t *testing.T, ws, phase string) []Envelope {
	t.Helper()
	f, err := os.Open(filepath.Join(ws, phase+"-events.ndjson"))
	if err != nil {
		t.Fatalf("open events: %v", err)
	}
	defer f.Close()
	var out []Envelope
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<10), 1<<20)
	for sc.Scan() {
		var e Envelope
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("decode envelope %q: %v", sc.Text(), err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan events: %v", err)
	}
	return out
}

func writeRaw(t *testing.T, ws, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestProduce_WritesResultAndInfra is the core post-phase render: stdout
// stream-json (with a result) plus a stderr infra marker normalize into one
// <phase>-events.ndjson carrying both the billing-critical result and the
// infra_failure event.
func TestProduce_WritesResultAndInfra(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRaw(t, ws, "build-stdout.log", strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta"}}`,
		`{"type":"result","total_cost_usd":0.5,"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}`,
	}, "\n")+"\n")
	writeRaw(t, ws, "build-stderr.log", "sandbox-exec: Operation not permitted\n")

	if err := Produce(ProduceConfig{Workspace: ws, Phase: "build", CLI: "claude-p", Cycle: 7}); err != nil {
		t.Fatalf("Produce: %v", err)
	}

	envs := readEvents(t, ws, "build")
	var sawResult, sawInfra bool
	for _, e := range envs {
		switch e.Kind {
		case KindResult:
			sawResult = true
			if e.Data["cost_usd"] != 0.5 {
				t.Errorf("result cost: got %v want 0.5", e.Data["cost_usd"])
			}
		case KindInfraFailure:
			sawInfra = true
			if e.Data["marker"] != "eperm" {
				t.Errorf("infra marker: got %v want eperm", e.Data["marker"])
			}
		}
	}
	if !sawResult {
		t.Error("expected a result envelope")
	}
	if !sawInfra {
		t.Error("expected an infra_failure envelope from stderr")
	}
}

// TestProduce_UnterminatedFinalLine is the billing-safety guarantee that the
// live Poll tail can NOT make: a finished log whose final result line has no
// trailing newline must still be captured (Poll holds back partial lines).
func TestProduce_UnterminatedFinalLine(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRaw(t, ws, "scout-stdout.log",
		`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}`+"\n"+
			`{"type":"result","total_cost_usd":0.25,"usage":{"input_tokens":10,"output_tokens":5}}`) // NO trailing newline

	if err := Produce(ProduceConfig{Workspace: ws, Phase: "scout", CLI: "claude-p", Cycle: 1}); err != nil {
		t.Fatalf("Produce: %v", err)
	}
	var sawResult bool
	for _, e := range readEvents(t, ws, "scout") {
		if e.Kind == KindResult {
			sawResult = true
			if e.Data["cost_usd"] != 0.25 {
				t.Errorf("cost: got %v want 0.25", e.Data["cost_usd"])
			}
		}
	}
	if !sawResult {
		t.Fatal("unterminated final result line was dropped — billing regression")
	}
}

// TestProduce_MissingStdout_NoOp: a phase that never produced a stdout log is
// not an error (it simply contributes no events), and no events file appears.
func TestProduce_MissingStdout_NoOp(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := Produce(ProduceConfig{Workspace: ws, Phase: "audit", CLI: "claude-p", Cycle: 3}); err != nil {
		t.Fatalf("Produce on missing stdout should be nil, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, "audit-events.ndjson")); !os.IsNotExist(err) {
		t.Fatal("no events file should be written when stdout log is absent")
	}
}

// TestProduce_NoTempLeftBehind: the atomic temp file is not left in the
// workspace (it would pollute the *-events.ndjson glob otherwise).
func TestProduce_NoTempLeftBehind(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRaw(t, ws, "tdd-stdout.log", `{"type":"result","total_cost_usd":0.1,"usage":{}}`+"\n")
	if err := Produce(ProduceConfig{Workspace: ws, Phase: "tdd", CLI: "claude-p", Cycle: 2}); err != nil {
		t.Fatalf("Produce: %v", err)
	}
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
