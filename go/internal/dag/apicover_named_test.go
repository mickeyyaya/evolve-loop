package dag

import (
	"reflect"
	"testing"
)

// TestLevelsAndFlatten_NamePublicAPI pins the package's two exported functions
// together: dependency levels flatten back into deterministic execution order.
func TestLevelsAndFlatten_NamePublicAPI(t *testing.T) {
	levels, err := Levels(
		[]string{"build", "study", "ship"},
		map[string][]string{"build": {"study"}, "ship": {"build"}},
	)
	if err != nil {
		t.Fatalf("Levels: %v", err)
	}
	if got, want := Flatten(levels), []string{"study", "build", "ship"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Flatten(Levels(...)) = %v, want %v", got, want)
	}
}
