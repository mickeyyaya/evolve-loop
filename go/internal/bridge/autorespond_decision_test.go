package bridge

import "testing"

// autorespond_decision_test.go — the full interactive-prompt decision matrix,
// driven by the REAL embedded manifests (LoadManifest) against REAL observed
// pane text. This is the scenario coverage of "what we learned from each LLM
// CLI in tmux": every auto_respond / escalate / extend / noop branch that the
// production rules must classify, encoded as one truth-table row per scenario.
//
// Pane fixtures are the actual strings captured 2026-05-26 (claude v2.1.150,
// codex v0.133.0, agy 1.0.2) — see knowledge-base/research/tmux-repl-cli-
// behavior-2026-05-26.md. decideAutoRespond is pure (no tmux), so this stays
// deterministic and fast; the real-tmux delivery + live-CLI proofs live in
// tmux_repl_interactive_test.go and tmux_repl_interactive_livecli_test.go.
func TestAutoRespond_RealManifestDecisionMatrix(t *testing.T) {
	cases := []struct {
		name, cli, pane, wantAction string
		wantRC                      int
	}{
		// --- claude-tmux: the dominant CLI; AskUserQuestion menus are the
		// real hangs that --dangerously-skip-permissions does NOT suppress.
		{
			"claude single-select menu → Enter (recommended/first)", "claude-tmux",
			"What's your favorite?\n❯ 1. Alpha\n     Option 1\n  2. Beta\n  3. Gamma\nEnter to select · ↑/↓ to navigate · Esc to cancel",
			"send:Enter", 1,
		},
		{
			"claude multi-select menu → Enter,Right,Enter (toggle→Submit→submit)", "claude-tmux",
			"←  ☐ Toppings  ✔ Submit  →\nWhich toppings would you like?\n❯ 1. [ ] Cheese\n  2. [ ] Mushroom\n  3. [ ] Onion\nEnter to select · ↑/↓ to navigate · Esc to cancel",
			"send:Enter,Right,Enter", 1,
		},
		{
			"claude model-deprecation → y,Enter", "claude-tmux",
			"Warning: this model is deprecated. Continue? (y/n)",
			"send:y,Enter", 1,
		},
		{
			"claude terminal-resize → Enter", "claude-tmux",
			"Terminal too small (80x10). Please resize to continue.",
			"send:Enter", 1,
		},
		{"claude auth-recheck → escalate", "claude-tmux", "Please log in to continue", "escalate:auth_recheck", 85},
		{"claude rate-limit → escalate", "claude-tmux", "Error: rate limit exceeded (429)", "escalate:rate_limit", 85},
		// Regression: an agent grepping rate-limit DETECTION CODE prints the
		// token "rate_limit" all over its pane. That must NOT be mistaken for a
		// real rate-limit banner (the old `rate.?limit` regex false-escalated
		// here, killing cycle 113 mid-research). Underscore token ≠ banner.
		{"claude rate_limit code-grep → noop (no false escalate)", "claude-tmux",
			"Bash(grep -rn \"rate_limit|error_spike|cost_anomaly\" internal/phaseobserver)\n  detection rules (infinite_loop, error_spike, cost_anomaly, rate_limit) emit",
			"noop", 0},
		{"claude normal output → noop", "claude-tmux", "Wrote 3 files; running the test suite now.", "noop", 0},

		// --- codex-tmux: trust dialog on first launch in an untrusted dir.
		{"codex trust → 1,Enter", "codex-tmux", "Do you trust the contents of this directory?", "send:1,Enter", 1},
		{"codex auth → escalate", "codex-tmux", "Please sign in to ChatGPT to continue", "escalate:auth_recheck", 85},
		{"codex rate-limit → escalate", "codex-tmux", "quota exceeded — too many requests", "escalate:rate_limit", 85},
		// Cycle-124 G1b: per-edit-approval modal that hung cycle-123 tdd.
		// '1' selects 'Yes, proceed'. Defense-in-depth behind G1a's --yolo
		// boot flag — covers the case where --yolo is dropped/renamed/
		// overridden. Pane fragment is the actual cycle-123 capture.
		{"codex per-edit-approval → 1,Enter (cycle-124 G1b)", "codex-tmux",
			"Would you like to make the following edits?\n  1. Yes, proceed\n  2. Yes, and don't ask again for these files\n  3. No, and tell Codex what to do differently\n\nPress enter to confirm or esc to cancel",
			"send:1,Enter", 1},

		// --- agy-tmux: trust + a belt-and-suspenders permission prompt.
		{"agy trust → Enter", "agy-tmux", "Do you trust the contents of this project?", "send:Enter", 1},
		{"agy permission → y,Enter", "agy-tmux", "Allow write to /tmp/out.txt?", "send:y,Enter", 1},
		{"agy quota → escalate", "agy-tmux", "You have exceeded your daily limit for the free tier", "escalate:quota_exhausted", 85},
		{"agy rate-limit → escalate", "agy-tmux", "RESOURCE_EXHAUSTED: retry later", "escalate:rate_limit", 85},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := LoadManifest(tc.cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", tc.cli, err)
			}
			// Fresh counts per case so the loop guard never bleeds across rows.
			gotAction, gotRC := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{})
			if gotAction != tc.wantAction || gotRC != tc.wantRC {
				t.Fatalf("decide[%s] on %q\n  = (%q, %d)\n  want (%q, %d)",
					tc.cli, tc.pane, gotAction, gotRC, tc.wantAction, tc.wantRC)
			}
		})
	}
}

// TestAutoRespond_MultiSelectRuleWinsOverSingleSelect guards the manifest
// ordering invariant: a multi-select pane contains BOTH the checkbox markers
// and the single-select footer, so the checkbox rule MUST be listed first or
// the bridge would send a bare Enter (toggling a checkbox) instead of the full
// Enter,Right,Enter submit sequence — leaving the REPL stuck mid-menu.
func TestAutoRespond_MultiSelectRuleWinsOverSingleSelect(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	multiPane := "❯ 1. [ ] Cheese\n  2. [ ] Mushroom\nEnter to select · ↑/↓ to navigate · Esc to cancel"
	action, rc := decideAutoRespond(multiPane, m.InteractivePrompts, map[string]int{})
	if action != "send:Enter,Right,Enter" || rc != 1 {
		t.Fatalf("multi-select pane (footer shared with single-select) = (%q,%d); "+
			"want send:Enter,Right,Enter — askuserquestion_multiselect must precede askuserquestion_select",
			action, rc)
	}
}
