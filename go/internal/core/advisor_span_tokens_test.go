package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestAdvisorSpan_CarriesTokenUsage — S5, token-telemetry. The advisor decision
// span (advisor-span-<kind>.json) must persist the advisor call's LLM token
// usage, not just DurationMS. Previously advisor token burn was dropped on the
// floor (non-phase attribution gap). This drives captureRedacted → span JSON.
func TestAdvisorSpan_CarriesTokenUsage(t *testing.T) {
	written := map[string][]byte{}
	p := &PhaseAdvisor{
		writeArtifact: func(path string, data []byte) error {
			written[filepath.Base(path)] = data
			return nil
		},
	}
	want := TokenUsage{Input: 1200, Output: 340, CacheRead: 80, CacheWrite: 16}

	p.captureRedacted(t.TempDir(), "plan", "prompt", "response", 4200, 1, want)

	raw, ok := written["advisor-span-plan.json"]
	if !ok {
		t.Fatalf("advisor-span-plan.json not written; got %v", keysOf(written))
	}
	var span AdvisorSpan
	if err := json.Unmarshal(raw, &span); err != nil {
		t.Fatalf("unmarshal span: %v", err)
	}
	if span.Tokens != want {
		t.Fatalf("span.Tokens = %+v, want %+v", span.Tokens, want)
	}
}

func keysOf(m map[string][]byte) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
