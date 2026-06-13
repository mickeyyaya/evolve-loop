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
		// Cycle-144: a ChatGPT-account codex auditor hit its quota mid-audit. The
		// actual banner ("You've hit your usage limit. Upgrade to Plus to
		// continue…") did NOT match the original (usage|rate)[ -]limit
		// (reached|exceeded|hit) regex ("hit" precedes "usage limit"), so codex
		// sat at the message until the artifact-wait deadline (generic exit 81)
		// instead of escalating. Must now fail fast.
		{"codex usage-limit ChatGPT quota → escalate", "codex-tmux",
			"■ You've hit your usage limit. Upgrade to Plus to continue using Codex (https://chatgpt.com/explore/plus), or try again at Jun 4th, 2026 3:45 PM.",
			"escalate:rate_limit", 85},
		// Negative guard (cycle-144 review HIGH): the real banner is caught by
		// "hit your usage limit", so we deliberately do NOT match a bare
		// "Upgrade to Plus to continue" — an agent echoing pricing/doc text with
		// that phrase (but no limit reached) must stay noop, not falsely abort.
		{"codex generic upgrade CTA (no limit) → noop", "codex-tmux",
			"Doc excerpt: 'Upgrade to Plus to continue using advanced features' — noted for the pricing section.",
			"noop", 0},
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
			gotAction, gotRC := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{}, false)
			if gotAction != tc.wantAction || gotRC != tc.wantRC {
				t.Fatalf("decide[%s] on %q\n  = (%q, %d)\n  want (%q, %d)",
					tc.cli, tc.pane, gotAction, gotRC, tc.wantAction, tc.wantRC)
			}
		})
	}
}

// TestAutoRespond_TrustPromptFiresOnce reproduces the codex-tmux loop-guard
// abandon (exit 86) and pins its fix. A boot-time trust dialog is dismissed by
// one `1,Enter`, but the artifact-wait loop re-captures bootScrollback=200 lines
// every poll — so the DISMISSED dialog text lingers in scrollback and re-matches
// trust_prompt on every subsequent tick. Without a fire-once guard the responder
// keeps re-sending `1,Enter` until counts>5 trips the loop guard and kills the
// run. With `"once": true` on the trust rule, it auto-responds exactly once and
// then noops on later ticks (sharing the live counts map, as ar.counts does
// across the boot→wait phases). The intentional cycle-121 disjuncts are
// untouched — this fixes the re-fire, not the detection.
func TestAutoRespond_TrustPromptFiresOnce(t *testing.T) {
	for _, cli := range []string{"codex-tmux", "agy-tmux"} {
		t.Run(cli, func(t *testing.T) {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", cli, err)
			}
			// A real trust dialog (matches a specific question disjunct, not just
			// the generic affirmative).
			pane := "Do you trust the contents of this directory?\n  1. Yes, continue\n  2. No, exit"
			if cli == "agy-tmux" {
				pane = "Do you trust the contents of this project?\n  1. Yes\n  2. No"
			}
			counts := map[string]int{} // shared across ticks, like the live ar.counts

			// First tick: the real dialog → auto-respond once.
			a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
			if a == "noop" || rc == 0 {
				t.Fatalf("%s first tick must auto-respond to the trust dialog; got (%q,%d)", cli, a, rc)
			}

			// Later ticks: the dialog is dismissed but its text lingers in the
			// captured scrollback. A fire-once trust rule must NOT re-fire (rc 0,
			// no keys) and must never reach the loop guard (rc 86). It surfaces the
			// distinct `suppress_once:` sentinel (rc 0) so the caller WARNs once
			// rather than silently skipping.
			for i := 0; i < 8; i++ {
				a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
				if rc != 0 || a != "suppress_once:trust_prompt" {
					t.Fatalf("%s tick %d = (%q,%d), want (suppress_once:trust_prompt, 0) — trust is fire-once; re-firing trips the loop guard and abandons the run", cli, i+2, a, rc)
				}
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
	action, rc := decideAutoRespond(multiPane, m.InteractivePrompts, map[string]int{}, false)
	if action != "send:Enter,Right,Enter" || rc != 1 {
		t.Fatalf("multi-select pane (footer shared with single-select) = (%q,%d); "+
			"want send:Enter,Right,Enter — askuserquestion_multiselect must precede askuserquestion_select",
			action, rc)
	}
}

// TestAutoRespond_CodexPerEditApprovalRegex covers each disjunct of the
// cycle-124 G1b regex in isolation. The regex is alternation —
// "Would you like to make the following edits|Press enter to confirm or
// esc to cancel|Yes, proceed" — so any of the three strings as a
// substring should fire `send:1,Enter`. Pinning each branch separately
// catches a regression where someone narrows the regex (e.g., to require
// all three substrings).
func TestAutoRespond_CodexPerEditApprovalRegex(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, pane, wantAction string
		wantRC                 int
	}{
		// Each disjunct alone — the regex must fire on any one of them.
		{
			"branch 1: 'Would you like to make the following edits' alone",
			"Working... Would you like to make the following edits to foo.go?",
			"send:1,Enter", 1,
		},
		{
			"branch 2: 'Press enter to confirm or esc to cancel' alone",
			"...some context...\nPress enter to confirm or esc to cancel\n",
			"send:1,Enter", 1,
		},
		{
			"branch 3: 'Yes, proceed' alone (in option list)",
			"What now?\n  1. Yes, proceed\n  2. Refuse",
			"send:1,Enter", 1,
		},
		// Full cycle-123 modal text — all 3 branches present.
		{
			"full modal: all three branches present (cycle-123 reproduction)",
			"Would you like to make the following edits?\n  1. Yes, proceed\n  2. Yes, and don't ask again for these files\n  3. No, and tell Codex what to do differently\n\nPress enter to confirm or esc to cancel",
			"send:1,Enter", 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotAction, gotRC := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{}, false)
			if gotAction != tc.wantAction || gotRC != tc.wantRC {
				t.Errorf("decide on %q\n  = (%q, %d)\n  want (%q, %d)", tc.pane, gotAction, gotRC, tc.wantAction, tc.wantRC)
			}
		})
	}
}

