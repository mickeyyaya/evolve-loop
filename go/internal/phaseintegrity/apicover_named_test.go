package phaseintegrity

import "testing"

// TestNamePublicAPI references phaseintegrity's exported surface (Source and its
// DigestSource methods) for the apicover public-API gate; behavior is exercised
// in source_test.go.
func TestNamePublicAPI(t *testing.T) {
	s := Source{GitTree: func(string) (string, error) { return "", nil }}
	if _, err := s.BinarySHA(); err != nil {
		t.Fatalf("BinarySHA: %v", err)
	}
	_ = s.BinaryCommit()
	if _, err := s.ProfileSHA(); err != nil {
		t.Fatalf("ProfileSHA: %v", err)
	}
	if _, err := s.ReportSHA(); err != nil {
		t.Fatalf("ReportSHA: %v", err)
	}
	if _, err := s.TreeSHA(); err != nil {
		t.Fatalf("TreeSHA: %v", err)
	}
}
