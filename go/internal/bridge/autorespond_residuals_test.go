package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

func TestAutoRespondOutcome_ATailScopedDialogThatScrolledUpIsCleared(t *testing.T) {
	t.Parallel()
	tailRule := []ManifestPrompt{{Name: "tail_trust", Regex: "Do you trust", Policy: "auto_respond", ResponseKeys: "y,Enter", TailLines: 2}}
	ar, rec := autoRespondHarness(t, []string{"noise\nDo you trust", "Do you trust\nline a\nline b\nline c"}, tailRule)
	ctx := context.Background()

	if _, rc := ar.tick(ctx, "s"); rc != 1 {
		t.Fatalf("first tick must auto-respond (rc=1)")
	}
	if _, rc := ar.tick(ctx, "s"); rc != 0 {
		t.Fatalf("second tick must noop: the dialog left the tail window")
	}

	outs := rec.Outcomes()
	if len(outs) != 1 || outs[0].Result != interaction.ResultPromptCleared {
		t.Fatalf("outcomes = %+v, want one prompt_cleared: the pending check must read the same tail window as the match", outs)
	}
}

func TestHumanInput_TheSentKeysNameTheRule(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	deps := Deps{Tmux: &fakeTmux{paneSeq: []string{"Do you trust this folder?"}}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: &stderr}.withDefaults()
	ar := newAutoResponder("claude-tmux", t.TempDir(), deps, true, 0)
	ar.prompts = trustPrompt

	if _, rc := ar.tick(context.Background(), "s"); rc != 1 {
		t.Fatalf("the tick must auto-respond (rc=1)")
	}

	if !strings.Contains(stderr.String(), "[human-input] sent keys: y,Enter (rule=trust_folder)") {
		t.Fatalf("stderr = %q, want the human-input send attributed to its rule", stderr.String())
	}
}

