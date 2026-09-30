//go:build acs

package cycle74

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestC74_AssertIntentStopCriterion(t *testing.T) {
	root := acsassert.RepoRoot(t)
	intent := filepath.Join(root, "agents", "evolve-intent.md")
	if !fixtures.FilePresent(intent) {
		t.Skip("evolve-intent.md missing — skip cycle-74")
	}
	for _, marker := range []string{"Emergency Exit", "Hard Stop"} {
		if !acsassert.FileContains(t, intent, marker) {
			return
		}
	}
}
