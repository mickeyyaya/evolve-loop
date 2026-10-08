package dossier

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CyclesDir returns <projectRoot>/knowledge-base/cycles, the committed dossier
// corpus; moving it is a protocol change.
// See ADR-0094.
func CyclesDir(projectRoot string) string {
	return filepath.Join(projectRoot, "knowledge-base", "cycles")
}

func ClosedOut(projectRoot string, cycle int) bool {
	_, ok := ClosedOutAt(projectRoot, cycle)
	return ok
}

func ClosedOutAt(projectRoot string, cycle int) (time.Time, bool) {
	at, _, ok := closeout(projectRoot, cycle)
	return at, ok
}

func CloseoutPath(projectRoot string, cycle int) (string, bool) {
	_, path, ok := closeout(projectRoot, cycle)
	return path, ok
}

func closeout(projectRoot string, cycle int) (time.Time, string, bool) {
	name := fmt.Sprintf("cycle-%d.json", cycle)
	for _, dir := range []string{CyclesDir(projectRoot), PendingDir(projectRoot)} {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil {
			return info.ModTime(), path, true
		}
	}
	return time.Time{}, "", false
}
