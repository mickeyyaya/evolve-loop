package main

import (
	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func universalFallbackTail(results []gobridge.DoctorResult, excluded []string) []string {
	seen := map[string]bool{}
	var tail []string
	for _, r := range results {
		fam := llmroute.Family(r.CLI)
		driver := fam + "-tmux"
		if seen[fam] || !r.Binary.Present || r.Verdict == "blocked" || !gobridge.HasToolUse(driver) {
			continue
		}
		seen[fam] = true
		tail = append(tail, driver)
	}
	return llmroute.ExcludeFamilies(tail, excluded)
}
