package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// resolveModelTier answers whether (cli,tier) resolves to a model; it uses
// bridge.LoadManifest rather than modelcatalog.Catalog.Lookup because Lookup
// reports ok=false for any CLI on source:"detect" (codex today).
func resolveModelTier(cli, tier string) (string, bool) {
	m, err := bridge.LoadManifest(policy.BaseCLI(cli) + "-tmux")
	if err != nil {
		return "", false
	}
	model := m.ModelTierMap[tier]
	return model, model != ""
}
