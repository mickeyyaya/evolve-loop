package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleidentity"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func resumeIdentityConfig(pol policy.Policy) core.ResumeIdentityConfig {
	stage, _ := config.GateStage(string(pol.ResumeIdentityStage()))
	return core.ResumeIdentityConfig{
		Stage: stage,
		Compute: func(projectRoot string) (cycleidentity.Identity, error) {
			return cycleidentity.Compute(cycleidentity.Live(projectRoot))
		},
	}
}
