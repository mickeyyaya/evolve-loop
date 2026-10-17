package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func resolveModelTier(cli, tier string) (string, bool) {
	r, err := bridge.Resolve(driverSet, policy.BaseCLI(cli))
	if err != nil {
		return "", false
	}
	model := r.Manifest.ModelTierMap[tier]
	return model, model != ""
}
