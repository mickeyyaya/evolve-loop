package dossier

import "path/filepath"

// CyclesDir returns <projectRoot>/knowledge-base/cycles, the committed dossier
// corpus; moving it is a protocol change.
// See ADR-0094.
func CyclesDir(projectRoot string) string {
	return filepath.Join(projectRoot, "knowledge-base", "cycles")
}
