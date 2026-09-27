package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// internalTreeRoot is the go/internal tree — the parent of this package. `go
// test` runs each test binary with its own package directory as the working
// directory, so ".." is go/internal regardless of where the module was invoked
// from.
const internalTreeRoot = ".."

// sharedTmpRoot is assembled by concatenation so this guard's own source
// can never match itself.
var sharedTmpRoot = `"/tmp/` + `p"`

func TestCoreTests_NeverPinSharedTmpProjectRoot(t *testing.T) {
	if _, err := os.Stat(internalTreeRoot); err != nil {
		t.Fatalf("go/internal tree not found at %q: %v — the guard would scan nothing and pass vacuously", internalTreeRoot, err)
	}
	var offenders []string
	err := filepath.WalkDir(internalTreeRoot, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		// Only *_test.go: the class is TESTS pinning a shared root. Production
		// code that merely names a tmp path is not cross-process mutable state.
		if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(raw), sharedTmpRoot) {
			rel, relErr := filepath.Rel(internalTreeRoot, path)
			if relErr != nil {
				rel = path
			}
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the go/internal tree: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("%d test file(s) under go/internal pin the shared machine-global %s project root: %v — use t.TempDir() so each test owns its root; "+
			"a fixed shared path is cross-process mutable state and produced the false suites_stay_green reds of cycles 1107/1116",
			len(offenders), sharedTmpRoot, offenders)
	}
}
