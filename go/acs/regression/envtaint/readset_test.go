//go:build acs

package envtaint

import (
	"reflect"
	"testing"
)

// EvolveConstKeys is the fold-aware read-set R primitive for the honest flag
// metric. It must, in ONE pass over a source file:
//   - FOLD a split-const so the cycle-20 dodge is caught: os.Getenv("EVOLVE_" +
//     "WORKTREE_BASE") yields EVOLVE_WORKTREE_BASE (a literal go/ast scan misses
//     this — that is the whole point);
//   - include a plain literal read (EVOLVE_PROJECT_ROOT);
//   - EXCLUDE a site carrying the `// SSOT IPC-protocol-allowed` marker (blessed
//     IPC keys are not operator dials), whether the marker is trailing or on the
//     line immediately above the declaration;
//   - EXCLUDE a dynamic key whose value is not a compile-time constant
//     (EVOLVE_PHASE_<name>_BIN), which has no fixed dial to count.
//
// It must not require imports to resolve (a whole-repo walk cannot depend on the
// build cache), so "os" being unresolvable must not abort the fold.
func TestEvolveConstKeys_FoldsExcludesMarkedAndDynamic(t *testing.T) {
	const src = `package p

import "os"

// the cycle-20 dodge: an operator dial hidden behind a split-const, no marker.
var _ = os.Getenv("EVOLVE_" + "WORKTREE_BASE")

// a plain literal operator-dial read.
var _ = os.Getenv("EVOLVE_PROJECT_ROOT")

// a blessed IPC key, trailing marker — excluded.
const ipcA = "EVOLVE_" + "RESUME_MODE" // SSOT IPC-protocol-allowed

// a blessed IPC key, marker on the line above — excluded.
// SSOT IPC-protocol-allowed: parent->child handoff
const ipcB = "EVOLVE_DISPATCH_DEPTH"

// a dynamic per-phase key — not a constant, excluded.
func read(name string) string { return os.Getenv("EVOLVE_PHASE_" + name + "_BIN") }
`
	got, err := EvolveConstKeys(src)
	if err != nil {
		t.Fatalf("EvolveConstKeys: %v", err)
	}
	want := []string{"EVOLVE_PROJECT_ROOT", "EVOLVE_WORKTREE_BASE"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EvolveConstKeys = %v, want %v", got, want)
	}
}
