package committedset

import (
	"reflect"
	"testing"
)

func TestLaneSpec_EncodeDecodeRoundTrips(t *testing.T) {
	t.Parallel()
	spec := LaneSpec{DependsOn: []string{"a", "b"}, Priority: 3, OutputContract: "pr"}

	got, err := DecodeLaneSpec(spec.Encode())

	if err != nil || !reflect.DeepEqual(got, spec) {
		t.Fatalf("DecodeLaneSpec(Encode) = %+v, %v, want %+v", got, err, spec)
	}
}

func TestDecodeLaneSpec_AnEmptyValueIsTheZeroSpecAndAMalformedValueIsAnError(t *testing.T) {
	t.Parallel()
	if got, err := DecodeLaneSpec(""); err != nil || !reflect.DeepEqual(got, LaneSpec{}) {
		t.Errorf("DecodeLaneSpec(empty) = %+v, %v, want the zero spec", got, err)
	}
	if _, err := DecodeLaneSpec("{not json"); err == nil {
		t.Error("DecodeLaneSpec(malformed) = nil error, want an error")
	}
}

func TestLaneSpec_IsZeroNamesASpecThatCarriesNothing(t *testing.T) {
	t.Parallel()
	if !(LaneSpec{}).IsZero() {
		t.Error("the zero spec is not IsZero")
	}
	for _, spec := range []LaneSpec{{DependsOn: []string{"a"}}, {Priority: 1}, {OutputContract: "pr"}} {
		if spec.IsZero() {
			t.Errorf("%+v is IsZero, want a spec that carries a field", spec)
		}
	}
}

func TestRecord_TheLaneSpecSurvivesTheLineageFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := Record{Cycle: 4, ItemIDs: []string{"x"}, LaneSpec: LaneSpec{DependsOn: []string{"d"}, Priority: 2, OutputContract: "docs"}}
	if err := WriteRecord(root, want); err != nil {
		t.Fatal(err)
	}

	got, ok := ReadRecord(root, 4)

	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadRecord = %+v, %v, want %+v", got, ok, want)
	}
}
