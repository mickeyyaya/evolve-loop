package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

func sourceCycleFloor(root string) (int, error) {
	moduleDir, err := codequality.ResolveModuleDir(root)
	if err != nil {
		return 0, err
	}
	return acssuite.HighestCyclePackageNumber(moduleDir)
}

func verifyFreshCycleSource(root string, cycle int) error {
	moduleDir, err := codequality.ResolveModuleDir(root)
	if err != nil {
		return err
	}
	occupied, err := acssuite.CyclePackageOccupied(moduleDir, cycle)
	if err != nil {
		return err
	}
	if occupied {
		return fmt.Errorf("%s is already occupied in provisioned source %s", acssuite.CyclePackage(cycle), root)
	}
	return nil
}
