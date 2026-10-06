package clihealth

import "testing"

func TestBenchable_TheQuotaExhaustedWallIsBenchable(t *testing.T) {
	if !Benchable(QuotaExhaustedPattern) || QuotaExhaustedPattern != "quota_exhausted" {
		t.Errorf("Benchable(%q) = false; agy names its quota wall quota_exhausted and must be benched", QuotaExhaustedPattern)
	}
	if Benchable("quota_exhausted_x") || Benchable("Quota_Exhausted") {
		t.Error("the bench vocabulary is exact-match")
	}
}
