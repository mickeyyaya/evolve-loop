package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func resolveModelTier(cli, tier string) (string, bool) {
	m, err := bridge.LoadManifest(policy.BaseCLI(cli) + "-tmux")
	if err != nil {
		return "", false
	}
	model := m.ModelTierMap[tier]
	return model, model != ""
}
