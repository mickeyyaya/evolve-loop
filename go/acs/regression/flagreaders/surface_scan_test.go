//go:build acs

package flagreaders

import (
	"path/filepath"
	"testing"
)

func TestScanTextTree_DetectsNonGoOnlyReference(t *testing.T) {
	fixture := filepath.Join("testdata", "surfacefixture")

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
	if locs, found := orphans["EVOLVE_E2E_MODEL"]; found {
		t.Errorf("scanTextTree produced EVOLVE_E2E_MODEL from the dynamic-prefix form `EVOLVE_E2E_MODEL_${cli}` at %v — the trailing _${cli} must prevent any match on that line", locs)
	}
}
