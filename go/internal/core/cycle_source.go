package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
)

// sourceCycleFloor returns the highest cycle identity visible in root. Module
// discovery is checked because an unreadable go/ path cannot safely be treated
// as an empty ACS inventory.
func sourceCycleFloor(root string) (int, error) {
	moduleDir, err := codequality.ResolveModuleDir(root)
	if err != nil {
		return 0, err
	}
	return acssuite.HighestCyclePackageNumber(moduleDir)
}

// verifyFreshCycleSource checks the provisioned source snapshot, after any
// upstream fetch, for the exact identity allocated from the caller's checkout.
// Resume bypasses newCycleRun and therefore does not apply this fresh-run gate.
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
