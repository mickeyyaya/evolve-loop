package selfsha

import "testing"

func TestNamePublicAPI(t *testing.T) {
	if _, err := Running(); err != nil {
		t.Fatalf("Running: %v", err)
	}
	if _, err := Of("/nonexistent-apicover-probe-path"); err == nil {
		t.Fatal("Of must error on a missing path")
	}
}

func TestNamePath(t *testing.T) {
	if p, err := Path(); err != nil || p == "" {
		t.Fatalf("Path = %q, %v", p, err)
	}
}
