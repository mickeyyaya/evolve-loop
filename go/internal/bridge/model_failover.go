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
			return e.floorChain(floorSeat(req), chain)
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

var familyModelWords = map[string][]string{"claude": {"claude", "opus", "sonnet", "haiku"}}

func isFamilyModel(family, model string) bool {
	lower := strings.ToLower(model)
	return slices.ContainsFunc(familyModelWords[family], func(w string) bool { return strings.Contains(lower, w) })
}

func (e *Engine) floors() policy.Floors {
	if e.deps.Floors != nil {
		return e.deps.Floors
	}
	return policy.DefaultFloors()
}

func (e *Engine) floorChain(seat string, chain []string) []string {
	floor := e.floors().FamilyOf(seat)
	if floor == "" {
		return chain
	}
	kept := []string{chain[0]}
	for _, model := range chain[1:] {
		if isFamilyModel(floor, model) {
			kept = append(kept, model)
			continue
		}
		fmt.Fprintf(e.deps.Stderr, "[bridge] WARN model failover: %s holds the %s family floor; skipping fallback %s, which is outside the %s family\n", seat, floor, model, floor)
	}
	return kept
}

func floorSeat(req core.BridgeRequest) string {
	if req.Profile != "" {
		return seatName(req.Profile)
	}
	return seatName(req.Agent)
}

func seatName(agent string) string {
	base := filepath.Base(agent)
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}
