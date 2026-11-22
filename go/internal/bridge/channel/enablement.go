package channel

import "github.com/mickeyyaya/evolve-loop/go/internal/policy"

func ResolveStage(raw string) string { return policy.ResolveIsolationStage(raw) }

func Enabled(stage string) bool {
	return stage == "enforce"
}
