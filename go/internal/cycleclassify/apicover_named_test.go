package cycleclassify

import "testing"

func TestResult_ClassifyInfrastructureFullValue(t *testing.T) {
	t.Parallel()
	ws := writeReport(t, "EPERM: operation not permitted\n")
	want := Result{Class: ClassInfrastructure, Marker: "EPERM", Source: "orchestrator-report.md"}
	got := Classify(ws)
	if got != want {
		t.Fatalf("Classify = %+v, want %+v", got, want)
	}
}
