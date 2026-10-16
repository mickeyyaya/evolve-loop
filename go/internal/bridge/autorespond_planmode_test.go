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
	t.Parallel()
	cases := []struct {
		name, cli, pane, wantAction string
		wantRC                      int
		paneBusy                    bool
	}{
		{"claude plan approval (bypass launch)", "claude-tmux",
			claudePlanApprovalBypassPane, "send:Enter", 1, false},
		{"claude plan approval (plan-mode launch, different option 1)", "claude-tmux",
			claudePlanApprovalPlanModePane, "send:Enter", 1, false},
		{"codex plan question (pane reads busy in production)", "codex-tmux",
			codexPlanQuestionPane, "send:Enter", 1, true},

		{"claude: answered dialog, ONE line of output below", "claude-tmux",
			claudePlanApprovalBypassPane + "\n● Writing hello.go…", "noop", 0, false},
		{"claude: answered dialog, two lines below", "claude-tmux",
			claudePlanApprovalBypassPane + "\n● Writing hello.go…\n  ⎿ wrote 12 lines", "noop", 0, false},
		{"codex: answered picker, ONE line of output below", "codex-tmux",
			codexPlanQuestionPane + "\n› ", "noop", 0, true},
		{"codex: answered picker, two lines below", "codex-tmux",
			codexPlanQuestionPane + "\n• Ran go build ./...\n  └ ok", "noop", 0, true},

		{"claude: prose quoting the approval sentence", "claude-tmux",
			`The dialog reads "Claude has written up a plan and is ready to execute. Would you like to proceed?" and blocks.`,
			"noop", 0, false},
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

func TestAutoRespond_PlanModeDoesNotMatchThisRepositorysOwnFiles(t *testing.T) {
	t.Parallel()
	files := []string{
		"autorespond_planmode_test.go",
		"manifests/claude-tmux.json",
		"manifests/codex-tmux.json",
		"../../../docs/incidents/2026-08-27-plan-mode-dialog-blind-spot.md",
	}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v (this guard is worthless if it cannot read its own sources)", f, err)
		}
		if !strings.Contains(string(body), "ready to execute") && !strings.Contains(string(body), "unanswered") {
			t.Fatalf("%s no longer contains the dialog text — this guard has gone vacuous; re-point it", f)
		}
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

func TestAutoRespond_PlanModeAnswersEveryLiveQuestion(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	counts := map[string]int{}
	for tick := 1; tick <= autoRespondLoopGuardLimit; tick++ {
		action, rc := decideAutoRespond(codexPlanQuestionPane, m.InteractivePrompts, counts, true)
		if action != "send:Enter" || rc != 1 {
			t.Fatalf("tick %d: got (%q, %d), want (\"send:Enter\", 1)", tick, action, rc)
		}
	}
	if _, rc := decideAutoRespond(codexPlanQuestionPane, m.InteractivePrompts, counts, true); rc != 86 {
		t.Errorf("past the guard limit the responder must abandon loudly, got rc %d", rc)
	}
}
