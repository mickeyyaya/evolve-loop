package phasestream

import "testing"

func TestKindCorrelation_Defined(t *testing.T) {
	if KindCorrelation != "correlation" {
		t.Fatalf("KindCorrelation = %q", KindCorrelation)
	}
}
