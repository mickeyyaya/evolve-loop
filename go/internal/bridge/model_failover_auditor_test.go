package bridge

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const auditorProfilePath = "/repo/.evolve/profiles/auditor.json"

func useLiveClaudeDeepChain(t *testing.T, primary string, fallbacks []string) {
	t.Helper()
	dir := t.TempDir()
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{"claude": {Source: modelcatalog.SourceLive,
		TierModels: map[string]string{"deep": primary}, TierFallbacks: map[string][]string{"deep": fallbacks}}}}
	if err := modelcatalog.Write(dir, cat); err != nil {
		t.Fatal(err)
	}
	prev := modelCatalogDirFn
	t.Cleanup(func() { SetModelCatalogDirFn(prev) })
	SetModelCatalogDirFn(func() string { return dir })
}

func launchAuditorAtDeep(t *testing.T, outcome map[string]core.BridgeResponse) (*scriptedModels, core.BridgeResponse, error, string) {
	t.Helper()
	s := &scriptedModels{outcome: outcome, artifact: t.TempDir() + "/artifact.md"}
	var stderr strings.Builder
	eng := NewEngine(Deps{Stderr: &stderr, ModelChain: CatalogModelChain, launchModel: s.launch,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	resp, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "claude-tmux", Model: "deep", Agent: "audit",
		Profile: auditorProfilePath, ArtifactPath: s.artifact})
	return s, resp, err, stderr.String()
}

func TestLaunch_TheAuditorSucceedsOnOpusWhenFableHitsItsQuotaWall(t *testing.T) {
	useLiveClaudeDeepChain(t, "fable", []string{"fable", "opus", "sonnet"})
	s, resp, err, log := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"fable": wall, "opus": ok, "sonnet": ok})
	if err != nil || resp.ExitCode != ExitOK || !slices.Equal(s.models, []string{"fable", "opus"}) {
		t.Fatalf("exit %d err %v models %v; want fable walled, then success on opus without reaching sonnet", resp.ExitCode, err, s.models)
	}
	if !strings.Contains(log, "model failover: fable on claude-tmux ended with exit 85") || !strings.Contains(log, "trying opus at the same tier") {
		t.Errorf("the fable to opus step is not logged: %q", log)
	}
	if len(s.stale) != 0 {
		t.Errorf("the opus retry saw the artifact fable left behind: %v", s.stale)
	}
}

func TestLaunch_TheAuditorStepsPastTwoWalledModelsToTheThird(t *testing.T) {
	useLiveClaudeDeepChain(t, "fable", []string{"fable", "opus", "sonnet"})
	s, resp, err, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"fable": wall, "opus": unsupported, "sonnet": ok})
	if err != nil || resp.ExitCode != ExitOK || !slices.Equal(s.models, []string{"fable", "opus", "sonnet"}) {
		t.Fatalf("exit %d err %v models %v; want success on sonnet after fable and opus wall", resp.ExitCode, err, s.models)
	}
}

func TestLaunch_TheAuditorReturnsTheQuotaExitOnlyWhenFableOpusAndSonnetAllWall(t *testing.T) {
	useLiveClaudeDeepChain(t, "fable", []string{"fable", "opus", "sonnet"})
	s, resp, err, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"fable": wall, "opus": wall, "sonnet": wall})
	if err == nil || resp.ExitCode != ExitUnknownPrompt || resp.CauseCode != "rate_limit" || !slices.Equal(s.models, []string{"fable", "opus", "sonnet"}) {
		t.Fatalf("exit %d cause %q err %v models %v; want exit 85 only after all three models wall", resp.ExitCode, resp.CauseCode, err, s.models)
	}
}

func TestLaunch_TheAuditorDoesNotLeaveFableForAPromptShapeProblem(t *testing.T) {
	useLiveClaudeDeepChain(t, "fable", []string{"fable", "opus", "sonnet"})
	s, resp, _, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"fable": unknown, "opus": ok, "sonnet": ok})
	if !slices.Equal(s.models, []string{"fable"}) || resp.ExitCode != ExitUnknownPrompt {
		t.Errorf("an unknown prompt tried %v (exit %d), want only fable", s.models, resp.ExitCode)
	}
}

func TestLaunch_TheAuditorKeepsFableWhenTheCatalogPrimaryIsOpus(t *testing.T) {
	useLiveClaudeDeepChain(t, "opus", []string{"fable", "opus", "sonnet"})
	s, resp, err, log := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"opus": wall, "fable": ok, "sonnet": ok})
	if err != nil || resp.ExitCode != ExitOK || !slices.Equal(s.models, []string{"opus", "fable"}) {
		t.Fatalf("exit %d err %v models %v; want opus walled, then success on fable, a Claude model the family floor must keep", resp.ExitCode, err, s.models)
	}
	if strings.Contains(log, "skipping fallback fable") {
		t.Errorf("the floor skipped fable as outside the claude family: %q", log)
	}
}

func TestLaunch_TheFamilyFloorRecognisesEveryClaudeAliasOfTheLiveChain(t *testing.T) {
	t.Parallel()
	for _, alias := range []string{"fable", "opus", "sonnet", "haiku", "claude-fable-5-1", "Claude Opus 5.5 (High)"} {
		if !isFamilyModel("claude", alias) {
			t.Errorf("isFamilyModel(claude, %q) = false, want true", alias)
		}
	}
	for _, other := range []string{"Gemini 3.1 Pro (High)", "gpt-5.6-sol", "Gemini 3.5 Flash (Low)"} {
		if isFamilyModel("claude", other) {
			t.Errorf("isFamilyModel(claude, %q) = true, want false", other)
		}
	}
}
