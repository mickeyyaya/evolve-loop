package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

func TestDecideAutoRespond_IdleGatesEscalateWhileBusy(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}

	busyPane := "+\tpane := \"You've hit your usage limit. Upgrade to Pro\"\nWorking (1m 18s · esc to interrupt)"
	busy := panestream.PaneBusy(busyPane, panestream.Profiles["codex"])
	if !busy {
		t.Fatal("fixture invalid: the Working/esc-to-interrupt frame must read busy")
	}
	if a, rc := decideAutoRespond(busyPane, m.InteractivePrompts, map[string]int{}, busy); rc == 85 {
		t.Errorf("busy pane must NOT escalate (agent quoting a banner): got %q/%d", a, rc)
	}

	idlePane := "■ You've hit your usage limit. Upgrade to Plus to continue, or try again at Jun 4th 3:45 PM."
	idle := panestream.PaneBusy(idlePane, panestream.Profiles["codex"])
	if idle {
		t.Fatal("fixture invalid: the idle banner must NOT read busy")
	}
	if a, rc := decideAutoRespond(idlePane, m.InteractivePrompts, map[string]int{}, idle); rc != 85 {
		t.Errorf("idle pane with real banner must escalate: got %q/%d", a, rc)
	}

	// An auto_respond prompt carries its own "esc to cancel", so it reads busy yet must still fire:
	// the gate is escalate-only.
	approvalPane := "Would you like to make the following edits?\n  1. Yes, proceed\n  2. Yes, and don't ask again\n  3. No\n\nPress enter to confirm or esc to cancel"
	if !panestream.PaneBusy(approvalPane, panestream.Profiles["codex"]) {
		t.Fatal("fixture invalid: the approval modal carries an esc-to-cancel affordance, must read busy")
	}
	if a, rc := decideAutoRespond(approvalPane, m.InteractivePrompts, map[string]int{}, true); rc != 1 {
		t.Errorf("auto_respond approval must still fire while busy: got %q/%d", a, rc)
	}
}

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
			a, rc := decideAutoRespond(tc.pane, prompts, map[string]int{}, false)
			if a != tc.wantAction || rc != tc.wantRC {
				t.Fatalf("decide(%q) = (%q,%d), want (%q,%d)", tc.pane, a, rc, tc.wantAction, tc.wantRC)
			}
		})
	}
}

func TestDecideAutoRespond_AgentDiffContentNotChrome(t *testing.T) {
	prompts := []ManifestPrompt{
		{Name: "rate_limit", Regex: `(usage|rate)[ -]limit (reached|exceeded|hit)|hit your (usage|rate) limit|too many requests|quota exceeded`, Policy: "escalate"},
	}
	// The agent's editor shows a numbered diff of a fixture while the footer shows it Working.
	agentEditPane := "" +
		"   223 +\t// fixture: codex usage-limit banner\n" +
		"   224 +\tpane := \"■ You've hit your usage limit. Upgrade to Pro\"\n" +
		"\n• Working (1m 18s · esc to interrupt)\n› Implement {feature}\n"
	if a, rc := decideAutoRespond(agentEditPane, prompts, map[string]int{}, false); rc != 0 {
		t.Fatalf("agent diff content must NOT escalate, got (%q,%d)", a, rc)
	}
	realBanner := "codex\n\n  You've hit your usage limit. try again in 3 hours.\n"
	if a, rc := decideAutoRespond(realBanner, prompts, map[string]int{}, false); rc != 85 {
		t.Fatalf("real rate-limit banner must escalate, got (%q,%d)", a, rc)
	}
	// Mixed pane: only the diff line is stripped, so the real banner still escalates.
	mixedPane := "" +
		"   224 +\tpane := \"■ You've hit your usage limit. Upgrade to Pro\"\n" +
		"You've hit your usage limit. try again in 3 hours.\n"
	if a, rc := decideAutoRespond(mixedPane, prompts, map[string]int{}, false); rc != 85 {
		t.Fatalf("real banner alongside agent diff content must still escalate, got (%q,%d)", a, rc)
	}
}

