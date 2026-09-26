package bridge

import (
	"os"
	"strings"
	"testing"
)

// Pane fixtures are verbatim captures from claude v2.1.150, codex v0.133.0 and agy 1.0.2.
func TestAutoRespond_RealManifestDecisionMatrix(t *testing.T) {
	cases := []struct {
		name, cli, pane, wantAction string
		wantRC                      int
	}{
		// --- claude-tmux: AskUserQuestion menus hang even under --dangerously-skip-permissions.
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
		// An agent grepping detection code prints the token "rate_limit"; that is not a banner.
		{"claude rate_limit code-grep → noop (no false escalate)", "claude-tmux",
			"Bash(grep -rn \"rate_limit|error_spike|cost_anomaly\" internal/phaseobserver)\n  detection rules (infinite_loop, error_spike, cost_anomaly, rate_limit) emit",
			"noop", 0},
		{"claude normal output → noop", "claude-tmux", "Wrote 3 files; running the test suite now.", "noop", 0},

		// --- codex-tmux: trust dialog on first launch in an untrusted dir.
		{"codex trust → 1,Enter", "codex-tmux", "Do you trust the contents of this directory?", "send:1,Enter", 1},
		{"codex auth → escalate", "codex-tmux", "Please sign in to ChatGPT to continue", "escalate:auth_recheck", 85},
		{"codex rate-limit → escalate", "codex-tmux", "quota exceeded — too many requests", "escalate:rate_limit", 85},
		// A rejected model is a wall like a quota wall: escalate at once so the family fallback runs in seconds.
		{"codex model-unsupported 400 → escalate", "codex-tmux",
			"⚠ Model metadata for gpt-5.6-sol not found. Using default model metadata.\n■ {\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The 'gpt-5.6-sol' model is not supported when using Codex with a ChatGPT account.\"}}\n\n›",
			"escalate:model_unsupported", 85},
		// The same JSON quoted by an agent reading a write-up must not fire: the rule matches the
		// pane tail only, and the busy gate holds.
		{"codex model-unsupported quoted far above the tail → noop", "codex-tmux",
			"■ {\"type\":\"error\",\"status\":400,\"error\":{\"type\":\"invalid_request_error\",\"message\":\"The 'x' model is not supported when using Codex with a ChatGPT account.\"}}\n" + strings.Repeat("  reading docs/incidents/...\n", 40) + "›",
			"noop", 0},
		// The real codex banner puts "hit" before "usage limit"; it must fail fast.
		{"codex usage-limit ChatGPT quota → escalate", "codex-tmux",
			"■ You've hit your usage limit. Upgrade to Plus to continue using Codex (https://chatgpt.com/explore/plus), or try again at Jun 4th, 2026 3:45 PM.",
			"escalate:rate_limit", 85},
		// A bare "Upgrade to Plus to continue" is pricing text an agent may echo; it must stay noop.
		{"codex generic upgrade CTA (no limit) → noop", "codex-tmux",
			"Doc excerpt: 'Upgrade to Plus to continue using advanced features' — noted for the pricing section.",
			"noop", 0},
		// codex's per-edit approval modal: '1' selects 'Yes, proceed'. Defense in depth behind the --yolo boot flag.
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

			a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
			if a == "noop" || rc == 0 {
				t.Fatalf("%s first tick must auto-respond to the trust dialog; got (%q,%d)", cli, a, rc)
			}

			for i := 0; i < 8; i++ {
				a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
				if rc != 0 || a != "suppress_once:trust_prompt" {
					t.Fatalf("%s tick %d = (%q,%d), want (suppress_once:trust_prompt, 0) — trust is fire-once; re-firing trips the loop guard and abandons the run", cli, i+2, a, rc)
				}
			}
		})
	}
}

func TestAutoRespond_FeedbackRatingFiresOnce(t *testing.T) {
	for _, cli := range []string{"claude-tmux", "agy-tmux"} {
		t.Run(cli, func(t *testing.T) {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", cli, err)
			}
			pane := "How's the CLI experience so far?\n  Rate 0-9 then Enter"
			counts := map[string]int{} // shared across ticks, like the live ar.counts

			a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
			if a == "noop" || rc == 0 {
				t.Fatalf("%s first tick must auto-respond to the feedback prompt; got (%q,%d)", cli, a, rc)
			}

			for i := 0; i < 8; i++ {
				a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
				if rc != 0 || a != "suppress_once:cli_feedback_rating" {
					t.Fatalf("%s tick %d = (%q,%d), want (suppress_once:cli_feedback_rating, 0) — feedback rating is fire-once; re-firing trips the loop guard and abandons the run", cli, i+2, a, rc)
				}
			}
		})
	}
}

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

