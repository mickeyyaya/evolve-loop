package main

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type stageContractInput struct {
	Effective       func() (map[string]string, error)
	Records         stageevidence.FlipRecords
	GenesisRequired func() (bool, error)
}

func stageContractSource(in stageContractInput) func() (stageevidence.CheckResult, error) {
	return func() (stageevidence.CheckResult, error) {
		required, err := in.GenesisRequired()
		if err != nil {
			return stageevidence.CheckResult{}, err
		}
		effective, err := in.Effective()
		if err != nil {
			return stageevidence.CheckResult{}, err
		}
		set, err := in.Records.FlipRecords(context.Background())
		if err != nil {
			return stageevidence.CheckResult{}, err
		}
		return stageevidence.Check(stageevidence.CheckInput{Effective: effective, Records: set, GenesisRequired: required}), nil
	}
}