// TestAutoRespond_CodexPerEditApproval_PartialDoesNotMatch is the negative
// counterpart: a TRUNCATED version of any disjunct must NOT match. The
// regex requires the full substring. A pane that says "Press enter to
// confirm" alone (without "or esc to cancel") must not fire the per-edit
// auto-response — that text appears in many other CLI prompts.
func TestAutoRespond_CodexPerEditApproval_PartialDoesNotMatch(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, pane string
	}{
		{
			"partial branch 1: 'Would you like to' alone (incomplete)",
			"Would you like to know more? (yes/no)",
		},
		{
			"partial branch 2: 'Press enter to confirm' without 'or esc to cancel'",
			"Press enter to confirm.",
		},
		{
			"partial branch 3: case-mismatched 'yes, proceed' (lowercase)",
			"Result: yes, proceed.",
		},
		{
			"unrelated text — no branch matches",
			"Working on it... please wait.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotAction, gotRC := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{}, false)
			if gotAction != "noop" || gotRC != 0 {
				t.Errorf("partial pane %q must be a noop; got (%q, %d)", tc.pane, gotAction, gotRC)
			}
		})
	}
}

// TestAutoRespond_CodexTrustWinsOverPerEditOnOverlap pins the manifest
// rule ordering: trust_prompt is declared BEFORE per_edit_approval in
// codex-tmux.json. If a pane contains BOTH "Yes, continue" (trust) AND
// "Yes, proceed" (per-edit) — unlikely but possible if codex chains
// dialogs — the first-match-wins semantics should pick trust. Both rules
// emit the same response_keys ("1,Enter"), so behaviorally this is a
// no-op even if ordering changed — but pinning it locks in the order so
// a future manifest reorder doesn't silently shift the auditable
// "pattern=trust_prompt" log line to "pattern=per_edit_approval".
func TestAutoRespond_CodexTrustWinsOverPerEditOnOverlap(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	// Find the indices of each rule by name.
	trustIdx, perEditIdx := -1, -1
	for i, p := range m.InteractivePrompts {
		switch p.Name {
		case "trust_prompt":
			trustIdx = i
		case "per_edit_approval":
			perEditIdx = i
		}
	}
	if trustIdx == -1 {
		t.Fatal("trust_prompt rule missing from codex-tmux manifest")
	}
	if perEditIdx == -1 {
		t.Fatal("per_edit_approval rule missing from codex-tmux manifest")
	}
	if trustIdx >= perEditIdx {
		t.Errorf("trust_prompt (idx=%d) MUST precede per_edit_approval (idx=%d) so first-match resolves correctly", trustIdx, perEditIdx)
	}

	// Behavioral assertion: a pane with BOTH texts still resolves to
	// "send:1,Enter" (both rules emit it). The win-by-ordering is a
	// log-attribution invariant; this asserts the response is unchanged.
	mixed := "Working with untrusted contents — Yes, continue\nWould you like to make the following edits?\n  1. Yes, proceed"
	gotAction, gotRC := decideAutoRespond(mixed, m.InteractivePrompts, map[string]int{}, false)
	if gotAction != "send:1,Enter" || gotRC != 1 {
		t.Errorf("overlap pane = (%q, %d); want (send:1,Enter, 1)", gotAction, gotRC)
	}
}

// TestAutoRespond_CodexPerEditApproval_AgentOutputFalseMatchGuard documents
// the known footgun: the per_edit_approval regex matches "Yes, proceed"
// as a substring, so an agent that PRINTS the literal phrase in its
// output (e.g., echoing a config option name, or in a grep result over a
// test fixture) would false-match. The loop_guard handles this — repeated
// failed responses without pane progression escalate to abandon — but
// the FIRST match still tries to respond. This test pins that contract:
// false-match IS expected, and the safety net is loop_guard (covered by
// other tests). A future redesign would need anchoring like
// "^.*1\\. Yes, proceed.*$" to be safer; tracked as a follow-up.
func TestAutoRespond_CodexPerEditApproval_AgentOutputFalseMatchGuard(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	// Agent output that mentions "Yes, proceed" as a string literal in
	// what looks like a code grep — exactly the cycle-113-style false
	// match pattern. We expect this to STILL fire send:1,Enter (it's
	// the documented contract; the safety net is loop_guard).
	pane := `Bash(grep -rn '"Yes, proceed"' internal/) ` + "\n" +
		`  cmd/codex_test.go:12:  Body: "Yes, proceed"` + "\n" +
		`  bridge/codex_test.go:45: "Yes, proceed",`
	gotAction, gotRC := decideAutoRespond(pane, m.InteractivePrompts, map[string]int{}, false)
	if gotAction != "send:1,Enter" || gotRC != 1 {
		t.Logf("DOCUMENTED FOOTGUN: agent code-grep mentioning %q matches per_edit_approval; got (%q, %d); want (send:1,Enter, 1)",
			"Yes, proceed", gotAction, gotRC)
		t.Logf("If this test changes behavior to 'noop', tighten the per_edit_approval regex to require the option-list context")
		// Use t.Fail() (not t.Fatalf) so the test runs visibly even if the
		// contract intentionally shifts.
		t.Fail()
	}
}
