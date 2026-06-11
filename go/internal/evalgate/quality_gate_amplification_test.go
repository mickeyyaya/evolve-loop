package evalgate

import "testing"

func TestQualityGateNameIsStablePredicateQuality(t *testing.T) {
	if got := (qualityGate{}).name(); got != "predicate-quality" {
		t.Fatalf("qualityGate.name() = %q, want %q", got, "predicate-quality")
	}
}
