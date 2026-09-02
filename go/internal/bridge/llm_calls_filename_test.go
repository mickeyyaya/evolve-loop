package bridge

import "testing"

// TestLLMCallsLogFilename pins the dispatch-ledger name the engine writes and
// every reader (tokens report, models live, the dashboard) resolves.
func TestLLMCallsLogFilename(t *testing.T) {
	if LLMCallsLogFilename != "llm-calls.ndjson" {
		t.Fatalf("LLMCallsLogFilename = %q; the recorded runs on disk carry llm-calls.ndjson", LLMCallsLogFilename)
	}
}
