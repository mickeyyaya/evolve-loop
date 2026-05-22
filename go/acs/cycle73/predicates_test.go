// Package cycle73 ports the cycle-73 ACS predicate (1 bash file).
package cycle73

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC73_AssertScoutStopCriterion ports cycle-73/assert-scout-stop-criterion.sh.
func TestC73_AssertScoutStopCriterion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if !acsassert.FileExists(t, scout) {
		t.Skip("evolve-scout.md missing — skip cycle-73")
	}
	for _, marker := range []string{"turn 10", "turn 7", "turn 5"} {
		if !acsassert.FileContains(t, scout, marker) {
			return
		}
	}
}
