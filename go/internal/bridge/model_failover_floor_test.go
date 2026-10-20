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

type seatCase struct{ agent, profile string }

func geminiInAgyClaude(cli, model string) []string {
	return []string{"Claude Opus 5.5 (High)", "Gemini 3.1 Pro (High)", "Claude Sonnet 5.5 (High)"}
}

func runFlooredChain(t *testing.T, agent, profile string) (*scriptedModels, string) {
	t.Helper()
	s := &scriptedModels{outcome: map[string]core.BridgeResponse{"Claude Opus 5.5 (High)": wall, "Gemini 3.1 Pro (High)": ok, "Claude Sonnet 5.5 (High)": ok},
		artifact: t.TempDir() + "/artifact.md"}
	var stderr strings.Builder
	eng := NewEngine(Deps{Stderr: &stderr, ModelChain: geminiInAgyClaude, launchModel: s.launch,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "agy-claude-tmux", Model: "deep", Agent: agent, Profile: profile, ArtifactPath: s.artifact}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	return s, stderr.String()
}

func TestLaunch_AFlooredSeatSkipsAFallbackModelOutsideTheClaudeFamily(t *testing.T) {
	t.Parallel()
	seats := []seatCase{
		{"audit", "/repo/.evolve/profiles/auditor.json"},
		{"tdd", "/repo/.evolve/profiles/tdd-engineer.json"},
		{"code-review", "/repo/.evolve/profiles/code-reviewer.json"},
		{"", "/repo/.evolve/profiles/adversarial-review.json"},
	}
	for seat := range profiles.ClaudeFamilyFloor() {
		seats = append(seats, seatCase{"", ".evolve/profiles/" + seat + ".json"})
	}
	for _, seat := range seats {
		s, log := runFlooredChain(t, seat.agent, seat.profile)
		if !slices.Equal(s.models, []string{"Claude Opus 5.5 (High)", "Claude Sonnet 5.5 (High)"}) {
			t.Errorf("agent %q profile %s ran %v: a floored seat must never fail over to a Gemini model", seat.agent, seat.profile, s.models)
		}
		if !strings.Contains(log, "WARN model failover: "+seatName(seat.profile)+" holds the Claude family floor; skipping fallback Gemini 3.1 Pro (High)") {
			t.Errorf("profile %s: the skip is not logged: %q", seat.profile, log)
		}
	}
}

func TestLaunch_AnUnflooredSeatKeepsEveryFallbackModel(t *testing.T) {
	t.Parallel()
	s, log := runFlooredChain(t, "build", "/repo/.evolve/profiles/builder.json")
	if !slices.Equal(s.models, []string{"Claude Opus 5.5 (High)", "Gemini 3.1 Pro (High)"}) || strings.Contains(log, "skipping fallback") {
		t.Errorf("builder ran %v (log %q): the floor binds only the floored seats", s.models, log)
	}
}

func TestLaunch_WithoutAProfileTheAgentNamesTheSeat(t *testing.T) {
	t.Parallel()
	s, _ := runFlooredChain(t, "auditor", "")
	if !slices.Equal(s.models, []string{"Claude Opus 5.5 (High)", "Claude Sonnet 5.5 (High)"}) {
		t.Errorf("ran %v: with no profile the agent name stands in for the seat", s.models)
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
	if _, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "agy-tmux", Model: "deep", Agent: "audit", Profile: "/repo/.evolve/profiles/auditor.json", ArtifactPath: s.artifact}); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if !slices.Equal(s.models, []string{"Gemini 3.1 Pro (High)"}) {
		t.Errorf("ran %v: the routed first model is routing's decision, the walk only filters the fallbacks", s.models)
	}
}
