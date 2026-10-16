package phaseintegrity

import "testing"

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

func TestNamePublicAPI_Repin(t *testing.T) {
	var _ ProvenanceVerified = func(string) bool { return true }
	want := RepinResult{Repinned: true, OldSHA: "old", NewSHA: "new", Authorized: "provenance"}
	if want.NewSHA != "new" || !want.Repinned {
		t.Fatalf("RepinResult fields: %+v", want)
	}
}

func TestNamePublicAPI_RepinIfDrifted(t *testing.T) {
	var _ func(statePath, binPath, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error) = RepinIfDrifted
}
