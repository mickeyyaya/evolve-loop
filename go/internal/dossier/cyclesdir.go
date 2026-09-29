package dossier

import (
	"fmt"
	"os"
	"path/filepath"
)

// CyclesDir returns <projectRoot>/knowledge-base/cycles, the committed dossier
// corpus; moving it is a protocol change.
// See ADR-0094.
func CyclesDir(projectRoot string) string {
	return filepath.Join(projectRoot, "knowledge-base", "cycles")
}

func ClosedOut(projectRoot string, cycle int) bool {
	name := fmt.Sprintf("cycle-%d.json", cycle)
	for _, dir := range []string{CyclesDir(projectRoot), PendingDir(projectRoot)} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}
