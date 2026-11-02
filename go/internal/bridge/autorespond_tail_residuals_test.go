package bridge

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

const tailRuleName = "plan_approval"

const tailDialogLine = "Would you like to proceed with this plan?"

func tailRule(tailLines int) []ManifestPrompt {
	return []ManifestPrompt{{
		Name:         tailRuleName,
		Regex:        `Would you like to proceed with this plan\?`,
		TailLines:    tailLines,
		Policy:       "auto_respond",
		ResponseKeys: "1",
	}}
}

func paneWithDialogLinesFromBottom(linesBelow int) string {
	rows := []string{"agent output above the dialog", tailDialogLine}
	for i := 0; i < linesBelow; i++ {
		rows = append(rows, fmt.Sprintf("later output row %d", i))
	}
	return strings.Join(rows, "\n")
}

func tailResponder(t *testing.T, tailLines int, human bool) (*autoResponder, *interaction.Recorder, *bytes.Buffer) {
	t.Helper()
	ws := t.TempDir()
	stderr := &bytes.Buffer{}
	deps := Deps{Tmux: &fakeTmux{}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: stderr}.withDefaults()
	rec := interaction.NewRecorder(ws)
	ar := newAutoResponder("claude-tmux", ws, deps, human, 0)
	ar.prompts = tailRule(tailLines)
	ar.rec, ar.phase, ar.cycle = rec, "build", 2101
	return ar, rec, stderr
}

func tailOutcomeSummary(rec *interaction.Recorder) []string {
	var got []string
	for _, o := range rec.Outcomes() {
		got = append(got, o.RuleID+":"+o.Result)
	}
	return got
}

func tailHasResult(rec *interaction.Recorder, result string) bool {
	for _, o := range rec.Outcomes() {
		if o.Result == result {
			return true
		}
	}
	return false
}

func TestAutoRespondTail_ScrolledUpDialogClearsPendingForATailScopedRule(t *testing.T) {
	t.Parallel()
	ar, rec, _ := tailResponder(t, 3, false)
	ctx := context.Background()

	if _, rc := ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(0), true); rc != 1 {
		t.Fatalf("first tick rc = %d, want 1: the dialog sits inside the rule's tail", rc)
	}
	if ar.pending == nil {
		t.Fatal("a send must open a pending outcome")
	}

	ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(6), true)

	if !tailHasResult(rec, interaction.ResultPromptCleared) {
		t.Errorf("outcomes = %v, want prompt_cleared: the dialog scrolled out of the %d-line tail, so the rule no longer sees it", tailOutcomeSummary(rec), 3)
	}
	if ar.pending != nil {
		t.Errorf("pending = %+v, want none after the dialog left the tail", ar.pending)
	}
}

func TestAutoRespondTail_DialogStillInsideTheTailStaysPending(t *testing.T) {
	t.Parallel()
	ar, rec, _ := tailResponder(t, 3, false)
	ctx := context.Background()

	ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(0), true)
	ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(1), true)

	if tailHasResult(rec, interaction.ResultPromptCleared) {
		t.Errorf("outcomes = %v, want no prompt_cleared: the dialog is still inside the tail", tailOutcomeSummary(rec))
	}
	if ar.pending == nil {
		t.Error("pending = nil, want the re-fired send to stay pending while the dialog is in the tail")
	}
}

func TestAutoRespondTail_DecideAndResolveAgreeAtEveryTailBoundary(t *testing.T) {
	t.Parallel()
	for _, tailLines := range []int{1, 2, 3, 5} {
		for linesBelow := 0; linesBelow <= tailLines+2; linesBelow++ {
			name := fmt.Sprintf("tail%d_below%d", tailLines, linesBelow)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				ar, rec, _ := tailResponder(t, tailLines, false)
				ctx := context.Background()
				ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(0), true)

				_, rc := ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(linesBelow), true)

				stillMatches := linesBelow < tailLines
				refired := rc == 1
				cleared := tailHasResult(rec, interaction.ResultPromptCleared)
				if refired != stillMatches {
					t.Fatalf("decide: refired=%v, want %v (dialog is %d lines above the bottom, tail is %d)", refired, stillMatches, linesBelow, tailLines)
				}
				if cleared == stillMatches {
					t.Errorf("resolve: cleared=%v while decide matched=%v — the two paths disagree on the tail window (outcomes %v)", cleared, stillMatches, tailOutcomeSummary(rec))
				}
			})
		}
	}
}

func TestAutoRespondTail_RuleWithoutATailKeepsMatchingTheWholePane(t *testing.T) {
	t.Parallel()
	ar, rec, _ := tailResponder(t, 0, false)
	ctx := context.Background()

	ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(0), true)
	ar.tickPane(ctx, "s", paneWithDialogLinesFromBottom(40), true)
	if tailHasResult(rec, interaction.ResultPromptCleared) {
		t.Errorf("outcomes = %v, want no prompt_cleared: an untailed rule matches the full pane", tailOutcomeSummary(rec))
	}

	ar.tickPane(ctx, "s", "the dialog is gone entirely", true)
	if !tailHasResult(rec, interaction.ResultPromptCleared) {
		t.Errorf("outcomes = %v, want prompt_cleared once the dialog is absent from the whole pane", tailOutcomeSummary(rec))
	}
}

func TestAutoRespondAttribution_HumanInputSentKeysLineNamesTheRule(t *testing.T) {
	t.Parallel()
	ar, _, stderr := tailResponder(t, 3, true)

	if _, rc := ar.tickPane(context.Background(), "s", paneWithDialogLinesFromBottom(0), true); rc != 1 {
		t.Fatalf("tick rc = %d, want 1", rc)
	}

	var sentLine string
	for _, ln := range strings.Split(stderr.String(), "\n") {
		if strings.Contains(ln, "[human-input] sent keys") {
			sentLine = ln
		}
	}
	if sentLine == "" {
		t.Fatalf("no human-input sent-keys line in stderr:\n%s", stderr.String())
	}
	if !strings.Contains(sentLine, "rule="+tailRuleName) {
		t.Errorf("human-input line %q does not name the rule %q", sentLine, tailRuleName)
	}
	if strings.Contains(sentLine, "rule=unknown") {
		t.Errorf("human-input line %q attributes the send to an unknown rule", sentLine)
	}
}

func TestAutoRespondAttribution_AutoPathStillNamesTheRule(t *testing.T) {
	t.Parallel()
	ar, _, stderr := tailResponder(t, 3, false)

	ar.tickPane(context.Background(), "s", paneWithDialogLinesFromBottom(0), true)

	if !strings.Contains(stderr.String(), "[auto-respond] sent keys: 1 (rule="+tailRuleName+")") {
		t.Errorf("auto-respond attribution regressed; stderr:\n%s", stderr.String())
	}
}
