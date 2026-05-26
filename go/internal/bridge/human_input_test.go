package bridge

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHumanActive(t *testing.T) {
	simOn := mapLookup(map[string]string{"BRIDGE_HUMAN_SIMULATION": "1"})
	if humanActive(Deps{LookupEnv: simOn}, false) {
		t.Fatal("humanInput=false → inactive")
	}
	if humanActive(Deps{LookupEnv: mapLookup(nil)}, true) {
		t.Fatal("no BRIDGE_HUMAN_SIMULATION → inactive")
	}
	if !humanActive(Deps{LookupEnv: simOn}, true) {
		t.Fatal("both gates → active")
	}
}

func TestHumanSampleMS(t *testing.T) {
	if d := humanSampleMS(65, 20); d < 10*time.Millisecond {
		t.Fatalf("sample = %v, want >= 10ms", d)
	}
	if d := humanSampleMS(0, 0); d < 10*time.Millisecond {
		t.Fatalf("floor sample = %v, want >= 10ms", d)
	}
}

func TestHumanPrimitives(t *testing.T) {
	tmux := &fakeTmux{}
	deps := Deps{Sleep: func(time.Duration) {}, Stderr: io.Discard, Tmux: tmux}

	humanBootPause(deps)

	ws := t.TempDir()
	pf := writeJSON(t, filepath.Join(ws, "p.txt"), "line1\nline2\nline3")
	humanPasteWithReview(context.Background(), deps, "s", pf)
	if len(tmux.sentKeys) == 0 {
		t.Fatal("paste-with-review should send Enter")
	}
	// ReadFile error path (missing file → lines=1, floor review)
	humanPasteWithReview(context.Background(), deps, "s", "/no/such/file-xyz")

	tmux2 := &fakeTmux{}
	deps.Tmux = tmux2
	humanSendKeysCSV(context.Background(), deps, "s", "y,,Enter") // empty token skipped + Enter
	if !tmux2.sentContains("y") {
		t.Fatalf("human send should deliver 'y'; sent=%v", tmux2.sentKeys)
	}

	humanReadingPause(deps, "a b")                       // <3 words → floor
	humanReadingPause(deps, strings.Repeat("word ", 40)) // many words
}

func TestClaudeTmux_HumanInput_GateAndPath(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")

	// Gate: --human-input without BRIDGE_HUMAN_SIMULATION → ExitSafetyGate.
	code, se := runTmux(t, fx, &fakeTmux{}, nil, "--allow-bypass", "--human-input")
	if code != ExitSafetyGate || !strings.Contains(se, "BRIDGE_HUMAN_SIMULATION") {
		t.Fatalf("human-input gate: code=%d se=%q", code, se)
	}

	// Gate passes (sim=1) → human prompt-delivery + human auto-respond send.
	// The pane boots (❯) then shows a model-deprecation auto_respond prompt
	// that never clears → the loop guard trips (exercising the human send path).
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault, "this model is deprecated, Continue?"}}
	code2, se2 := runTmux(t, fx, tmux, map[string]string{"BRIDGE_HUMAN_SIMULATION": "1"}, "--allow-bypass", "--human-input")
	if code2 != ExitRespondLoopGuard {
		t.Fatalf("human path exit = %d, want ExitRespondLoopGuard", code2)
	}
	if !tmux.sentContains("y") {
		t.Fatalf("human auto-respond should send 'y'; sent=%v", tmux.sentKeys)
	}
	if !strings.Contains(se2, "[human-input]") {
		t.Fatalf("human path should log [human-input]; se=%q", se2)
	}
}
