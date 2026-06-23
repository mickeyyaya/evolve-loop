package research

// research_adversarial_test.go — cycle-281 test amplification.
// Targets: SearchPathsFromEnv (0%), Digest (75%), firstSentence (75%),
// SplitSearchPaths edge cases, and Query.terms() deduplication.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

// TestSearchPathsFromEnv_AllBranches — adversarial: the three branches of
// SearchPathsFromEnv (cfg set/non-empty, cfg empty, cfg whitespace-only).
func TestSearchPathsFromEnv_AllBranches(t *testing.T) {
	t.Run("cfg.KBSearchPaths set to explicit paths returns those paths", func(t *testing.T) {
		cfg := policy.PathsConfig{KBSearchPaths: "/a/b:/c/d"}
		paths := SearchPathsFromEnv(cfg)
		if len(paths) != 2 || paths[0] != "/a/b" || paths[1] != "/c/d" {
			t.Errorf("SearchPathsFromEnv with explicit value = %v, want [/a/b /c/d]", paths)
		}
	})
	t.Run("cfg.KBSearchPaths empty falls back to default paths", func(t *testing.T) {
		paths := SearchPathsFromEnv(policy.PathsConfig{})
		if len(paths) == 0 {
			t.Error("empty cfg must fall back to default paths (not empty)")
		}
	})
	t.Run("cfg.KBSearchPaths whitespace-only falls back to default paths", func(t *testing.T) {
		cfg := policy.PathsConfig{KBSearchPaths: "   "}
		paths := SearchPathsFromEnv(cfg)
		if len(paths) == 0 {
			t.Error("whitespace-only cfg must fall back to default paths (not empty)")
		}
	})
	t.Run("empty PathsConfig falls back to default paths with knowledge-base", func(t *testing.T) {
		paths := SearchPathsFromEnv(policy.PathsConfig{})
		found := false
		for _, p := range paths {
			if strings.Contains(p, "knowledge-base") || strings.Contains(p, "instincts") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("default paths must reference knowledge-base or instincts; got %v", paths)
		}
	})
}

// TestSplitSearchPaths_EdgeCases — adversarial: empty string, colons only,
// whitespace-padded entries, and mixed empty/valid entries.
func TestSplitSearchPaths_EdgeCases(t *testing.T) {
	cases := []struct {
		raw     string
		wantLen int
	}{
		{"", 0},
		{":", 0},             // colon-only splits to two empties → both dropped
		{"::", 0},            // all empties
		{"/a:/b", 2},         // normal case
		{"/a::/b", 2},        // empty middle entry dropped
		{"  /a  :  /b  ", 2}, // whitespace-trimmed entries
		{"/single", 1},       // no colon
	}
	for _, c := range cases {
		got := SplitSearchPaths(c.raw)
		if len(got) != c.wantLen {
			t.Errorf("SplitSearchPaths(%q) = %v (len=%d), want len=%d", c.raw, got, len(got), c.wantLen)
		}
	}
}

// TestFirstSentence_AllBranches — adversarial: empty string, no sentence
// terminator, terminator at position 0 (edge), and multi-sentence strings.
func TestFirstSentence_AllBranches(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"One sentence.", "One sentence"},
		{"First sentence. Second sentence.", "First sentence"},
		{"No terminator here", "No terminator here"},
		{"  Leading whitespace. More.", "Leading whitespace"},
		{"Line\nbreak here", "Line"},
		// Edge: period is position 0 — IndexAny returns 0 which is NOT > 0,
		// so the whole trimmed string is returned.
		{". starts with period", ". starts with period"},
	}
	for _, c := range cases {
		got := firstSentence(c.input)
		if got != c.want {
			t.Errorf("firstSentence(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestLessonDigest_AllPaths — adversarial: Digest uses PreventiveAction when
// set, falls back to Description when empty. Both paths must be covered.
func TestLessonDigest_AllPaths(t *testing.T) {
	t.Run("PreventiveAction set → used in digest", func(t *testing.T) {
		l := Lesson{
			ID:               "inst-L099",
			Pattern:          "test-pattern",
			Description:      "Description text.",
			PreventiveAction: "Run the test suite first.",
		}
		digest := l.Digest()
		if !strings.Contains(digest, "inst-L099") {
			t.Errorf("Digest must start with the ID; got %q", digest)
		}
		if !strings.Contains(digest, "Run the test suite first") {
			t.Errorf("Digest must use PreventiveAction; got %q", digest)
		}
		if strings.Contains(digest, "Description text") {
			t.Errorf("Digest must NOT use Description when PreventiveAction is set; got %q", digest)
		}
	})
	t.Run("empty PreventiveAction falls back to Description", func(t *testing.T) {
		l := Lesson{
			ID:               "inst-L100",
			Pattern:          "fallback-pattern",
			Description:      "Description text is the fallback.",
			PreventiveAction: "",
		}
		digest := l.Digest()
		if !strings.Contains(digest, "inst-L100") {
			t.Errorf("Digest must include the ID; got %q", digest)
		}
		if !strings.Contains(digest, "Description text is the fallback") {
			t.Errorf("Digest must fall back to Description; got %q", digest)
		}
	})
	t.Run("Digest output is one line (no embedded newlines)", func(t *testing.T) {
		l := Lesson{
			ID:               "inst-L101",
			Pattern:          "multiline-pattern",
			PreventiveAction: "First line.\nSecond line.",
		}
		digest := l.Digest()
		// firstSentence trims at \n, so the digest must be single-line.
		if strings.Contains(digest, "\n") {
			t.Errorf("Digest must be single-line; got %q", digest)
		}
	})
}

// TestQueryTerms_Deduplication — adversarial: duplicate tokens across
// FailureMode/Consequence/Keywords must appear exactly once in terms().
func TestQueryTerms_Deduplication(t *testing.T) {
	q := Query{
		FailureMode: "egps red count",
		Consequence: "egps gate fail",                 // "egps" already in FailureMode
		Keywords:    []string{"red", "egps", "count"}, // all already seen
	}
	terms := q.terms()
	seen := map[string]int{}
	for _, t := range terms {
		seen[t]++
	}
	for tok, count := range seen {
		if count > 1 {
			t.Errorf("term %q appears %d times; want exactly 1 (dedup contract)", tok, count)
		}
	}
}

// TestQueryTerms_ShortTokensDropped — adversarial: tokens shorter than 3 chars
// must be silently dropped (the noise filter).
func TestQueryTerms_ShortTokensDropped(t *testing.T) {
	q := Query{
		FailureMode: "a b cd efg hijklmn",
		Keywords:    []string{"xy"},
	}
	terms := q.terms()
	for _, term := range terms {
		if len(term) < 3 {
			t.Errorf("term %q has length < 3; noise tokens must be dropped", term)
		}
	}
	// "efg" and "hijklmn" are ≥3 and must be present.
	found := map[string]bool{}
	for _, term := range terms {
		found[term] = true
	}
	if !found["efg"] {
		t.Errorf("term 'efg' (len=3) should survive the noise filter; got %v", terms)
	}
	if !found["hijklmn"] {
		t.Errorf("term 'hijklmn' should survive the noise filter; got %v", terms)
	}
}
