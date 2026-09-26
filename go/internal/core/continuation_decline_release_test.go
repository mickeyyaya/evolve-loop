package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestReleaseDeclinedBinding_RemovesMatchingScopesOnly(t *testing.T) {
	root := t.TempDir()
	declined := &continuation.Continuation{Cycle: 1405, SnapshotSHA: "c4f6b41b4dbe"}
	if err := continuation.WriteRegistryEntry(root, "defect-disposition-contract-unsatisfiable", *declined); err != nil {
		t.Fatal(err)
	}
	// A second scope bound to a DIFFERENT ancestor must survive the release.
	if err := continuation.WriteRegistryEntry(root, "unrelated-live-scope", continuation.Continuation{Cycle: 1421, SnapshotSHA: "feedface"}); err != nil {
		t.Fatal(err)
	}

	releaseDeclinedBinding(root, []string{"defect-disposition-contract-unsatisfiable", "unrelated-live-scope", "scope-with-no-binding"}, declined)

	if _, ok, _ := continuation.ReadRegistryEntry(root, "defect-disposition-contract-unsatisfiable"); ok {
		t.Error("the declined binding survived — the scope stays an absorbing FAIL for every future lane")
	}
	if c, ok, _ := continuation.ReadRegistryEntry(root, "unrelated-live-scope"); !ok || c.Cycle != 1421 {
		t.Errorf("a binding to a different ancestor must not be collateral damage, got ok=%v cycle=%d", ok, c.Cycle)
	}
}

func TestReleaseDeclinedBinding_NilAndEmptyAreNoops(t *testing.T) {
	root := t.TempDir()
	releaseDeclinedBinding(root, nil, nil)
	releaseDeclinedBinding(root, []string{"a"}, nil)
	releaseDeclinedBinding(root, nil, &continuation.Continuation{Cycle: 1})
}
