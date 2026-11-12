package core

import "testing"

func TestBoundaryOwned_ListedPrefixOwnedSourceNot(t *testing.T) {
	t.Parallel()
	if !BoundaryOwned(boundaryOwnedPrefixes[0] + "probe") {
		t.Error("a path under a listed prefix must be boundary-owned")
	}
	if BoundaryOwned("go/internal/core/landed_main.go") {
		t.Error("a source path must not be boundary-owned")
	}
}
