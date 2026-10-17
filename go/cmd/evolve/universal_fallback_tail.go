package main

import (
	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func universalFallbackTail(results []gobridge.DoctorResult) []string {
	seen := map[string]bool{}
	var tail []string
	for _, r := range results {
		bin := llmroute.Binary(r.CLI)
		resolved, err := gobridge.Resolve(driverSet, bin)
		if err != nil || seen[bin] || !r.Binary.Present || r.Verdict == "blocked" || !gobridge.HasToolUse(resolved.Driver) {
			continue
		}
		seen[bin] = true
		tail = append(tail, resolved.Driver)
	}
	return tail
}
