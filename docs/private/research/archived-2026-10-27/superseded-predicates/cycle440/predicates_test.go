//go:build acs

package cycle440

import "testing"

func TestC440_005_GenuineCatalogMissStillClamps(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestClampPlanModelRouting_ClampsCatalogMiss", false, routerPkg)
	if code != 0 {
		t.Errorf("C440_005: catalog-miss-still-clamps test exit=%d\nstderr=%s", code, stderr)
	}
}
