package bridge

import (
	"context"
	"fmt"
	"os"
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
			return chain
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
