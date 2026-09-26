package policy

import (
	"encoding/json"
	"testing"
)

func assertChronicle(t *testing.T, got ChronicleConfig, digest string, tokens, cycles int, escalation, historian string) {
	t.Helper()
	if got.Digest != digest {
		t.Errorf("Digest = %q, want %q", got.Digest, digest)
	}
	if got.DigestTokens != tokens {
		t.Errorf("DigestTokens = %d, want %d", got.DigestTokens, tokens)
	}
	if got.DigestCycles != cycles {
		t.Errorf("DigestCycles = %d, want %d", got.DigestCycles, cycles)
	}
	if got.Escalation != escalation {
		t.Errorf("Escalation = %q, want %q", got.Escalation, escalation)
	}
	if got.Historian != historian {
		t.Errorf("Historian = %q, want %q", got.Historian, historian)
	}
}

func TestChronicleConfig_CompiledDefaults(t *testing.T) {
	var p Policy
	assertChronicle(t, p.ChronicleConfig(), "shadow", 1200, 10, "shadow", "off")

	p = Policy{Chronicle: &ChroniclePolicy{}}
	assertChronicle(t, p.ChronicleConfig(), "shadow", 1200, 10, "shadow", "off")
}

func TestChronicleConfig_PolicyOverrides(t *testing.T) {
	var p Policy
	doc := `{"chronicle":{"digest":"enforce","digest_tokens":800,"digest_cycles":5,"escalation":"enforce","historian":"shadow"}}`
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatalf("unmarshal policy doc: %v", err)
	}
	if p.Chronicle == nil {
		t.Fatalf("chronicle block did not unmarshal — Policy.Chronicle json tag missing")
	}
	assertChronicle(t, p.ChronicleConfig(), "enforce", 800, 5, "enforce", "shadow")

	var partial Policy
	if err := json.Unmarshal([]byte(`{"chronicle":{"digest":"enforce"}}`), &partial); err != nil {
		t.Fatalf("unmarshal partial policy doc: %v", err)
	}
	assertChronicle(t, partial.ChronicleConfig(), "enforce", 1200, 10, "shadow", "off")
}
