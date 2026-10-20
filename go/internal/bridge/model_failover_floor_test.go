package bridge

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

func geminiInAgyClaude(cli, model string) []string {
	return []string{"Claude Opus 5.5 (High)", "Gemini 3.1 Pro (High)", "Claude Sonnet 5.5 (High)"}
}

func runFlooredChain(t *testing.T, agent string) (*scriptedModels, string) {
	t.Helper()
	s := &scriptedModels{outcome: map[string]core.BridgeResponse{"Claude Opus 5.5 (High)": wall, "Gemini 3.1 Pro (High)": ok, "Claude Sonnet 5.5 (High)": ok},
		artifact: t.TempDir() + "/artifact.md"}
	var stderr strings.Builder
	eng := NewEngine(Deps{Stderr: &stderr, ModelChain: geminiInAgyClaude, launchModel: s.launch,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Model: "deep", Agent: agent, ArtifactPath: s.artifact}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	return s, stderr.String()
}

func TestLaunch_AFlooredSeatSkipsAFallbackModelOutsideTheClaudeFamily(t *testing.T) {
	t.Parallel()
	agents := []string{"Auditor", ".evolve/profiles/code-reviewer.json"}
	for seat := range profiles.ClaudeFamilyFloor() {
		agents = append(agents, seat)
	}
	for _, agent := range agents {
		s, log := runFlooredChain(t, agent)
		if !slices.Equal(s.models, []string{"Claude Opus 5.5 (High)", "Claude Sonnet 5.5 (High)"}) {
			t.Errorf("%s ran %v: a floored seat must never fail over to a Gemini model", agent, s.models)
		}
		if !strings.Contains(log, "WARN model failover: "+agent+" holds the Claude family floor; skipping fallback Gemini 3.1 Pro (High)") {
			t.Errorf("%s: the skip is not logged: %q", agent, log)
		}
	}
}

func TestLaunch_AnUnflooredSeatKeepsEveryFallbackModel(t *testing.T) {
	t.Parallel()
	s, log := runFlooredChain(t, "builder")
	if !slices.Equal(s.models, []string{"Claude Opus 5.5 (High)", "Gemini 3.1 Pro (High)"}) || strings.Contains(log, "skipping fallback") {
		t.Errorf("builder ran %v (log %q): the floor binds only the floored seats", s.models, log)
	}
}

func TestIsClaudeFamilyModel_ReadsCatalogNames(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]bool{"opus": true, "sonnet": true, "haiku": true, "Claude Opus 5.5 (High)": true,
		"claude-sonnet-5-5": true, "Gemini 3.1 Pro (High)": false, "gpt-5.6-sol": false, "gemma4:31b-cloud": false, "": false} {
		if got := isClaudeFamilyModel(name); got != want {
			t.Errorf("isClaudeFamilyModel(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestLaunch_AFlooredSeatKeepsItsRoutedFirstModel(t *testing.T) {
	t.Parallel()
	s := &scriptedModels{outcome: map[string]core.BridgeResponse{"Gemini 3.1 Pro (High)": ok}, artifact: t.TempDir() + "/artifact.md"}
	var stderr strings.Builder
	eng := NewEngine(Deps{Stderr: &stderr, launchModel: s.launch,
		ModelChain:    func(string, string) []string { return []string{"Gemini 3.1 Pro (High)", "Gemini 3.5 Flash (Low)"} },
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "agy-tmux", Model: "deep", Agent: "auditor", ArtifactPath: s.artifact}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if !slices.Equal(s.models, []string{"Gemini 3.1 Pro (High)"}) {
		t.Errorf("ran %v: the routed first model is routing's decision, the walk only filters the fallbacks", s.models)
	}
}
