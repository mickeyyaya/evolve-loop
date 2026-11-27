package repocontract

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/testrefs"
)

func TestDocsOfRecord_EveryNamedTestExists(t *testing.T) {
	t.Parallel()
	problems, err := testrefs.Scan(filepath.Dir(realModuleRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}
