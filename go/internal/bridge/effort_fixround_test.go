package bridge

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func modelFlag(t *testing.T, flags []string) string {
	t.Helper()
	i := slices.Index(flags, "--model")
	if i < 0 || i+1 >= len(flags) {
		t.Fatalf("no --model in %v", flags)
	}
	return flags[i+1]
}

func TestSmokeLaunchConfig_CarriesTheTierDefaultEffort(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	deep := smokeLaunchConfig(&Config{Model: "deep"}, "claude-tmux", policy.EffortTable{})
	if !containsSubsequence(deep.Realization.LaunchFlags, []string{"--effort", "medium"}) {
		t.Errorf("deep smoke flags = %v, want --effort medium", deep.Realization.LaunchFlags)
	}
	untiered := smokeLaunchConfig(&Config{}, "agy-claude-tmux", policy.EffortTable{})
	if got := modelFlag(t, untiered.Realization.LaunchFlags); got != "Claude Sonnet 5.5 (Low)" || untiered.Realization.EffortVariant != "(Low)" {
		t.Errorf("model-less agy-claude smoke = %q variant %q, want the fast tier at its own (Low) effort", got, untiered.Realization.EffortVariant)
	}
	policyTable := smokeLaunchConfig(&Config{Model: "deep"}, "claude-tmux", effortTableOf(policy.CLIRouting{Tiers: map[string]policy.TierRule{"deep": {Effort: "high"}}}))
	if !containsSubsequence(policyTable.Realization.LaunchFlags, []string{"--effort", "high"}) {
		t.Errorf("smoke with a policy table = %v, want --effort high from the table", policyTable.Realization.LaunchFlags)
	}
}

func TestControllerFamilyConfig_CarriesAnEffort(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	c := NewController(&Config{AllowBypass: true}, Deps{}).(*cliController)
	cfg := c.perFamilyConfig("claude-tmux")
	if !containsSubsequence(cfg.Realization.LaunchFlags, []string{"--effort", "medium"}) {
		t.Errorf("control session flags = %v, want --effort medium (a model-less launch takes the balanced tier)", cfg.Realization.LaunchFlags)
	}
}

func TestLaunchIntentFor_AnUntieredModelTakesTheManifestDefaultTier(t *testing.T) {
	cases := []struct{ cli, model, want string }{
		{"claude-tmux", "auto", "medium"},
		{"claude-tmux", "", "medium"},
		{"agy-claude-tmux", "auto", "low"},
		{"claude-tmux", "claude-opus-5-5", ""},
	}
	for _, tc := range cases {
		got := launchIntentFor(&Config{CLI: tc.cli, Model: tc.model}, Profile{}, policy.EffortTable{}).Effort
		if got != tc.want {
			t.Errorf("launchIntentFor(cli=%s model=%q).Effort = %q, want %q", tc.cli, tc.model, got, tc.want)
		}
	}
}

func TestClaudePHeadlessLaunchCarriesTheEffort(t *testing.T) {
	cfg := &Config{CLI: "claude-p", Model: "deep"}
	cfg.Realization = RealizeFor("claude-p", launchIntentFor(cfg, Profile{}, policy.EffortTable{}))
	args, _ := claudePArgs(cfg, "prompt")
	if !containsSubsequence(args, []string{"--effort", "medium"}) {
		t.Fatalf("claude -p argv = %v, want --effort medium", args)
	}
}

func TestAgyClaudeExplicitAgentEffortWinsOverAPinnedVariant(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	cfg := &Config{CLI: "agy-claude-tmux", Model: "Claude Opus 5.5 (High)"}
	r := RealizeFor("agy-claude-tmux", launchIntentFor(cfg, Profile{Name: "scout"}, effortTableOf(policy.CLIRouting{Agents: map[string]policy.AgentRule{"scout": {Effort: "low"}}})))
	if got := modelFlag(t, r.LaunchFlags); got != "Claude Opus 5.5 (Low)" {
		t.Fatalf("--model = %q, want the profile effort to replace the pinned (High)", got)
	}
}

func TestModelVariant_ANameWithoutAVariantRecordsTheUnappliedEffort(t *testing.T) {
	m := Manifest{
		CLI:          "variant-fixture",
		ModelTierMap: map[string]string{"deep": "claude-opus-5-5"},
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "flag", Flag: "--model", From: "model_tier_map"},
			"effort":     {Channel: effortChannelModelVariant, Values: map[string][]string{"medium": {"Medium"}}},
		},
	}
	if r := Realize(m, LaunchIntent{ModelTier: "deep", Effort: "medium"}); r.EffortUnapplied != "medium" {
		t.Fatalf("EffortUnapplied = %q, want %q", r.EffortUnapplied, "medium")
	}
	if r := Realize(m, LaunchIntent{ModelTier: "deep"}); r.EffortUnapplied != "" {
		t.Fatalf("no effort requested, EffortUnapplied = %q, want none", r.EffortUnapplied)
	}
}

func TestReportLaunchModel_WarnsOnceForAnUnappliedEffort(t *testing.T) {
	var stderr bytes.Buffer
	cfg := &Config{Model: "deep", Realization: Realization{EffortUnapplied: "high"}}
	reportLaunchModel(Deps{Stderr: &stderr}, cfg, "[agy-claude-tmux]", "s", "/w")
	want := "[agy-claude-tmux] WARN effort=high is not applied: the model name has no (Variant) suffix"
	if got := strings.Count(stderr.String(), want); got != 1 {
		t.Fatalf("stderr has %d warn lines %q, want 1:\n%s", got, want, stderr.String())
	}
}

func TestBootSmokeTest_ALaunchFromRawDepsCarriesTheCompiledEffort(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	tmux := &fakeTmux{paneSeq: []string{"❯"}}
	deps, _ := bootSmokeDeps(tmux)
	if rc, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: t.TempDir(), Model: "deep"}, deps); rc != ExitOK {
		t.Fatalf("rc = %d, want ExitOK", rc)
	}
	if !tmux.sentContains("--effort medium") {
		t.Errorf("the boot smoke launch line lacks --effort medium; sent=%v", tmux.sentKeys)
	}
}
