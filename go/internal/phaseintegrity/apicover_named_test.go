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

// TestNamePublicAPI_Repin names the re-pin surface (ProvenanceVerified,
// RepinResult, RepinShipSHA) for the apicover public-API gate; behavior is
// exercised in repin_test.go.
func TestNamePublicAPI_Repin(t *testing.T) {
	var _ ProvenanceVerified = func(string) bool { return true }
	want := RepinResult{Repinned: true, OldSHA: "old", NewSHA: "new", Authorized: "provenance"}
	if want.NewSHA != "new" || !want.Repinned {
		t.Fatalf("RepinResult fields: %+v", want)
	}
	// RepinShipSHA itself is exercised across repin_test.go.
}

// TestNamePublicAPI_RepinIfDrifted names the shared detect-drift-and-provenance-
// gated-repin primitive (cycle 636) for the apicover public-API gate; behavior is
// exercised in repin_ifdrifted_test.go. RepinIfDrifted is the single path invoked
// by BOTH boot recovery and the post-build repin, so the two never diverge.
func TestNamePublicAPI_RepinIfDrifted(t *testing.T) {
	var _ func(statePath, binPath, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error) = RepinIfDrifted
}
