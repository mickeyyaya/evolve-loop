package signalcenter

import "testing"

// StreamFileName is the one spelling of the per-cycle signal stream's file
// name: the root composes the sink's path with it and readers (the dashboard)
// open the same name.
func TestStreamFileName_IsTheNDJSONStream(t *testing.T) {
	if StreamFileName != "signals.ndjson" {
		t.Fatalf("StreamFileName = %q", StreamFileName)
	}
}
