package phasespec

import (
	"encoding/json"
	"strings"
	"testing"
)

// metadataRegistry exercises the advisor-facing metadata fields (ADR-0038):
// description, when_to_use, categories.
const metadataRegistry = `{
  "schema_version": 4,
  "phases": [
    {
      "name": "reproduce-bug",
      "kind": "llm",
      "optional": true,
      "archetype": "evaluate",
      "description": "Produce a failing reproduction test before any patch.",
      "when_to_use": "bugfix cycles, before tdd/build — reproduce-first anchors the fix.",
      "categories": ["bugfix"],
      "outputs": { "files": ["reproduce-bug-report.md"] }
    },
    {
      "name": "scout",
      "outputs": { "files": ["scout-report.md"] }
    }
  ]
}`

func TestLoad_MetadataFields(t *testing.T) {
	cat, err := Load(writeRegistry(t, metadataRegistry))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s, ok := cat.Get("reproduce-bug")
	if !ok {
		t.Fatal("reproduce-bug not found")
	}
	if s.Description != "Produce a failing reproduction test before any patch." {
		t.Errorf("Description = %q", s.Description)
	}
	if !strings.Contains(s.WhenToUse, "reproduce-first") {
		t.Errorf("WhenToUse = %q", s.WhenToUse)
	}
	if len(s.Categories) != 1 || s.Categories[0] != "bugfix" {
		t.Errorf("Categories = %v", s.Categories)
	}

	// A spec without metadata parses with zero values (tolerant schema).
	scout, _ := cat.Get("scout")
	if scout.Description != "" || scout.WhenToUse != "" || len(scout.Categories) != 0 {
		t.Errorf("scout metadata should be empty; got %q/%q/%v", scout.Description, scout.WhenToUse, scout.Categories)
	}
}

func TestPhaseSpec_MetadataOmittedWhenEmpty(t *testing.T) {
	raw, err := json.Marshal(PhaseSpec{Name: "x", Optional: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"description", "when_to_use", "categories"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("empty %s must be omitted from JSON; got %s", key, raw)
		}
	}
}

func TestUnknownCategories(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"all known", []string{"bugfix", "feature", "refactor", "security", "performance", "release", "docs"}, nil},
		{"domain goal types known", []string{"project-management", "business-strategy", "accounting-close", "product-discovery", "ops-incident"}, nil},
		{"adversarial-pipeline goal types known", []string{"concurrency", "api-design", "data-migration", "observability", "supply-chain", "agent-instruction", "accessibility", "frontend-ui", "i18n"}, nil},
		{"wave-5 goal types known", []string{"database", "caching", "resilience", "messaging", "infrastructure", "data-pipeline"}, nil},
		{"empty", nil, nil},
		{"one unknown", []string{"bugfix", "foo"}, []string{"foo"}},
		{"case-sensitive", []string{"Bugfix"}, []string{"Bugfix"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := UnknownCategories(PhaseSpec{Name: "x", Categories: tc.in})
			if len(got) != len(tc.want) {
				t.Fatalf("UnknownCategories(%v) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("UnknownCategories(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}