func TestDecideAutoRespond_BareDiffLineNotChrome(t *testing.T) {
	prompts := []ManifestPrompt{
		{Name: "rate_limit", Regex: `(usage|rate)[ -]limit (reached|exceeded|hit)|hit your (usage|rate) limit|too many requests|quota exceeded`, Policy: "escalate"},
	}
	// The pane is idle, so the busy gate does not apply; only the bare-diff strip can save this.
	bareAdd := "diff --git a/clihealth_test.go b/clihealth_test.go\n" +
		"+\tpane := \"You've hit your usage limit. Upgrade to Pro\"\n"
	if a, rc := decideAutoRespond(bareAdd, prompts, map[string]int{}, false); rc == 85 {
		t.Errorf("bare-diff ADDED line on an idle pane must NOT escalate (agent content): got %q/%d", a, rc)
	}
	bareDel := "-\told := \"quota exceeded for this org\"\n"
	if a, rc := decideAutoRespond(bareDel, prompts, map[string]int{}, false); rc == 85 {
		t.Errorf("bare-diff REMOVED line must NOT escalate (agent content): got %q/%d", a, rc)
	}
	realBanner := "codex\n\n  You've hit your usage limit. try again in 3 hours.\n"
	if a, rc := decideAutoRespond(realBanner, prompts, map[string]int{}, false); rc != 85 {
		t.Errorf("real rate-limit banner must still escalate: got %q/%d", a, rc)
	}
}

func TestDecideAutoRespond_IndentedBareDiffLineNotChrome(t *testing.T) {
	prompts := []ManifestPrompt{
		{Name: "rate_limit", Regex: `(usage|rate)[ -]limit (reached|exceeded|hit)|hit your (usage|rate) limit|too many requests|quota exceeded`, Policy: "escalate"},
	}
	cases := []string{
		"  + quoted := \"You've hit your usage limit. Upgrade to Pro\"\n",
		"\t- old := \"quota exceeded while testing clihealth\"\n",
	}
	for _, pane := range cases {
		if a, rc := decideAutoRespond(pane, prompts, map[string]int{}, false); rc == 85 {
			t.Fatalf("indented bare diff content must not escalate: got %q/%d for %q", a, rc, pane)
		}
	}
}

func TestDecideAutoRespond_LoopGuard(t *testing.T) {
	prompts := []ManifestPrompt{{Name: "stuck", Regex: "Please log in", Policy: "escalate"}}
	counts := map[string]int{}
	var a string
	var rc int
	for i := 0; i < 6; i++ { // 6th match: count 6 > 5 → loop guard
		a, rc = decideAutoRespond("Please log in", prompts, counts, false)
	}
	if a != "loop_guard:stuck" || rc != 86 {
		t.Fatalf("after 6 matches = (%q,%d), want (loop_guard:stuck,86)", a, rc)
	}
}

func TestSendKeySequence(t *testing.T) {
	// The multi-keystroke case is the load-bearing one: claude's multi-select needs three distinct presses.
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
	// The REPL boots, then shows an auth-recheck prompt the manifest escalates.
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
	// A model-deprecation prompt that never clears: the 6th match trips the loop guard.
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

func TestDecideAutoRespond_CodexModelUnsupportedIsIdleGated(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	wall := "■ {\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account.\"}}"
	busyPane := wall + "\nWorking (0m 04s · esc to interrupt)"
	if !panestream.PaneBusy(busyPane, panestream.Profiles["codex"]) {
		t.Fatal("fixture invalid: the Working frame must read busy")
	}
	if a, rc := decideAutoRespond(busyPane, m.InteractivePrompts, map[string]int{}, true); rc != 0 {
		t.Errorf("busy pane must not escalate: got %q/%d", a, rc)
	}
	idlePane := wall + "\n\n›"
	if panestream.PaneBusy(idlePane, panestream.Profiles["codex"]) {
		t.Fatal("fixture invalid: the idle prompt must not read busy")
	}
	if a, rc := decideAutoRespond(idlePane, m.InteractivePrompts, map[string]int{}, false); a != "escalate:model_unsupported" || rc != 85 {
		t.Errorf("idle pane with the 400 must escalate model_unsupported/85: got %q/%d", a, rc)
	}
}
