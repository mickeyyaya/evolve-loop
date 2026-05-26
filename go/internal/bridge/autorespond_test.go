package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

// autorespond_test.go — pure decision truth table + key-CSV parsing +
// integration through the claude-tmux driver (escalate → ExitUnknownPrompt;
// a stuck auto_respond prompt → loop guard → ExitRespondLoopGuard).

func TestDecideAutoRespond(t *testing.T) {
	prompts := []ManifestPrompt{
		{Name: "escA", Regex: "Please log in", Policy: "escalate"},
		{Name: "autoB", Regex: `Continue\?`, ResponseKeys: "y,Enter", Policy: "auto_respond"},
		{Name: "extC", Regex: "slow work", ResponseKeys: "60", Policy: "extend_timeout"},
		{Name: "autoNoKeys", Regex: "weird-prompt", Policy: "auto_respond"}, // missing keys → escalate
	}
	cases := []struct {
		name, pane, wantAction string
		wantRC                 int
	}{
		{"noop", "nothing matches here", "noop", 0},
		{"escalate", "Please log in now", "escalate:escA", 85},
		{"send", "Continue?", "send:y,Enter", 1},
		{"extend", "doing slow work now", "extend:60", 2},
		{"auto_respond missing keys → escalate", "weird-prompt!", "escalate:autoNoKeys", 85},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, rc := decideAutoRespond(tc.pane, prompts, map[string]int{})
			if a != tc.wantAction || rc != tc.wantRC {
				t.Fatalf("decide(%q) = (%q,%d), want (%q,%d)", tc.pane, a, rc, tc.wantAction, tc.wantRC)
			}
		})
	}
}

func TestDecideAutoRespond_LoopGuard(t *testing.T) {
	prompts := []ManifestPrompt{{Name: "stuck", Regex: "Please log in", Policy: "escalate"}}
	counts := map[string]int{}
	var a string
	var rc int
	for i := 0; i < 6; i++ { // 6th match: count 6 > 5 → loop guard
		a, rc = decideAutoRespond("Please log in", prompts, counts)
	}
	if a != "loop_guard:stuck" || rc != 86 {
		t.Fatalf("after 6 matches = (%q,%d), want (loop_guard:stuck,86)", a, rc)
	}
}

func TestParseSendKeysCSV(t *testing.T) {
	cases := []struct {
		csv, keys string
		enter     bool
	}{
		{"y,Enter", "y", true},
		{"Enter", "", true},
		{"3,Enter", "3", true},
		{"y", "y", false},
	}
	for _, tc := range cases {
		k, e := parseSendKeysCSV(tc.csv)
		if k != tc.keys || e != tc.enter {
			t.Fatalf("parseSendKeysCSV(%q) = (%q,%v), want (%q,%v)", tc.csv, k, e, tc.keys, tc.enter)
		}
	}
}

func TestClaudeTmux_AutoRespond_EscalateWritesReport(t *testing.T) {
	// REPL boots (❯), then the pane shows an auth-recheck prompt the
	// claude-tmux manifest classifies as escalate → ExitUnknownPrompt + report.
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault, "Please log in to continue"}}
	code, _ := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitUnknownPrompt {
		t.Fatalf("exit = %d, want %d (ExitUnknownPrompt)", code, ExitUnknownPrompt)
	}
	if _, err := os.Stat(filepath.Join(fx.ws, "escalation-report.json")); err != nil {
		t.Fatalf("escalation report should be written: %v", err)
	}
}

func TestClaudeTmux_AutoRespond_StuckPromptTripsLoopGuard(t *testing.T) {
	// A model-deprecation prompt (auto_respond y,Enter) that never clears:
	// the engine sends keys each tick, and the 6th match trips the loop guard.
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault, "this model is deprecated, Continue?"}}
	code, _ := runTmux(t, fx, tmux, nil, "--allow-bypass")
	if code != ExitRespondLoopGuard {
		t.Fatalf("exit = %d, want %d (ExitRespondLoopGuard)", code, ExitRespondLoopGuard)
	}
	if !tmux.sentContains("y") {
		t.Fatalf("auto_respond should have sent keys before the guard tripped; sentKeys=%v", tmux.sentKeys)
	}
}
