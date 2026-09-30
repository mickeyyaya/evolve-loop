package modelquery

import (
	"reflect"
	"testing"
)

func TestLineageKey_RealCatalogIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id, want string
	}{
		{"Gemini 3.5 Flash (Medium)", "gemini-flash-(medium)"},
		{"Gemini 3.1 Pro (High)", "gemini-pro-(high)"},
		{"Gemini 3.5 Pro (High)", "gemini-pro-(high)"},
		{"gpt-5.5", "gpt"},
		{"gpt-5.5-mini", "gpt-mini"},
		{"llama3.1:8b", "llama:8b"},
		{"llama3.3:8b", "llama:8b"},
		{"llama3.3:latest", "llama:latest"},
		{"opus", "opus"},
		{"opus-4.6", "opus"},
		{"sonnet", "sonnet"},
	}
	for _, c := range cases {
		if got := LineageKey(c.id); got != c.want {
			t.Errorf("LineageKey(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestLineageKey_SeparatesCapabilityClasses(t *testing.T) {
	t.Parallel()
	mustDiffer := [][2]string{
		{"Gemini 3.5 Flash (Medium)", "Gemini 3.1 Pro (High)"},
		{"gpt-5.5", "gpt-5.5-mini"},
		{"llama3.1:8b", "llama3.3:latest"},
		{"llama3.1:8b", "llama3.1:70b"},
		{"qwen2.5-coder:32b", "qwen2.5-coder:70b"},
	}
	for _, p := range mustDiffer {
		if LineageKey(p[0]) == LineageKey(p[1]) {
			t.Errorf("LineageKey(%q) == LineageKey(%q) = %q — capability classes collided", p[0], p[1], LineageKey(p[0]))
		}
	}
	mustMatch := [][2]string{
		{"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro (High)"},
		{"opus", "opus-4.6"},
		{"llama3.1:8b", "llama3.3:8b"},
		{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"},
	}
	for _, p := range mustMatch {
		if LineageKey(p[0]) != LineageKey(p[1]) {
			t.Errorf("LineageKey(%q)=%q != LineageKey(%q)=%q — version siblings split", p[0], LineageKey(p[0]), p[1], LineageKey(p[1]))
		}
	}
}

func TestGroupByLineage_OrderPreservingBuckets(t *testing.T) {
	t.Parallel()
	ids := []string{
		"Gemini 3.1 Pro (High)",
		"Gemini 3.5 Flash (Medium)",
		"Gemini 3.5 Pro (High)",
		"Gemini 3.1 Flash (Medium)",
	}
	got := GroupByLineage(ids)
	want := map[string][]string{
		"gemini-pro-(high)":     {"Gemini 3.1 Pro (High)", "Gemini 3.5 Pro (High)"},
		"gemini-flash-(medium)": {"Gemini 3.5 Flash (Medium)", "Gemini 3.1 Flash (Medium)"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GroupByLineage = %#v, want %#v", got, want)
	}
	if len(GroupByLineage(nil)) != 0 {
		t.Error("GroupByLineage(nil) should be empty")
	}
}
