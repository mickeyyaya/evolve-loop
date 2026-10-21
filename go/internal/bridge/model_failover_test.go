package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

type scriptedModels struct {
	outcome  map[string]core.BridgeResponse
	models   []string
	artifact string
	stale    []string
}

func (s *scriptedModels) launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	s.models = append(s.models, req.Model)
	if _, err := os.Stat(s.artifact); err == nil {
		s.stale = append(s.stale, req.Model)
	}
	resp := s.outcome[req.Model]
	if resp.ExitCode == ExitOK {
		return resp, nil
	}
	_ = os.WriteFile(s.artifact, []byte("PARTIAL\n"), 0o644)
	return resp, errors.New("bridge: launch exit=85")
}

var (
	wall        = core.BridgeResponse{ExitCode: ExitUnknownPrompt, CauseCode: "rate_limit"}
	unsupported = core.BridgeResponse{ExitCode: ExitUnknownPrompt, CauseCode: "model_unsupported"}
	unknown     = core.BridgeResponse{ExitCode: ExitUnknownPrompt, CauseCode: "unknown_prompt"}
	boot        = core.BridgeResponse{ExitCode: ExitREPLBootTimeout}
	ok          = core.BridgeResponse{ExitCode: ExitOK, Stdout: "OK"}
)

func deepChain(cli, model string) []string {
	if cli == "claude-tmux" && model == "deep" {
		return []string{"opus", "sonnet"}
	}
	return nil
}

