package bridge

import (
	"os"
	"strings"
	"testing"
)

// autorespond_planmode_test.go — plan-mode dialogs, the one interactive class
// the responder was blind to.
//
// Both CLIs can reach a blocking plan-mode dialog, and no manifest rule matched
// either. Captured live 2026-08-27 (claude v2.x, codex-cli 0.147.0):
//
//   - claude: an agent called EnterPlanMode from a --dangerously-skip-permissions
//     session — the footer flipped "⏵⏵ bypass permissions on" → "⏸ plan mode on" —
//     and ExitPlanMode raised the approval dialog. Bypass does NOT prevent plan mode.
//   - codex: collaboration_modes is graduated-on; /plan engages "Plan mode" and
//     the agent asks clarifying questions through a blocking picker.
//
// Zero occurrences across 384 retained run dirs (cycles 1217-1574), so these
// rules are a net for a reachable hole, not a fix for an observed burn.
//
// TWO FIXTURE FACTS THAT DROVE THE RULE DESIGN, both found by probing rather
// than by reading:
//
//  1. claude's option 1 text VARIES by launch mode ("Yes, and use auto mode"
//     under --permission-mode plan; "Yes, and switch to BYPASS PERMISSIONS…"
//     under bypass). A rule anchored on option 1 passes a test written from one
//     capture and fails silently in production. Both variants are pinned.
//  2. The live dialog is always at the BOTTOM of the capture (measured: 155
//     chars of tail on claude, 47 on codex, one trailing newline). Both rules
//     are therefore bottom-anchored, which is what makes them safe — see
//     TestAutoRespond_PlanModeDialogs' noop rows.

const claudePlanApprovalBypassPane = `   2. hello.go
   package main — a runnable command, matching the "runnable slice" scope.
  ────────────────────────────────────────────────────────────────────────────
   Claude has written up a plan and is ready to execute. Would you like to proceed?
   ❯ 1. Yes, and switch to BYPASS PERMISSIONS (no further prompts) for this session
     2. Yes, manually approve edits
     3. Tell Claude what to change
        shift+tab to approve with this feedback
   ctrl+g to edit in Vim · ~/.claude/plans/staged-beaming-donut.md`

const claudePlanApprovalPlanModePane = `   1. go.mod
   Module root so Go tooling works.
  ────────────────────────────────────────────────────────────────────────────
   Claude has written up a plan and is ready to execute. Would you like to proceed?
   ❯ 1. Yes, and use auto mode
     2. Yes, manually approve edits
     3. Tell Claude what to change
        shift+tab to approve with this feedback
   ctrl+g to edit in Vim · ~/.claude/plans/use-plan-mode-propose-stateful-pudding.md`

const codexPlanQuestionPane = `• The workspace is effectively empty: there is no hello.go, go.mod, Go package, or existing test convention.
  Question 1/2 (2 unanswered)
  What should the no-argument ` + "`greet()`" + ` function do?
  › 1. Return "hello" (Recommended)  Implements ` + "`func greet() string`" + ` using the repository's only existing greeting text.
    2. Print "hello"                 Implements ` + "`func greet()`" + ` with stdout as its observable behavior.
    3. Return "Hello, World!"        Implements the common kata-style string contract.
    4. None of the above             Optionally, add details in notes (tab).
  tab to add notes | enter to submit answer | ←/→ to navigate questions | esc to interrupt`