func TestAutoRespond_CodexPerEditApprovalRegex(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, pane, wantAction string
		wantRC                 int
	}{
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
		// The full modal text, with all three disjuncts.
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

// Both rules send "1,Enter", so the order only decides which pattern the log names; pinning it keeps
// that attribution stable.
func TestAutoRespond_CodexTrustWinsOverPerEditOnOverlap(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
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

	mixed := "Working with untrusted contents — Yes, continue\nWould you like to make the following edits?\n  1. Yes, proceed"
	gotAction, gotRC := decideAutoRespond(mixed, m.InteractivePrompts, map[string]int{}, false)
	if gotAction != "send:1,Enter" || gotRC != 1 {
		t.Errorf("overlap pane = (%q, %d); want (send:1,Enter, 1)", gotAction, gotRC)
	}
}

// Pins a known footgun: an agent printing "Yes, proceed" still fires once; the loop guard is the safety net.
func TestAutoRespond_CodexPerEditApproval_AgentOutputFalseMatchGuard(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatal(err)
	}
	pane := `Bash(grep -rn '"Yes, proceed"' internal/) ` + "\n" +
		`  cmd/codex_test.go:12:  Body: "Yes, proceed"` + "\n" +
		`  bridge/codex_test.go:45: "Yes, proceed",`
	gotAction, gotRC := decideAutoRespond(pane, m.InteractivePrompts, map[string]int{}, false)
	if gotAction != "send:1,Enter" || gotRC != 1 {
		t.Logf("DOCUMENTED FOOTGUN: agent code-grep mentioning %q matches per_edit_approval; got (%q, %d); want (send:1,Enter, 1)",
			"Yes, proceed", gotAction, gotRC)
		t.Logf("If this test changes behavior to 'noop', tighten the per_edit_approval regex to require the option-list context")
		// t.Fail, not t.Fatalf, so both log lines print when the contract shifts.
		t.Fail()
	}
}

func TestAutoRespond_ClaudeTrustDialog_v2252(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	// Verbatim live capture from claude 2.1.252.
	pane := ` Accessing workspace:
 /private/var/folders/11/n1_42bt961s29wjcr9qxj45m0000gn/T/tmp.j6PN2iejzV
 Quick safety check: Is this a project you created or one you trust? (Like your own code, a well-known open source project, or work from your team). If not, take a moment to review what's in this
 folder first.
 Claude Code'll be able to read, edit, and execute files here.
 Security guide
 ❯ No, exit
   Yes, I trust this folder
 Enter to confirm · Esc to cancel`
	counts := map[string]int{}
	a, rc := decideAutoRespond(pane, m.InteractivePrompts, counts, false)
	if a != "send:Down,Enter" || rc != 1 {
		t.Fatalf("2.1.252 trust dialog must answer Down,Enter (select 'Yes, I trust this folder' off the No-default); got (%q,%d)", a, rc)
	}
	// Pin the exact sentinel, not just the suppress_once: prefix, so a rule rename fails loudly.
	for i := 0; i < 8; i++ {
		a, rc = decideAutoRespond(pane, m.InteractivePrompts, counts, false)
		if rc != 0 || a != "suppress_once:trust_prompt_no_default" {
			t.Fatalf("tick %d = (%q,%d), want (suppress_once:trust_prompt_no_default, 0) — trust_prompt_no_default is fire-once; re-firing trips the loop guard and abandons the run", i+2, a, rc)
		}
	}
	// Older claude builds stay launchable, so the numbered-dialog rule must still win on the old pane.
	oldPane := "Quick safety check: Is this a project you created or one you trust?\n ❯ 1. Yes, I trust this folder\n   2. No, exit\n Enter to confirm"
	a, rc = decideAutoRespond(oldPane, m.InteractivePrompts, map[string]int{}, false)
	if a != "send:Enter" || rc != 1 {
		t.Fatalf("the v2.1.193 numbered dialog must keep its Enter response; got (%q,%d)", a, rc)
	}
}

// This repo quotes both trust dialogs verbatim in tracked files an agent reads. The bottom anchor
// (footer, \z and tail_lines) keeps a quoted dialog from firing mid-document.
func TestAutoRespond_TrustRulesDoNotMatchThisRepositorysOwnFiles(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	var trustRules []ManifestPrompt
	for _, p := range m.InteractivePrompts {
		if strings.HasPrefix(p.Name, "trust_prompt") {
			trustRules = append(trustRules, p)
		}
	}
	if len(trustRules) < 2 {
		t.Fatalf("expected both trust rules under test, got %d", len(trustRules))
	}
	files := []string{
		"autorespond_decision_test.go", // both pane fixtures verbatim
		"driver_claudetmux_test.go",    // the boot-path dialog fixtures
		"manifests/claude-tmux.json",   // the rules and their notes
		"../../../docs/incidents/2026-09-01-claude-2252-trust-default-flip.md",
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v (this guard is worthless if it cannot read its own sources)", f, err)
		}
		a, rc := decideAutoRespond(string(body), trustRules, map[string]int{}, false)
		if rc != 0 || a != "noop" {
			t.Fatalf("trust rule fired on rendered tracked file %s: (%q,%d) — the bottom anchor regressed", f, a, rc)
		}
	}
}
