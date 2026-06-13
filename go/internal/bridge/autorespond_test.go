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

// TestDecideAutoRespond_IdleGatesEscalateWhileBusy pins the ADR-0047 state-gate
// (cycle-314): a policy=escalate match while the CLI is BUSY is the agent
// QUOTING the banner in its output, not the CLI's own chrome — it must NOT
// escalate/bench. A real banner on an IDLE pane still escalates. This catches
// the residual the diff-line strip missed: a BARE (unnumbered) "+\t..." edit
// line carrying the quoted banner.
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

	// Scoping invariant: an auto_respond prompt (per-edit-approval) carries its
	// own "esc to cancel" affordance → PaneBusy=true, yet it MUST still fire —
	// the gate suppresses escalate only. Guards against a future widening of the
	// gate condition silently swallowing approval modals.
	approvalPane := "Would you like to make the following edits?\n  1. Yes, proceed\n  2. Yes, and don't ask again\n  3. No\n\nPress enter to confirm or esc to cancel"
	if !panestream.PaneBusy(approvalPane, panestream.Profiles["codex"]) {
		t.Fatal("fixture invalid: the approval modal carries an esc-to-cancel affordance, must read busy")
	}
	if a, rc := decideAutoRespond(approvalPane, m.InteractivePrompts, map[string]int{}, true); rc != 1 {
		t.Errorf("auto_respond approval must still fire while busy: got %q/%d", a, rc)
	}
}

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
			a, rc := decideAutoRespond(tc.pane, prompts, map[string]int{}, false)
			if a != tc.wantAction || rc != tc.wantRC {
				t.Fatalf("decide(%q) = (%q,%d), want (%q,%d)", tc.pane, a, rc, tc.wantAction, tc.wantRC)
			}
		})
	}
}

// TestDecideAutoRespond_AgentDiffContentNotChrome pins the soak-#4 cycle-314
// false positive: the codex agent editing the clihealth package (the
// rate-limit PARSER) types a test fixture containing "You've hit your usage
// limit" in a numbered-diff line; the escalate rule matched that agent
// content and benched codex 30min on a false rate-limit. CLI rate-limit
// chrome is never a numbered-diff line — agent diff content must be excluded
// from escalate-pattern matching, while a real banner still escalates.
func TestDecideAutoRespond_AgentDiffContentNotChrome(t *testing.T) {
	prompts := []ManifestPrompt{
		{Name: "rate_limit", Regex: `(usage|rate)[ -]limit (reached|exceeded|hit)|hit your (usage|rate) limit|too many requests|quota exceeded`, Policy: "escalate"},
	}
	// The exact cycle-314 shape: the agent's editor shows a numbered diff of
	// clihealth_test.go, and the footer shows it actively Working.
	agentEditPane := "" +
		"   223 +\t// fixture: codex usage-limit banner\n" +
		"   224 +\tpane := \"■ You've hit your usage limit. Upgrade to Pro\"\n" +
		"\n• Working (1m 18s · esc to interrupt)\n› Implement {feature}\n"
	if a, rc := decideAutoRespond(agentEditPane, prompts, map[string]int{}, false); rc != 0 {
		t.Fatalf("agent diff content must NOT escalate, got (%q,%d)", a, rc)
	}
	// A real codex rate-limit banner (CLI chrome, not a diff line) must still
	// escalate.
	realBanner := "codex\n\n  You've hit your usage limit. try again in 3 hours.\n"
	if a, rc := decideAutoRespond(realBanner, prompts, map[string]int{}, false); rc != 85 {
		t.Fatalf("real rate-limit banner must escalate, got (%q,%d)", a, rc)
	}
	// Mixed pane: the agent is editing the fixture AND a real banner appears
	// on a non-diff line. The strip removes only the diff line, so the real
	// banner still escalates — the fix never suppresses genuine chrome.
	mixedPane := "" +
		"   224 +\tpane := \"■ You've hit your usage limit. Upgrade to Pro\"\n" +
		"You've hit your usage limit. try again in 3 hours.\n"
	if a, rc := decideAutoRespond(mixedPane, prompts, map[string]int{}, false); rc != 85 {
		t.Fatalf("real banner alongside agent diff content must still escalate, got (%q,%d)", a, rc)
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