// TestAutoRespond_PlanModeDialogs is the truth table: live dialogs respond,
// everything else noops. Positive and negative rows share one runner because
// they assert the same function over the same inputs — the package's own
// decision matrix mixes them the same way.
func TestAutoRespond_PlanModeDialogs(t *testing.T) {
	cases := []struct {
		name, cli, pane, wantAction string
		wantRC                      int
		paneBusy                    bool
	}{
		// --- live dialogs: respond ---
		// Enter accepts the pre-highlighted option, an approval in both
		// variants. An autonomous loop has no human to read the plan and the
		// auditor grades the resulting diff regardless, so approving beats a
		// dead lane. Escalating would abandon the phase over a dialog whose
		// only correct answer is "yes"; extending the deadline buys time a
		// BLOCKED agent cannot use.
		{"claude plan approval (bypass launch)", "claude-tmux",
			claudePlanApprovalBypassPane, "send:Enter", 1, false},
		{"claude plan approval (plan-mode launch, different option 1)", "claude-tmux",
			claudePlanApprovalPlanModePane, "send:Enter", 1, false},
		// codex's picker footer carries "esc to interrupt", so the real pane
		// reads BUSY. Decided under the production input, not a convenient one.
		{"codex plan question (pane reads busy in production)", "codex-tmux",
			codexPlanQuestionPane, "send:Enter", 1, true},

		// --- the bottom anchor: scrollback that scrolled up must NOT fire ---
		// This is the rule's load-bearing safety property. A dialog that has
		// been answered, or one quoted in a document an agent is reading, sits
		// ABOVE later output and therefore cannot match \z. Without this, an
		// answered dialog lingering in the capture re-fires every 2s poll and
		// trips the loop guard (limit 5) — killing the lane in ~10s. That is
		// not hypothetical: codex-tmux.json records exactly that abandon on
		// 2026-06-03 from a dismissed trust dialog at bootScrollback=200.
		// ONE line of new output is the boundary that matters, and it is the
		// boundary an earlier revision of these rules FAILED: with a
		// `.{0,N}\s*\z` tail, `.` already matches newlines under (?s), so the
		// bound absorbed real output and the rule re-fired on a dismissed
		// dialog at 1-3 trailing lines. Those tests passed only because their
		// 3-4 line trailers happened to also cross the TailLines window — the
		// right answer for the wrong reason. The tails are now anchored to each
		// dialog's OWN final line (claude's `ctrl+g … <plan>.md`; codex's
		// footer via `[^\n]*`, which cannot cross a newline), so ANY new line
		// below the dialog ends the match structurally.
		{"claude: answered dialog, ONE line of output below", "claude-tmux",
			claudePlanApprovalBypassPane + "\n● Writing hello.go…", "noop", 0, false},
		{"claude: answered dialog, two lines below", "claude-tmux",
			claudePlanApprovalBypassPane + "\n● Writing hello.go…\n  ⎿ wrote 12 lines", "noop", 0, false},
		{"codex: answered picker, ONE line of output below", "codex-tmux",
			codexPlanQuestionPane + "\n› ", "noop", 0, true},
		{"codex: answered picker, two lines below", "codex-tmux",
			codexPlanQuestionPane + "\n• Ran go build ./...\n  └ ok", "noop", 0, true},

		// --- prose/paraphrase must not fire (the cycle-314 class) ---
		{"claude: prose quoting the approval sentence", "claude-tmux",
			`The dialog reads "Claude has written up a plan and is ready to execute. Would you like to proceed?" and blocks.`,
			"noop", 0, false},
		// Deliberately NOT diff- or bullet-prefixed: stripAgentDiffLines would
		// eat those before the regex ever ran, so such a fixture would prove
		// diff-stripping works rather than proving anything about this rule.
		// (A surviving mutant found in review: with only stripped fixtures, a
		// rule reduced to the option-2 anchor alone still passed.)
		{"claude: option-2 text on a plain line, no lead sentence", "claude-tmux",
			"Here are the choices:\n   2. Yes, manually approve edits\n   3. Something else", "noop", 0, false},
		{"codex: prose describing the question picker", "codex-tmux",
			`In plan mode codex asks clarifying questions; the footer says "enter to submit answer" and it waits.`,
			"noop", 0, true},
		{"codex: doc mentioning Question counts without the footer", "codex-tmux",
			"The picker header shows Question 1/2 (2 unanswered) but no live dialog is present here.", "noop", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := LoadManifest(tc.cli)
			if err != nil {
				t.Fatalf("LoadManifest(%s): %v", tc.cli, err)
			}
			gotAction, gotRC := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{}, tc.paneBusy)
			if gotAction != tc.wantAction || gotRC != tc.wantRC {
				t.Fatalf("decide[%s]\n  = (%q, %d)\n  want (%q, %d)\npane:\n%s",
					tc.cli, gotAction, gotRC, tc.wantAction, tc.wantRC, tc.pane)
			}
		})
	}
}

