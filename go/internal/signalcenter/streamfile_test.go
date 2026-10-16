package signalcenter

import "testing"

func TestStreamFileName_IsTheNDJSONStream(t *testing.T) {
	if StreamFileName != "signals.ndjson" {
		t.Fatalf("StreamFileName = %q", StreamFileName)
	}
}
