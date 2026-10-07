package modelquery

import (
	"reflect"
	"slices"
	"testing"
)

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

func TestFamilyOf_UnknownReturnsEmpty(t *testing.T) {
	for _, id := range []string{"llama3.1", "mistral-large", "some-vendor-x", ""} {
		if got := FamilyOf(id); got != "" {
			t.Errorf("FamilyOf(%q) = %q, want \"\" (unknown family)", id, got)
		}
	}
}

func TestFilterByFamily_GeminiOnlyDropsClaudeAndGPT(t *testing.T) {
	in := []string{"Gemini 3.1 Pro (High)", "GPT-OSS-120B", "Sonnet 4.6"}
	got := FilterByFamily(in, "gemini")
	want := []string{"Gemini 3.1 Pro (High)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterByFamily(%v, \"gemini\") = %v, want %v", in, got, want)
	}
}

func TestFilterByFamily_NoGeminiYieldsEmptyNotPassthrough(t *testing.T) {
	in := []string{"opus-4.8", "sonnet-4.6"}
	got := FilterByFamily(in, "gemini")
	if len(got) != 0 {
		t.Errorf("FilterByFamily(%v, \"gemini\") = %v, want empty (no gemini model → nothing survives, NOT passthrough)", in, got)
	}
}

func TestFilterByFamily_EmptyAllowedIsPassthrough(t *testing.T) {
	in := []string{"opus-4.8", "llama3.1"}
	got := FilterByFamily(in)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("FilterByFamily(%v) with no allowed families = %v, want the input unchanged", in, got)
	}
}

func TestFilterByFamily_MultiFamilyKeepsOrderDropsUnknown(t *testing.T) {
	in := []string{"gpt-5.5", "mystery-x", "opus-4.8", "gpt-4.1"}
	got := FilterByFamily(in, "gpt", "claude")
	want := []string{"gpt-5.5", "opus-4.8", "gpt-4.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterByFamily(%v, \"gpt\",\"claude\") = %v, want %v (order preserved, unknown dropped)", in, got, want)
	}
}

func TestFilterByFamily_ComposesWithNewestWins(t *testing.T) {
	in := []string{"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro", "GPT-OSS-120B"}
	pure := FilterByFamily(in, "gemini")
	if got := NewestInLineage(pure); got != "Gemini 3.5 Pro" {
		t.Errorf("NewestInLineage(FilterByFamily(%v, \"gemini\")) = %q, want %q (family-filter then newest-wins promotes the frontier Gemini)", in, got, "Gemini 3.5 Pro")
	}
}

func TestFamiliesIn_NamesEveryFamilyALabelMentionsInTableOrder(t *testing.T) {
	for label, want := range map[string][]string{
		"Claude Opus 5.5 · high":                       {"claude"},
		"Gemini 3.8 Flash · low":                       {"gemini"},
		"Gemini 3.1 Pro (Claude Opus 5.5 unavailable)": {"claude", "gemini"},
		"gpt-5.6-sol":                                  {"gpt"},
		"bypass permissions on (shift+tab to cycle)":   nil,
	} {
		if got := FamiliesIn(label); !slices.Equal(got, want) {
			t.Errorf("FamiliesIn(%q) = %v, want %v", label, got, want)
		}
	}
}
