package acssuite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HighestCyclePackageNumber returns the largest occupied canonical cycle
// package name under <moduleDir>/acs. Missing ACS trees have no floor. An
// entry need not be a directory to reserve its number: any existing cycleN
// path would collide with a fresh package of the same name.
func HighestCyclePackageNumber(moduleDir string) (int, error) {
	if moduleDir == "" {
		return 0, nil
	}
	acsDir := filepath.Join(moduleDir, "acs")
	entries, err := os.ReadDir(acsDir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read ACS package inventory %s: %w", acsDir, err)
	}

	highest := 0
	for _, entry := range entries {
		n, ok := canonicalCyclePackageNumber(entry.Name())
		if ok && n > highest {
			highest = n
		}
	}
	return highest, nil
}

// CyclePackageOccupied reports whether any filesystem entry already reserves
// the canonical package path for cycle. It uses Lstat so even a dangling
// symlink blocks reuse of that identity.
func CyclePackageOccupied(moduleDir string, cycle int) (bool, error) {
	if moduleDir == "" {
		return false, nil
	}
	path := currentCycleGoPkgDir(moduleDir, cycle)
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("inspect ACS cycle package %s: %w", path, err)
}

func canonicalCyclePackageNumber(name string) (int, bool) {
	const prefix = "cycle"
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	digits := strings.TrimPrefix(name, prefix)
	if digits == "" || digits[0] == '0' {
		return 0, false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil && n > 0
}
