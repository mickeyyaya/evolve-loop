package selfsha

import "testing"

// TestNamePublicAPI references the package's exported surface (Of, Running) for
// the apicover public-API gate; behavior is exercised in selfsha_test.go.
func TestNamePublicAPI(t *testing.T) {
	if _, err := Running(); err != nil {
		t.Fatalf("Running: %v", err)
	}
	if _, err := Of("/nonexistent-apicover-probe-path"); err == nil {
		t.Fatal("Of must error on a missing path")
	}
}
