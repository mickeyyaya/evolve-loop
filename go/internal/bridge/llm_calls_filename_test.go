package bridge

import "testing"

func TestLLMCallsLogFilename(t *testing.T) {
	if LLMCallsLogFilename != "llm-calls.ndjson" {
		t.Fatalf("LLMCallsLogFilename = %q; the recorded runs on disk carry llm-calls.ndjson", LLMCallsLogFilename)
	}
}
