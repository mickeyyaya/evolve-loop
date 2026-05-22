// Package cycle74 ports the cycle-74 ACS predicate (1 bash file).
package cycle74

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC74_AssertIntentStopCriterion ports cycle-74/assert-intent-stop-criterion.sh.
func TestC74_AssertIntentStopCriterion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	intent := filepath.Join(root, "agents", "evolve-intent.md")
	if !acsassert.FileExists(t, intent) {
		t.Skip("evolve-intent.md missing — skip cycle-74")
	}
	for _, marker := range []string{"Emergency Exit", "Hard Stop"} {
		if !acsassert.FileContains(t, intent, marker) {
			return
		}
	}
}
