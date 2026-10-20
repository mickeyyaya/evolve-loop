package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func ModelChainFor(on bool) func(cli, model string) []string {
	if on {
		return CatalogModelChain
	}
	return nil
}

func CatalogModelChain(cli, tier string) []string {
	return loadCatalogCached().DispatchModelChain(policy.BaseCLI(cli), tier)
}

func (e *Engine) launchOnce(ctx context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if e.deps.launchModel != nil {
		return e.deps.launchModel(ctx, req)
	}
	return e.launchModel(ctx, req)
}

func (e *Engine) modelChain(req core.BridgeRequest) []string {
	if e.deps.ModelChain != nil && req.SessionName == "" {
		if chain := e.deps.ModelChain(req.CLI, req.Model); len(chain) > 0 {
			return e.floorChain(req.Agent, chain)
		}
	}
	return []string{req.Model}
}

var modelWallCauses = []string{"rate_limit", "model_unsupported"}

func modelWalled(resp core.BridgeResponse) bool {
	return resp.ExitCode == ExitUnknownPrompt && slices.Contains(modelWallCauses, resp.CauseCode)
}

func (e *Engine) logModelStep(req core.BridgeRequest, from, to string) {
	fmt.Fprintf(e.deps.Stderr, "[bridge] model failover: %s on %s ended with exit %d (%s); trying %s at the same tier\n", from, req.CLI, ExitUnknownPrompt, strings.Join(modelWallCauses, " or "), to)
}

func (e *Engine) clearArtifact(req core.BridgeRequest) {
	if err := os.Remove(req.ArtifactPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(e.deps.Stderr, "[bridge] WARN model failover: could not clear the artifact %s of the walled model: %v\n", req.ArtifactPath, err)
	}
}

var claudeModelWords = []string{"claude", "opus", "sonnet", "haiku"}

func isClaudeFamilyModel(model string) bool {
	lower := strings.ToLower(model)
	return slices.ContainsFunc(claudeModelWords, func(w string) bool { return strings.Contains(lower, w) })
}

func (e *Engine) floorChain(agent string, chain []string) []string {
	if !profiles.IsClaudeFamilyFloor(seatName(agent)) {
		return chain
	}
	kept := []string{chain[0]}
	for _, model := range chain[1:] {
		if isClaudeFamilyModel(model) {
			kept = append(kept, model)
			continue
		}
		fmt.Fprintf(e.deps.Stderr, "[bridge] WARN model failover: %s holds the Claude family floor; skipping fallback %s, which is not a Claude model\n", agent, model)
	}
	return kept
}

func seatName(agent string) string {
	base := filepath.Base(agent)
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}
