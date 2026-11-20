package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/coversplit"
)

func tierRuns(runs []coversplit.Run) []coversplit.Run {
	var plain, tiered []coversplit.Run
	for _, run := range runs {
		var rest []string
		for _, pkg := range run.Packages {
			if _, ok := ciparity.EnvExclusiveTagOf(pkg); !ok {
				rest = append(rest, pkg)
				continue
			}
			tiered = append(tiered, coversplit.Run{Packages: []string{pkg}, CoverPkg: run.CoverPkg, Tags: packageCoverTags(pkg)})
		}
		if len(rest) > 0 {
			run.Packages = rest
			plain = append(plain, run)
		}
	}
	return append(plain, tiered...)
}
