package bridge

import (
	"os"
	"strings"
	"testing"
)

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

func TestAutoRespond_PlanModeDialogs(t *testing.T) {
	cases := []struct {
		name, cli, pane, wantAction string
		wantRC                      int
		paneBusy                    bool
	}{
		// --- live dialogs: respond ---
		// Enter approves. An autonomous loop has no human to read the plan and the auditor grades the diff
		// anyway, so approving beats a dead lane; a blocked agent cannot use an extended deadline.
		{"claude plan approval (bypass launch)", "claude-tmux",
			claudePlanApprovalBypassPane, "send:Enter", 1, false},
		{"claude plan approval (plan-mode launch, different option 1)", "claude-tmux",
			claudePlanApprovalPlanModePane, "send:Enter", 1, false},
		// codex's picker footer carries "esc to interrupt", so the real pane reads busy.
		{"codex plan question (pane reads busy in production)", "codex-tmux",
			codexPlanQuestionPane, "send:Enter", 1, true},

		// --- the bottom anchor: scrollback that scrolled up must NOT fire ---
		// An answered or quoted dialog sits above later output and cannot match \z. One new line below the
		// dialog must end the match, which is why each tail anchors on the dialog's own final line.
		{"claude: answered dialog, ONE line of output below", "claude-tmux",
			claudePlanApprovalBypassPane + "\n● Writing hello.go…", "noop", 0, false},
		{"claude: answered dialog, two lines below", "claude-tmux",
			claudePlanApprovalBypassPane + "\n● Writing hello.go…\n  ⎿ wrote 12 lines", "noop", 0, false},
		{"codex: answered picker, ONE line of output below", "codex-tmux",
			codexPlanQuestionPane + "\n› ", "noop", 0, true},
		{"codex: answered picker, two lines below", "codex-tmux",
			codexPlanQuestionPane + "\n• Ran go build ./...\n  └ ok", "noop", 0, true},

		// --- prose and paraphrase must not fire ---
		{"claude: prose quoting the approval sentence", "claude-tmux",
			`The dialog reads "Claude has written up a plan and is ready to execute. Would you like to proceed?" and blocks.`,
			"noop", 0, false},
		// Deliberately not diff- or bullet-prefixed: stripAgentDiffLines would eat such a fixture,
		// proving diff stripping instead of this rule.
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

// Feeds the rules' own tracked source material through as an agent's Read or cat renders it;
// stripAgentDiffLines protects only a diff view. The bottom anchor is what makes this pass.
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
		// Scoped to the plan rules on purpose: the manifest file holds other rules' patterns as data,
		// which trips them. That is a separate hazard this guard cannot fix.
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

// codex's picker asks several questions, so the rule must stay multi-fire (once:true would hang on Q2).
// The loop guard's budget bounds a form; the bottom anchor keeps a dismissed picker from spending it.
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
	// Past the budget the responder abandons loudly (rc 86) rather than looping forever.
	if _, rc := decideAutoRespond(codexPlanQuestionPane, m.InteractivePrompts, counts, true); rc != 86 {
		t.Errorf("past the guard limit the responder must abandon loudly, got rc %d", rc)
	}
}
