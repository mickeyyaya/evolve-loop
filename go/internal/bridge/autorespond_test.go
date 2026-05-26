package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSendKeySequence(t *testing.T) {
	// Each token becomes its own ordered keystroke ("keys|enter"). The
	// multi-keystroke case is the load-bearing one: claude's multi-select
	// needs Enter (toggle) → Right (to Submit) → Enter (submit) as three
	// distinct presses, which the old (keys,enter) collapse could not express.
	cases := []struct {
		name, csv string
		want      []string
	}{
		{"single key + enter", "y,Enter", []string{"y|false", "|true"}},
		{"bare enter", "Enter", []string{"|true"}},
		{"digit + enter", "3,Enter", []string{"3|false", "|true"}},
		{"key only, no enter", "y", []string{"y|false"}},
		{"multi-keystroke sequence", "Enter,Right,Enter", []string{"|true", "Right|false", "|true"}},
		{"empty tokens skipped", "y,,Enter", []string{"y|false", "|true"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &fakeTmux{}
			// no-op Sleep so the inter-key pacing doesn't slow the unit test.
			deps := Deps{Tmux: rec, Sleep: func(time.Duration) {}}.withDefaults()
			sendKeySequence(context.Background(), deps, "s", tc.csv)
			if strings.Join(rec.sentSeq, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("sendKeySequence(%q) = %v, want %v", tc.csv, rec.sentSeq, tc.want)
			}
		})
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
