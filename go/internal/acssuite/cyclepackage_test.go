package acssuite

import (
	"path/filepath"
	"testing"
)

func TestCyclePackage_IsTheOneSpelling(t *testing.T) {
	t.Parallel()
	if got := CyclePackage(1605); got != "./acs/cycle1605" {
		t.Fatalf("CyclePackage(1605) = %q", got)
	}
	if got := currentCycleGoPkgDir("/m", 7); got != filepath.Join("/m", "acs", "cycle7") {
		t.Fatalf("currentCycleGoPkgDir must derive from CyclePackage: %q", got)
	}
}
