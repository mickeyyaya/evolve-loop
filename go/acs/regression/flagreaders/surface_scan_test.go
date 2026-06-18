//go:build acs

package flagreaders

import (
	"path/filepath"
	"testing"
)

// TestScanTextTree_DetectsNonGoOnlyReference is the cycle-360 regression proof.
//
// Cycle-360 removed flags classified "dead" by a Go-only scan while they still
// had live readers in a non-Go surface (adapters/claude.sh). This test pins that
// the broadened guard detects an EVOLVE_* reference living ONLY in a non-Go
// surface: scanTextTree over a fixture skill flags EVOLVE_FAKE_NONGO_ONLY_FLAG
// (no registry row) while ignoring a real, registered flag in the same file and
// a dynamic-prefix fragment.
func TestScanTextTree_DetectsNonGoOnlyReference(t *testing.T) {
	fixture := filepath.Join("testdata", "surfacefixture")

	// hasRow stubs the registry: only the real flag is "registered".
	hasRow := func(name string) bool { return name == "EVOLVE_SANDBOX" }

	orphans := map[string][]string{}
	if err := scanTextTree(fixture, textExts, hasRow, orphans); err != nil {
		t.Fatalf("scanTextTree(%s): %v", fixture, err)
	}

	if _, found := orphans["EVOLVE_FAKE_NONGO_ONLY_FLAG"]; !found {
		t.Errorf("scanTextTree did not flag EVOLVE_FAKE_NONGO_ONLY_FLAG — a flag "+
			"referenced only in a non-Go surface; the cycle-360 false-\"dead\" class is NOT closed.\n  got orphans: %v", orphans)
	}
	if locs, found := orphans["EVOLVE_SANDBOX"]; found {
		t.Errorf("scanTextTree falsely flagged registered flag EVOLVE_SANDBOX as orphan at %v", locs)
	}
	// The fixture line is the dynamic-prefix form `EVOLVE_E2E_MODEL_${cli}`; the
	// trailing `_${cli}` must prevent ANY match on that line (the `_` after MODEL is
	// a word char, so no \b fires). This does NOT claim a standalone EVOLVE_E2E_MODEL
	// would be ignored — that would correctly be flagged.
	if locs, found := orphans["EVOLVE_E2E_MODEL"]; found {
		t.Errorf("scanTextTree produced EVOLVE_E2E_MODEL from the dynamic-prefix form `EVOLVE_E2E_MODEL_${cli}` at %v — the trailing _${cli} must prevent any match on that line", locs)
	}
}
