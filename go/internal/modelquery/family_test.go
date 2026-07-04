package modelquery

import (
	"reflect"
	"testing"
)

// family_test.go is the RED contract for design point D7 (FAMILY CONSTRAINT) of
// the latest-model-preference feature. It pins the two pure, deterministic Go
// helpers a family-pure candidate set needs BEFORE classification / newest-wins:
//
//	FamilyOf(id) string              — classify a raw model id into a family
//	FilterByFamily(ids, allowed...)  — keep only ids in an allowed family
//
// Live evidence motivating D7 (inbox 2026-07-02 latest-model-preference): the
// agy classifier FLAPPED an identical list Sonnet-4.6→GPT-OSS-120B and agy's
// tier map carried Claude/GPT-OSS models — a family violation for a Gemini-only
// CLI. Cross-family coverage belongs to the cli_fallback chain, never a CLI's
// own tier map. A deterministic family filter makes that purity structural
// instead of an operator hand-correction. Authored by the TDD engineer — the
// Builder implements internal/modelquery/family.go and must NOT modify this file.

// TestFamilyOf_ClassifiesKnownFamilies pins the family vocabulary used to
// enforce per-CLI purity: Anthropic ids (incl. the fable frontier) → "claude",
// OpenAI/GPT ids → "gpt", Google ids → "gemini". Case- and format-insensitive
// so display-name tokens ("Gemini 3.1 Pro (High)") classify the same as bare
// ids ("gemini-3.1").
func TestFamilyOf_ClassifiesKnownFamilies(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"opus-4.8", "claude"},
		{"sonnet-4.6", "claude"},
		{"haiku-4.5", "claude"},
		{"claude-fable-5", "claude"},
		{"fable", "claude"},
		{"GPT-OSS-120B", "gpt"},
		{"gpt-5.5", "gpt"},
		{"Gemini 3.1 Pro (High)", "gemini"},
		{"Gemini 3.5 Flash (Low)", "gemini"},
		{"gemini-3.1", "gemini"},
	}
	for _, tc := range cases {
		if got := FamilyOf(tc.id); got != tc.want {
			t.Errorf("FamilyOf(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// TestFamilyOf_UnknownReturnsEmpty is the negative/edge axis: an id in no known
// LLM family (a local Ollama id or a novel vendor) returns "" rather than being
// force-fit into a family. Under a family constraint, "" ids are dropped
// (conservative — only proven-family ids survive); with no constraint they pass
// through (see TestFilterByFamily_EmptyAllowedIsPassthrough).
func TestFamilyOf_UnknownReturnsEmpty(t *testing.T) {
	for _, id := range []string{"llama3.1", "mistral-large", "some-vendor-x", ""} {
		if got := FamilyOf(id); got != "" {
			t.Errorf("FamilyOf(%q) = %q, want \"\" (unknown family)", id, got)
		}
	}
}

// TestFilterByFamily_GeminiOnlyDropsClaudeAndGPT is the POSITIVE bug-fix
// predicate: the exact agy live-evidence scenario. A mixed candidate list
// containing a cross-family GPT-OSS id and a Claude id, filtered to the gemini
// family, keeps ONLY the Gemini id. This is what stops agy's tier map from ever
// carrying a Claude/GPT model again.
func TestFilterByFamily_GeminiOnlyDropsClaudeAndGPT(t *testing.T) {
	in := []string{"Gemini 3.1 Pro (High)", "GPT-OSS-120B", "Sonnet 4.6"}
	got := FilterByFamily(in, "gemini")
	want := []string{"Gemini 3.1 Pro (High)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterByFamily(%v, \"gemini\") = %v, want %v", in, got, want)
	}
}

// TestFilterByFamily_NoGeminiYieldsEmptyNotPassthrough is the strongest
// anti-no-op signal (adversarial §2): a stub that returns its input unchanged
// would PASS the positive test above by accident when the input is already
// mostly gemini, but here — a list with NO gemini model, filtered to gemini —
// the only correct answer is an empty result. A passthrough no-op returns the
// two Claude ids and fails loudly.
func TestFilterByFamily_NoGeminiYieldsEmptyNotPassthrough(t *testing.T) {
	in := []string{"opus-4.8", "sonnet-4.6"}
	got := FilterByFamily(in, "gemini")
	if len(got) != 0 {
		t.Errorf("FilterByFamily(%v, \"gemini\") = %v, want empty (no gemini model → nothing survives, NOT passthrough)", in, got)
	}
}

// TestFilterByFamily_EmptyAllowedIsPassthrough is the edge axis for an
// unconstrained CLI (ollama: allowed_families = [local/any]). With no allowed
// families the filter imposes no constraint and returns the input verbatim,
// order preserved — including unknown-family ids.
func TestFilterByFamily_EmptyAllowedIsPassthrough(t *testing.T) {
	in := []string{"opus-4.8", "llama3.1"}
	got := FilterByFamily(in)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("FilterByFamily(%v) with no allowed families = %v, want the input unchanged", in, got)
	}
}

// TestFilterByFamily_MultiFamilyKeepsOrderDropsUnknown is the semantic axis: a
// multi-family allow set (a CLI whose purity spans two families) keeps ids from
// EITHER allowed family, preserves input order, and drops an unknown-family id.
func TestFilterByFamily_MultiFamilyKeepsOrderDropsUnknown(t *testing.T) {
	in := []string{"gpt-5.5", "mystery-x", "opus-4.8", "gpt-4.1"}
	got := FilterByFamily(in, "gpt", "claude")
	want := []string{"gpt-5.5", "opus-4.8", "gpt-4.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterByFamily(%v, \"gpt\",\"claude\") = %v, want %v (order preserved, unknown dropped)", in, got, want)
	}
}

// TestFilterByFamily_ComposesWithNewestWins encodes the D2+D7+D8 acceptance
// scenario from the inbox spec: "when agy's picker lists Gemini 3.5 Pro,
// newest-wins must promote agy top automatically." Family-filter first strips
// the cross-family GPT id, THEN the existing NewestInLineage comparator picks
// the newest surviving Gemini — proving the two deterministic steps compose to
// the frontier model without any classifier call.
func TestFilterByFamily_ComposesWithNewestWins(t *testing.T) {
	in := []string{"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro", "GPT-OSS-120B"}
	pure := FilterByFamily(in, "gemini")
	if got := NewestInLineage(pure); got != "Gemini 3.5 Pro" {
		t.Errorf("NewestInLineage(FilterByFamily(%v, \"gemini\")) = %q, want %q (family-filter then newest-wins promotes the frontier Gemini)", in, got, "Gemini 3.5 Pro")
	}
}
