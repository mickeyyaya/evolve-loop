package gateregistry

import "testing"

func TestStageEntries_RegressionTIAIsRetired(t *testing.T) {
	t.Parallel()
	for _, e := range StageEntries() {
		if e.Name == "regression_tia.stage" || e.Shadow.Signal == "TIA_SHADOW" {
			t.Errorf("%s is still in the stage registry with signal %q: the stage retired because 0 of 44 predicates are pure", e.Name, e.Shadow.Signal)
		}
	}
}
