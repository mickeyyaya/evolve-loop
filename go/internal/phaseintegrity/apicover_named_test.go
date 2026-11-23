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
	var prov ProvenanceVerified = func(commit string) bool { return commit == "abc123" }
	p := seedStateJSON(t, t.TempDir(), nil)
	got, err := RepinShipSHA(p, "NEW_SHA", "abc123", "21.1.1", prov, false)
	if err != nil {
		t.Fatal(err)
	}
	want := RepinResult{Repinned: true, OldSHA: "OLD_SHA", NewSHA: "NEW_SHA", Authorized: "provenance"}
	if got != want {
		t.Errorf("RepinShipSHA = %+v, want %+v", got, want)
	}
}

func TestNamePublicAPI_RepinIfDrifted(t *testing.T) {
	binBytes := []byte("\x7fELF-drifted")
	statePath, binPath := writeRepinFixture(t, t.TempDir(), "STALE_PIN", binBytes)
	var prov ProvenanceVerified = func(commit string) bool { return commit == "good" }
	got, err := RepinIfDrifted(statePath, binPath, "good", "22.0.1", prov)
	if err != nil {
		t.Fatal(err)
	}
	want := RepinResult{Repinned: true, OldSHA: "STALE_PIN", NewSHA: sha256HexBytes(binBytes), Authorized: "provenance"}
	if got != want {
		t.Errorf("RepinIfDrifted = %+v, want %+v", got, want)
	}
	statePath, binPath = writeRepinFixture(t, t.TempDir(), "STALE_PIN", binBytes)
	if _, err := RepinIfDrifted(statePath, binPath, "bad", "22.0.1", prov); err == nil {
		t.Error("an unverified build commit must be refused")
	}
}
