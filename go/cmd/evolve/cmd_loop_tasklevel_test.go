package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
)

func TestIsTaskLevelFailure_AllClassifications(t *testing.T) {
	t.Parallel()
	cases := []struct {
		c    cycleclassify.Classification
		want bool
	}{
		{cycleclassify.ClassBuildFail, true},
		{cycleclassify.ClassAuditFail, true},
		{cycleclassify.ClassShipGateConfig, true},
		{cycleclassify.ClassInfrastructure, false},
		{cycleclassify.ClassIntegrityBreach, false},
	}
	for _, tc := range cases {
		if got := isTaskLevelFailure(tc.c); got != tc.want {
			t.Errorf("isTaskLevelFailure(%v)=%v, want %v", tc.c, got, tc.want)
		}
	}
}
