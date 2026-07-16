package apicover

import (
	"context"
	"testing"
)

func TestNamesReferencedInTests_CollectsIdentsAndSelectors(t *testing.T) {
	named, err := NamesReferencedInTests(context.Background(), "testdata/sample")
	if err != nil {
		t.Fatalf("NamesReferencedInTests: %v", err)
	}
	// usage_test.go references these (idents + selector names).
	for _, want := range []string{"ExportedFunc", "ExportedVar", "ExportedType", "ExportedMethod"} {
		if !named[want] {
			t.Errorf("expected %q to be referenced by the _test fixture; got %v", want, named)
		}
	}
	// A name never mentioned in any _test.go must be absent.
	if named["NeverReferencedAnywhere"] {
		t.Error("did not expect an unreferenced name to appear")
	}
}
