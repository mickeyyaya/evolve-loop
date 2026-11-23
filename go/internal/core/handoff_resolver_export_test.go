package core

import "github.com/mickeyyaya/evolve-loop/go/internal/handoff/request"

func ContractPathResolverForTest(o *Orchestrator, projectRoot string) request.PathResolver {
	return o.contractPathResolver(nil, projectRoot)
}
