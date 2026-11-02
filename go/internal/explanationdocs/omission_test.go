package explanationdocs

import (
	"context"
	"slices"
	"testing"
)

func TestCheckBuild_ReportsEachChangedAreasOmissionAsATypedFinding(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	f.write(t, "config/extra.yaml", "x: 1\n")
	var got []Omission
	binding := f.binding()
	binding.OnOmission = func(o Omission) { got = append(got, o) }

	failures := CheckBuild(context.Background(), binding)

	want := Omission{Path: "config/extra.yaml", Document: cycleDocumentPath(f.cycle, f.runID), Failure: omissionFailure("config/extra.yaml")}
	if !slices.Equal(got, []Omission{want}) {
		t.Fatalf("omissions = %+v, want %+v", got, want)
	}
	if !slices.Contains(failures, want.Failure) {
		t.Fatalf("failures %v do not carry the omission text %q", failures, want.Failure)
	}
}

func TestCheckBuild_AnUnobservedBindingStillRendersTheOmission(t *testing.T) {
	f := newFixture(t)
	f.activate(t)
	f.prepareRequired(t)
	f.write(t, "config/extra.yaml", "x: 1\n")

	if failures := f.check(t); !slices.Contains(failures, omissionFailure("config/extra.yaml")) {
		t.Fatalf("failures = %v, want the omission", failures)
	}
}