// TestAutoRespond_PlanModeDoesNotMatchThisRepositorysOwnFiles is the blocker a
// review caught: this PR documents these dialogs verbatim, on tracked files, so
// the rules' own source material is a false-match vector. stripAgentDiffLines
// protects a `git diff` view (those lines are +/- prefixed) but NOT a Read or
// `cat` render, which is exactly how an agent reads a file it is editing.
//
// So feed the real files through as if an agent had rendered them. A rule that
// fires here would inject stray Enters into a working agent and, after five
// matches, abandon the phase — manufacturing a stall out of a hang that has
// never occurred. The bottom anchor is what makes this pass: the quoted dialogs
// sit mid-document, never at \z.
func TestAutoRespond_PlanModeDoesNotMatchThisRepositorysOwnFiles(t *testing.T) {
	t.Parallel()
	files := []string{
		"autorespond_planmode_test.go",                                      // this file: all three fixtures verbatim
		"manifests/claude-tmux.json",                                        // the rule and its note
		"manifests/codex-tmux.json",                                         // ditto
		"../../../docs/incidents/2026-08-27-plan-mode-dialog-blind-spot.md", // the incident write-up
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v (this guard is worthless if it cannot read its own sources)", f, err)
		}
		if !strings.Contains(string(body), "ready to execute") && !strings.Contains(string(body), "unanswered") {
			t.Fatalf("%s no longer contains the dialog text — this guard has gone vacuous; re-point it", f)
		}
		// Scoped to the PLAN rules on purpose. Running the whole rule set here
		// reds on pre-existing rules — the manifest file literally contains
		// cli_feedback_rating's and auth_recheck's patterns as data, so reading
		// it trips them. That is a real, separate hazard (filed as a follow-up),
		// not something these rules introduced, and folding it in would make
		// this guard fail for reasons it cannot fix.
		for _, cli := range []string{"claude-tmux", "codex-tmux"} {
			m, err := LoadManifest(cli)
			if err != nil {
				t.Fatal(err)
			}
			var planRules []ManifestPrompt
			for _, p := range m.InteractivePrompts {
				if p.Name == "plan_approval" || p.Name == "plan_question" {
					planRules = append(planRules, p)
				}
			}
			if len(planRules) != 1 {
				t.Fatalf("%s: expected exactly one plan rule, found %d — guard mis-scoped", cli, len(planRules))
			}
			action, rc := decideAutoRespond(string(body), planRules, map[string]int{}, false)
			if action != "noop" || rc != 0 {
				t.Errorf("%s rendered as agent output fires the %s plan rule: (%q, %d) — the repo's own docs must never trigger the responder",
					f, cli, action, rc)
			}
		}
	}
}

// TestAutoRespond_PlanModeAnswersEveryLiveQuestion: codex's picker is
// multi-question, so the rule MUST stay multi-fire (once:true would answer Q1
// and hang on Q2). The loop guard abandons past 5 cumulative fires, which is
// the budget for a form; the bottom anchor is what keeps a DISMISSED picker
// from spending that budget. Named for what it asserts — an earlier version of
// this test pinned the abandonment itself and read as if that were the goal.
func TestAutoRespond_PlanModeAnswersEveryLiveQuestion(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	counts := map[string]int{} // shared across ticks, as ar.counts is live
	for tick := 1; tick <= autoRespondLoopGuardLimit; tick++ {
		action, rc := decideAutoRespond(codexPlanQuestionPane, m.InteractivePrompts, counts, true)
		if action != "send:Enter" || rc != 1 {
			t.Fatalf("tick %d: got (%q, %d), want (\"send:Enter\", 1)", tick, action, rc)
		}
	}
	// Past the budget the responder abandons LOUDLY (rc 86) rather than looping
	// forever. With the bottom anchor this should only be reachable by a form
	// with more live questions than the guard allows — not by a dismissed one.
	if _, rc := decideAutoRespond(codexPlanQuestionPane, m.InteractivePrompts, counts, true); rc != 86 {
		t.Errorf("past the guard limit the responder must abandon loudly, got rc %d", rc)
	}
}
