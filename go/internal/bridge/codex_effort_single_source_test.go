package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const reasoningEffortKey = "model_reasoning_effort="

func effortValues(args []string) []string {
	var out []string
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, reasoningEffortKey); ok {
			out = append(out, v)
		}
	}
	return out
}

func launchHeadlessCodex(t *testing.T, realization Realization) []string {
	t.Helper()
	runner := &fakeRunner{}
	var stderr bytes.Buffer
	deps := Deps{Runner: runner.runner(), Stderr: &stderr,
		Env: map[string]string{"ANTHROPIC_API_KEY": "", "ANTHROPIC_BASE_URL": "", "OPENAI_API_KEY": ""}}
	logs := t.TempDir()
	rc, err := codexDriver{}.Launch(context.Background(), &Config{
		CLI: "codex", PromptFile: agyPromptFile(t), Model: "auto",
		Artifact:  filepath.Join(logs, "artifact.md"),
		StdoutLog: filepath.Join(logs, "out.log"), StderrLog: filepath.Join(logs, "err.log"),
		Worktree: logs, Realization: realization,
	}, deps)
	if err != nil || rc != 0 || len(runner.calls) != 1 {
		t.Fatalf("Launch rc=%d err=%v calls=%d stderr=%q", rc, err, len(runner.calls), stderr.String())
	}
	return runner.calls[0].args
}

func TestCodexEffort_HeadlessRealizationDrawsFromFamilyTable(t *testing.T) {
	t.Parallel()
	family, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(codex-tmux): %v", err)
	}
	table := family.Params["effort"].Values
	if len(table) == 0 {
		t.Fatal("codex-tmux declares no effort table — this guard would be vacuous")
	}
	for tier, familyTokens := range table {
		got := RealizeFor("codex", LaunchIntent{Effort: tier}).LaunchFlags
		if v := effortValues(got); !slices.Equal(v, []string{tier}) {
			t.Errorf("headless codex effort=%q: %s values = %v in %v, want exactly [%s]", tier, reasoningEffortKey, v, got, tier)
		}
		for _, tok := range got {
			if !slices.Contains(familyTokens, tok) {
				t.Errorf("headless codex effort=%q emits %q, which the family table %v does not declare — a second copy of the policy", tier, tok, familyTokens)
			}
		}
	}
}

func TestCodexEffort_HeadlessUnknownEffortEmitsNoReasoningKey(t *testing.T) {
	t.Parallel()
	for _, effort := range []string{"", "bogus", "HIGH "} {
		got := RealizeFor("codex", LaunchIntent{Effort: effort}).LaunchFlags
		if v := effortValues(got); len(v) != 0 {
			t.Errorf("effort=%q realized %s%v, want no reasoning key for an effort the family table does not name", effort, reasoningEffortKey, v)
		}
	}
}

func TestCodexEffort_LaunchIntentEffortReachesHeadlessArgv(t *testing.T) {
	t.Parallel()
	for _, tier := range []string{"low", "medium", "xhigh"} {
		intent := launchIntentFor(&Config{CLI: "codex", Model: "balanced", Effort: tier}, Profile{}, policy.EffortTable{})
		args := launchHeadlessCodex(t, RealizeFor("codex", intent))
		if v := effortValues(args); !slices.Equal(v, []string{tier}) {
			t.Errorf("resolved effort %q: headless argv %s values = %v in %v, want exactly [%s]", tier, reasoningEffortKey, v, args, tier)
		}
	}
}

func TestCodexEffort_HeadlessManifestHoldsNoSecondEffortTable(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("manifests", "codex.json"))
	if err != nil {
		t.Fatalf("read headless manifest: %v", err)
	}
	if strings.Contains(string(raw), reasoningEffortKey) {
		t.Error("manifests/codex.json spells " + reasoningEffortKey + ": the effort table has one home, codex-tmux.json")
	}
}

func TestArgsContainEffort_MatchesOnlyTheKeyAtTokenStart(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"exact key", []string{"-c", "model_reasoning_effort=high"}, true},
		{"quoted value", []string{"-c", `model_reasoning_effort="low"`}, true},
		{"plan-mode key", []string{"-c", "plan_mode_reasoning_effort=high"}, false},
		{"key as suffix of a longer name", []string{"-c", "not_model_reasoning_effort=high"}, false},
		{"key inside another value", []string{"--note", "see model_reasoning_effort=high"}, false},
		{"no args", nil, false},
	}
	for _, tc := range cases {
		if got := argsContainEffort(tc.args); got != tc.want {
			t.Errorf("%s: argsContainEffort(%v) = %v, want %v", tc.name, tc.args, got, tc.want)
		}
	}
}
