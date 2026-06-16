package cycleclassify

import "testing"

// TestResult_ClassifyInfrastructureFullValue names the cycleclassify.Result type
// (Classify returns it but the bare type is never named in a test) and pins the
// WHOLE value Classify returns on an infrastructure marker — not just Class.
// Result is all-comparable (Classification is a string), so the Class+Marker+
// Source triple is asserted with one struct equality.
func TestResult_ClassifyInfrastructureFullValue(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "EPERM: operation not permitted\n")
	want := Result{Class: ClassInfrastructure, Marker: "EPERM", Source: "orchestrator-report.md"}
	got := Classify(ws)
	if got != want {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
}
