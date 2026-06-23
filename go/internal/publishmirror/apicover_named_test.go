package publishmirror

import (
	"strings"
	"testing"
)

// TestAPICover_NamesExportedSurface references every exported symbol so the
// apicover gate sees the package's full public surface independent of the
// build-tagged integration tests that exercise Run end-to-end. Scan is fully
// exercised in sanitize_test.go; Run under -tags integration.
func TestAPICover_NamesExportedSurface(t *testing.T) {
	if !strings.Contains(DefaultRemote, "evolveloop") {
		t.Errorf("DefaultRemote should target the evolveloop mirror, got %q", DefaultRemote)
	}
	_ = Run
	_ = Scan
	_ = Options{Ref: "HEAD", Remote: DefaultRemote}
	_ = Result{StagedFiles: 0, Dropped: nil}
	_ = Violation{File: "x", Line: 1, Rule: "denylist", Match: "y"}
}
