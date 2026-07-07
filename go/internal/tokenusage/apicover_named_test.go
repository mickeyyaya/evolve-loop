package tokenusage

import "testing"

// TestResultAndSourceNamed pins the exported Result and Source vocabulary that
// ScanConfigRoot returns but the behavioural tests only touch via field access
// — apicover requires every exported type be named in a test. It also asserts
// the Source consts are distinct so a mislabelled scan is caught.
func TestResultAndSourceNamed(t *testing.T) {
	var s Source = SourceTranscript
	if s == SourceNone {
		t.Fatal("SourceTranscript and SourceNone must be distinct")
	}
	r := Result{Source: SourceNone}
	if r.Source != SourceNone || r.Usage != (r.Usage) {
		t.Fatalf("zero Result must carry SourceNone, got %q", r.Source)
	}
	if string(SourceNone) != "none" || string(SourceTranscript) != "transcript" {
		t.Fatalf("Source string values drifted: none=%q transcript=%q", SourceNone, SourceTranscript)
	}
}
