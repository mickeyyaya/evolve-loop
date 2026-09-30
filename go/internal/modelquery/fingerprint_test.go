package modelquery

import (
	"strings"
	"testing"
)

func TestFingerprint_DeterministicAndPrefixed(t *testing.T) {
	t.Parallel()
	in := FingerprintInput{
		CLI:        "agy",
		Candidates: []string{"Gemini 3.5 Pro (High)", "Gemini 3.5 Flash (Medium)"},
		Policy:     FreshnessPolicy{},
		Tiers:      []string{"fast", "balanced", "deep", "top"},
	}
	a, b := Fingerprint(in), Fingerprint(in)
	if a != b {
		t.Fatalf("Fingerprint not deterministic: %q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "sha256:") || len(a) != len("sha256:")+64 {
		t.Fatalf("Fingerprint rendering = %q, want sha256:<64 hex>", a)
	}
}

func TestFingerprint_CandidateOrderInsensitive(t *testing.T) {
	t.Parallel()
	base := FingerprintInput{CLI: "codex", Candidates: []string{"gpt-5.5", "gpt-5.5-mini"}, Tiers: []string{"fast", "deep"}}
	shuffled := FingerprintInput{CLI: "codex", Candidates: []string{"gpt-5.5-mini", "gpt-5.5"}, Tiers: []string{"fast", "deep"}}
	if Fingerprint(base) != Fingerprint(shuffled) {
		t.Error("candidate order changed the fingerprint")
	}
	if shuffled.Candidates[0] != "gpt-5.5-mini" {
		t.Error("Fingerprint mutated its input slice")
	}
}

func TestFingerprint_SensitiveToEveryField(t *testing.T) {
	t.Parallel()
	base := FingerprintInput{
		CLI:        "claude",
		Candidates: []string{"opus", "sonnet"},
		Policy:     FreshnessPolicy{PreferAlias: true, AliasIDs: []string{"opus", "sonnet"}},
		Tiers:      []string{"fast", "balanced", "deep", "top"},
	}
	ref := Fingerprint(base)
	variants := []FingerprintInput{
		{CLI: "codex", Candidates: base.Candidates, Policy: base.Policy, Tiers: base.Tiers},
		{CLI: base.CLI, Candidates: []string{"opus", "sonnet", "haiku"}, Policy: base.Policy, Tiers: base.Tiers},
		{CLI: base.CLI, Candidates: base.Candidates, Policy: FreshnessPolicy{}, Tiers: base.Tiers},
		{CLI: base.CLI, Candidates: base.Candidates, Policy: base.Policy, Tiers: []string{"fast", "balanced", "deep"}},
	}
	for i, v := range variants {
		if Fingerprint(v) == ref {
			t.Errorf("variant %d hashed identically to base — field not covered", i)
		}
	}
}

func TestFingerprint_FramingUnambiguous(t *testing.T) {
	t.Parallel()
	a := FingerprintInput{CLI: "x", Candidates: []string{"ab", "c"}}
	b := FingerprintInput{CLI: "x", Candidates: []string{"a", "bc"}}
	if Fingerprint(a) == Fingerprint(b) {
		t.Error(`["ab","c"] and ["a","bc"] collided — framing is ambiguous`)
	}
	c := FingerprintInput{CLI: "xa", Candidates: []string{"b", "c"}}
	if Fingerprint(a) == Fingerprint(c) {
		t.Error("CLI/candidate boundary is ambiguous")
	}
}
