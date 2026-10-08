package bridge

import (
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestLaunchEffortEqualsTheExplainEffortForAPhaseLabelLaunch(t *testing.T) {
	block := policy.CLIRouting{
		CLIs: []string{"claude"}, Default: []string{"claude"},
		Tiers:  map[string]policy.TierRule{"deep": {Effort: "high"}},
		Agents: map[string]policy.AgentRule{"auditor": {Effort: "xhigh"}},
	}
	pol := policy.Policy{CLIRouting: &block}
	profs := profiles.NewFromFS(fstest.MapFS{
		"auditor.json": {Data: []byte(`{"name":"auditor","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"deep"}`)},
		"builder.json": {Data: []byte(`{"name":"builder","cli":"claude-tmux","model_tier_default":"balanced"}`)},
	})
	table, findings := cliroute.Compile(pol, nil, profs)
	for _, f := range findings {
		if f.Severity == cliroute.SeverityError {
			t.Fatalf("compile: %+v", f)
		}
	}
	router, err := cliroute.New(table, cliroute.Host{LookPath: func(b string) (string, error) { return b, nil }})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ agent, label, tier string }{
		{"auditor", "audit", "deep"},
		{"auditor", "auditor", "top"},
		{"builder", "build", "deep"},
		{"builder", "build", "balanced"},
	}
	for _, tc := range cases {
		launched := launchIntentFor(&Config{Model: tc.tier, Agent: tc.label}, Profile{Name: tc.agent}, pol.Efforts()).Effort
		explained, _ := router.Effort(tc.agent, tc.tier)
		if launched != explained || launched == "" {
			t.Errorf("%s launched as %q at %s: launch effort %q, explain effort %q, want them equal", tc.agent, tc.label, tc.tier, launched, explained)
		}
	}
}

func TestLaunchEffortNeverReadsAKeyThatIsNotTheAgentName(t *testing.T) {
	block := policy.CLIRouting{Agents: map[string]policy.AgentRule{"audit": {Effort: "low"}}}
	got := launchIntentFor(&Config{Model: "deep", Agent: "audit"}, Profile{Name: "auditor"}, policy.Policy{CLIRouting: &block}.Efforts()).Effort
	if got != "medium" {
		t.Fatalf("a launch labelled audit took effort %q, want the deep default medium: the agent key is the agent name only", got)
	}
}

func TestCodexModelIDLaunchSendsNoHiddenEffort(t *testing.T) {
	injectCatalogDir(t, t.TempDir())
	intent := launchIntentFor(&Config{CLI: "codex-tmux", Model: "gpt-5.6-sol"}, Profile{Name: "builder"}, policy.EffortTable{})
	if intent.Effort != "" {
		t.Fatalf("a model-id launch resolved effort %q, want none", intent.Effort)
	}
	for _, flag := range RealizeFor("codex-tmux", intent).LaunchFlags {
		if flag == "model_reasoning_effort=high" || flag == "plan_mode_reasoning_effort=high" {
			t.Fatalf("a model-id codex launch carries a hidden %s", flag)
		}
	}
}

func TestLaunchEffort_IsTheOneResolverOfEveryLaunchPath(t *testing.T) {
	if got := LaunchEffort("claude-tmux", "", policy.EffortTable{}); got != "medium" {
		t.Errorf("LaunchEffort(claude-tmux, no model) = %q, want medium (the balanced default)", got)
	}
	if got := LaunchEffort("agy-claude-tmux", "auto", policy.EffortTable{}); got != "low" {
		t.Errorf("LaunchEffort(agy-claude-tmux, auto) = %q, want low (the fast default of its manifest)", got)
	}
}
