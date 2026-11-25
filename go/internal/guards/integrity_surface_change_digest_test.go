package guards

import "testing"

func TestProtectedSurface_ListsTreestate(t *testing.T) {
	for _, p := range []string{
		"go/internal/treestate/pinned.go",
		"go/internal/treedelta/treedelta.go",
		"go/internal/repocontract/changedigest_home_test.go",
		"go/internal/core/phase_bindings.go",
		"go/internal/phases/ship/audit.go",
		"go/internal/phases/ship/commitgate.go",
		"go/cmd/evolve/cmd_integration_batch.go",
		"go/internal/integration/consolepr.go",
		"go/internal/integration/train.go",
	} {
		if !IsProtectedSurface(p) {
			t.Errorf("%s must be protected: a lane that edits it could fix a change digest to a constant or widen the legacy fallback", p)
		}
	}
	for _, p := range []string{
		"go/internal/integration/compose.go",
		"go/internal/phases/ship/verify.go",
		"go/internal/core/phase_bindings_other.go",
	} {
		if IsProtectedSurface(p) {
			t.Errorf("%s must stay ordinary cycle territory: the rows name the digest files only", p)
		}
	}
}