func TestDecideAutoRespond_AManifestRenderedInThePaneFiresNoRule(t *testing.T) {
	t.Parallel()
	for _, name := range ManifestNames() {
		raw, err := os.ReadFile(filepath.Join("manifests", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatal(err)
		}
		if action, rc := decideAutoRespond(string(raw), m.InteractivePrompts, map[string]int{}, false); action != "noop" || rc != 0 {
			t.Errorf("%s: the manifest text in the pane gave %q (rc=%d); its patterns are data, not a dialog", name, action, rc)
		}
		if fires := firesAtPaneBottoms(string(raw), m.InteractivePrompts); len(fires) > 0 {
			t.Errorf("%s: (%q, rc %d) fires on a pane that ends at line %d of the manifest", name, fires[0].action, fires[0].rc, fires[0].bottom+1)
		}
	}
}

func TestDecideAutoRespond_ALiveDialogStillFiresBesideManifestData(t *testing.T) {
	t.Parallel()
	pane := "  \"regex\": \"Please log in\",\n  \"policy\": \"escalate\"\nHow's the CLI experience so far?\n"
	prompts := []ManifestPrompt{
		{Name: "auth_recheck", Regex: "Please log in", Policy: "escalate"},
		{Name: "cli_feedback_rating", Regex: `How's the CLI experience so far\?`, Policy: "auto_respond", ResponseKeys: "0,Enter"},
	}

	if action, rc := decideAutoRespond(pane, prompts, map[string]int{}, false); action != "send:0,Enter" || rc != 1 {
		t.Fatalf("decide = %q rc=%d, want the live feedback dialog answered and the data line ignored", action, rc)
	}
}

const reworkedPlanDialog = `Claude has written up a plan and is ready to execute. Would you like to proceed?

 ❯ 1. Yes, and auto-accept edits
   2. Yes, and review each edit
   3. No, keep planning
`

func TestWarnPromptRuleDrift_ARewordedPlanDialogNamesTheDriftedRule(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer

	warnPromptRuleDrift(&w, "[claude-tmux]", "claude-tmux", reworkedPlanDialog)

	if !strings.Contains(w.String(), "POSSIBLE PROMPT-RULE DRIFT") || !strings.Contains(w.String(), "interactive_prompts.plan_approval") {
		t.Fatalf("warn = %q, want the plan_approval rule named as drifted", w.String())
	}
}

func TestWarnPromptRuleDrift_StaysQuietWhenTheRuleStillMatchesOrNothingLooksLikeTheDialog(t *testing.T) {
	t.Parallel()
	live := "Claude has written up a plan and is ready to execute. Would you like to proceed?\n\n ❯ 1. Yes, auto-accept edits\n   2. Yes, manually approve edits\n   3. No, keep planning\n\n ctrl+g to edit in Vim · ~/.claude/plans/x.md\n"
	for name, tc := range map[string]struct{ cli, pane string }{
		"the rule matches":    {"claude-tmux", live},
		"no dialog shape":     {"claude-tmux", "● working on the task…"},
		"an empty pane":       {"claude-tmux", "   "},
		"no probe in the cli": {"ollama-tmux", reworkedPlanDialog},
		"an unknown cli":      {"no-such-cli", reworkedPlanDialog},
	} {
		var w bytes.Buffer
		warnPromptRuleDrift(&w, "[x]", tc.cli, tc.pane)
		if w.Len() != 0 {
			t.Errorf("%s: warn = %q, want nothing", name, w.String())
		}
	}
	warnPromptRuleDrift(nil, "[x]", "claude-tmux", reworkedPlanDialog)
}

func TestAutoRespondOutcome_AOnceRuleWhoseDialogLingersIsRecordedAndLoggedOnce(t *testing.T) {
	t.Parallel()
	once := []ManifestPrompt{{Name: "trust_folder", Regex: "Do you trust", Policy: "auto_respond", ResponseKeys: "y,Enter", Once: true}}
	ar, rec := autoRespondHarness(t, []string{"Do you trust", "Do you trust", "Do you trust"}, once)
	var stderr bytes.Buffer
	ar.deps.Stderr = &stderr
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ar.tick(ctx, "s")
	}

	outs := rec.Outcomes()
	if len(outs) != 1 || outs[0].Result != interaction.ResultSuppressedLingering {
		t.Fatalf("outcomes = %+v, want one suppressed_lingering", outs)
	}
	if n := strings.Count(stderr.String(), "already handled; suppressing re-fire"); n != 1 {
		t.Fatalf("the lingering notice was logged %d times, want once:\n%s", n, stderr.String())
	}
}

func TestWarnPromptRuleDrift_AnAcceptedDialogThatScrolledUpIsNoDrift(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer
	pane := reworkedPlanDialog + strings.Repeat("● editing go/internal/x.go\n", 12)

	warnPromptRuleDrift(&w, "[claude-tmux]", "claude-tmux", pane)

	if w.Len() != 0 {
		t.Fatalf("warn = %q: a dialog above the tail window of the rule was answered, it did not drift", w.String())
	}
}

func TestDecideAutoRespond_DataLinesBelowAQuotedDialogStillKeepItOffThePaneBottom(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	quoted := "Claude has written up a plan and is ready to execute. Would you like to proceed?\n\n ❯ 1. Yes, auto-accept edits\n   2. Yes, manually approve edits\n   3. No, keep planning\n\n ctrl+g to edit in Vim · ~/.claude/plans/x.md\n"
	pane := quoted + "  \"name\": \"plan_approval\",\n  \"policy\": \"auto_respond\"\n"

	if action, rc := decideAutoRespond(pane, m.InteractivePrompts, map[string]int{}, false); action != "noop" || rc != 0 {
		t.Fatalf("decide = %q rc=%d: data lines after a quoted dialog must still hold it off the pane bottom", action, rc)
	}
	if action, _ := decideAutoRespond(quoted, m.InteractivePrompts, map[string]int{}, false); action != "send:Enter" {
		t.Fatalf("the live dialog at the pane bottom gave %q, want send:Enter", action)
	}
}