func runChain(t *testing.T, ctx context.Context, outcome map[string]core.BridgeResponse, chain func(string, string) []string, session string) (*scriptedModels, core.BridgeResponse, error, string) {
	t.Helper()
	s := &scriptedModels{outcome: outcome, artifact: filepath.Join(t.TempDir(), "artifact.md")}
	var stderr strings.Builder
	eng := NewEngine(Deps{Stderr: &stderr, ModelChain: chain, launchModel: s.launch,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	resp, err := eng.Launch(ctx, core.BridgeRequest{CLI: "claude-tmux", Model: "deep", ArtifactPath: s.artifact, SessionName: session})
	return s, resp, err, stderr.String()
}

func TestLaunch_AQuotaWallOnTheFirstModelOfATierSucceedsOnTheNextModel(t *testing.T) {
	t.Parallel()
	s, resp, err, log := runChain(t, context.Background(), map[string]core.BridgeResponse{"opus": wall, "sonnet": ok}, deepChain, "")
	if err != nil || resp.ExitCode != ExitOK || !slices.Equal(s.models, []string{"opus", "sonnet"}) {
		t.Fatalf("exit %d err %v models %v; want a wall on opus, then success on sonnet", resp.ExitCode, err, s.models)
	}
	if !strings.Contains(log, "model failover: opus on claude-tmux ended with exit 85 (rate_limit or model_unsupported); trying sonnet at the same tier") {
		t.Errorf("the step is not logged: %q", log)
	}
	if len(s.stale) != 0 {
		t.Errorf("a retry saw the artifact the walled model left: %v", s.stale)
	}
}

func TestLaunch_OnlyAWallOfTheWholeChainReturnsTheQuotaExit(t *testing.T) {
	t.Parallel()
	s, resp, err, _ := runChain(t, context.Background(), map[string]core.BridgeResponse{"opus": wall, "sonnet": unsupported}, deepChain, "")
	if err == nil || resp.ExitCode != ExitUnknownPrompt || !slices.Equal(s.models, []string{"opus", "sonnet"}) {
		t.Fatalf("exit %d err %v models %v; want exit 85 after both models wall", resp.ExitCode, err, s.models)
	}
}

func TestLaunch_OnlyAQuotaWallOrAnUnsupportedModelAdvancesTheChain(t *testing.T) {
	t.Parallel()
	for name, first := range map[string]core.BridgeResponse{"an unknown prompt": unknown, "a boot timeout": boot} {
		if s, resp, _, _ := runChain(t, context.Background(), map[string]core.BridgeResponse{"opus": first, "sonnet": ok}, deepChain, ""); !slices.Equal(s.models, []string{"opus"}) || resp.ExitCode != first.ExitCode {
			t.Errorf("%s tried %v (exit %d), want only opus", name, s.models, resp.ExitCode)
		}
	}
	if s, resp, _, _ := runChain(t, context.Background(), map[string]core.BridgeResponse{"opus": unsupported, "sonnet": ok}, deepChain, ""); !slices.Equal(s.models, []string{"opus", "sonnet"}) || resp.ExitCode != ExitOK {
		t.Errorf("an unsupported model tried %v (exit %d), want opus then sonnet", s.models, resp.ExitCode)
	}
}

func TestLaunch_ACancelledContextStopsTheChain(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s, _, _, _ := runChain(t, ctx, map[string]core.BridgeResponse{"opus": wall, "sonnet": ok}, deepChain, ""); !slices.Equal(s.models, []string{"opus"}) {
		t.Errorf("a cancelled context tried %v, want no retry", s.models)
	}
}

func TestLaunch_TheModelAxisStaysOffWithoutAChainANamedSessionOrAnEmptyChain(t *testing.T) {
	t.Parallel()
	walls := map[string]core.BridgeResponse{"deep": wall, "opus": wall}
	for name, c := range map[string]struct {
		chain   func(string, string) []string
		session string
	}{
		"no chain":        {nil, ""},
		"a named session": {deepChain, "swarm-7"},
		"an empty chain":  {func(string, string) []string { return nil }, ""},
	} {
		if s, _, _, _ := runChain(t, context.Background(), walls, c.chain, c.session); !slices.Equal(s.models, []string{"deep"}) {
			t.Errorf("%s: models %v, want one launch at the tier", name, s.models)
		}
	}
}

func TestLaunch_AnArtifactThatCannotBeClearedIsAWarning(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "artifact.md")
	if err := os.MkdirAll(filepath.Join(dir, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	eng := NewEngine(Deps{Stderr: &stderr, TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	eng.clearArtifact(core.BridgeRequest{ArtifactPath: dir})
	if !strings.Contains(stderr.String(), "could not clear the artifact "+dir) {
		t.Errorf("stderr %q, want the clear failure named", stderr.String())
	}
}

func TestLaunch_WithoutASeamLaunchesTheModelForReal(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	artifact := filepath.Join(ws, "artifact.md")
	fr := &fakeRunner{writeArtifactPath: artifact, writeArtifactBody: "OK\n"}
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil), ModelChain: deepChain,
		TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) { return tokenusage.Result{}, nil }})
	resp, err := eng.Launch(context.Background(), core.BridgeRequest{CLI: "claude-p", Profile: writeProfile(t, ws, "real", ""), Model: "deep",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "auditor"})
	if err != nil || resp.ExitCode != ExitOK || len(fr.calls) != 1 {
		t.Errorf("exit %d err %v calls %d, want one real launch", resp.ExitCode, err, len(fr.calls))
	}
}

func TestModelChainFor_OnlyOnWiresTheCatalogChain(t *testing.T) {
	t.Parallel()
	if ModelChainFor(false) != nil {
		t.Error("an off failover must wire no chain")
	}
	if ModelChainFor(true) == nil {
		t.Error("an on failover must wire the catalog chain")
	}
}

func TestCatalogModelChain_ReadsTheLiveCatalogOfTheBaseCLI(t *testing.T) {
	dir := t.TempDir()
	cat := modelcatalog.Catalog{CLIs: map[string]modelcatalog.CLIEntry{"claude": {Source: modelcatalog.SourceLive,
		TierModels: map[string]string{"deep": "opus"}, TierFallbacks: map[string][]string{"deep": {"sonnet"}}}}}
	if err := modelcatalog.Write(dir, cat); err != nil {
		t.Fatal(err)
	}
	prev := modelCatalogDirFn
	t.Cleanup(func() { SetModelCatalogDirFn(prev) })
	SetModelCatalogDirFn(func() string { return dir })
	if got := CatalogModelChain("claude-tmux", "deep"); !slices.Equal(got, []string{"opus", "sonnet"}) {
		t.Errorf("CatalogModelChain = %v, want the claude deep chain", got)
	}
	if got := CatalogModelChain("claude-tmux", "fast"); len(got) != 0 {
		t.Errorf("a tier with no models = %v, want none", got)
	}
}
