//go:build acs

package cycle73

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestC73_AssertScoutStopCriterion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	scout := filepath.Join(root, "agents", "evolve-scout.md")
	if !fixtures.FilePresent(scout) {
		t.Skip("evolve-scout.md missing — skip cycle-73")
	}
	for _, marker := range []string{"turn 10", "turn 7", "turn 5"} {
		if !acsassert.FileContains(t, scout, marker) {
			return
		}
	}
}
