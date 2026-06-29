package prompts

import (
	"strings"
	"testing"
)

// strip_amplified_test.go — Adversarial amplification for StripOnDemandSections.
//
// Probes boundary conditions of the line-anchored prefix-match rule (cycle-413 Task A).
// Distinct from strip_test.go's 8 cases; targets failure modes a naive strings.Contains
// or wrong-space-delimiter implementation would exhibit.

func TestStripOnDemandSections_Adversarial(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// HasPrefix requires "## Reference Index " (trailing space).
			// A suffix glued directly to "Index" with no space must NOT strip.
			name: "suffix-without-space-not-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index(no-space)\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n## Reference Index(no-space)\n\n- ref\n",
		},
		{
			// Two headings: strip from first occurrence, discard everything after.
			name: "multiple-headings-strip-from-first",
			body: "# Agent\n\nBody.\n\n## Reference Index\n\nfirst\n\n## Reference Index (Layer 3, on-demand)\n\nsecond\n",
			want: "# Agent\n\nBody.\n\n",
		},
		{
			// Heading at EOF with no trailing newline (bare form).
			name: "bare-heading-at-eof-no-trailing-newline",
			body: "Body.\n## Reference Index",
			want: "Body.\n",
		},
		{
			// Heading at EOF with no trailing newline (production form).
			name: "production-heading-at-eof-no-trailing-newline",
			body: "Body.\n## Reference Index (Layer 3, on-demand)",
			want: "Body.\n",
		},
		{
			// Match is case-sensitive; lowercase must not trigger strip.
			name: "lowercase-heading-not-stripped",
			body: "# Agent\n\nBody.\n\n## reference index\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n## reference index\n\n- ref\n",
		},
		{
			// Tab between "Index" and suffix is not a space — must NOT strip.
			name: "tab-separator-not-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index\t(Legacy)\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n## Reference Index\t(Legacy)\n\n- ref\n",
		},
		{
			// Any suffix after a space is stripped — rule is suffix-agnostic.
			name: "non-standard-suffix-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index (Legacy)\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n",
		},
		{
			// H3 "###" must not strip; only H2 "##" is the section marker.
			name: "h3-heading-not-stripped",
			body: "# Agent\n\nBody.\n\n### Reference Index\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n### Reference Index\n\n- ref\n",
		},
		{
			// Single-character suffix after space: "## Reference Index X" → stripped.
			name: "single-char-suffix-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index X\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StripOnDemandSections(c.body)
			if got != c.want {
				t.Fatalf("StripOnDemandSections(%q)\n  got  %q\n  want %q", c.body, got, c.want)
			}
		})
	}
}

// TestStripOnDemandSections_Idempotent verifies that applying the strip twice
// yields the same result as applying it once. Guards against partial-line residue.
func TestStripOnDemandSections_Idempotent(t *testing.T) {
	bodies := []string{
		"# Agent\n\nBody.\n\n## Reference Index\n\n- ref\n",
		"# Agent\n\nBody.\n\n## Reference Index (Layer 3, on-demand)\n\n- ref\n",
		"Just body text, no heading at all.\n",
		"",
	}
	for _, body := range bodies {
		once := StripOnDemandSections(body)
		twice := StripOnDemandSections(once)
		if once != twice {
			t.Errorf("not idempotent for %q:\n  first  %q\n  second %q", body, once, twice)
		}
	}
}

// TestStripOnDemandSections_OutputContainsNoHeading asserts that after stripping,
// no line in the result starts with "## Reference Index". Output-contract test.
func TestStripOnDemandSections_OutputContainsNoHeading(t *testing.T) {
	inputs := []string{
		"# Agent\n\nBody.\n\n## Reference Index\n\n- ref\n",
		"# Agent\n\nBody.\n\n## Reference Index (Layer 3, on-demand)\n\n- ref one\n- ref two\n",
		"## Reference Index (Layer 3, on-demand)\n- only refs\n",
	}
	for _, body := range inputs {
		stripped := StripOnDemandSections(body)
		for _, line := range strings.Split(stripped, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "## Reference Index") {
				t.Errorf("stripped body still contains heading line %q\n  full result: %q", line, stripped)
			}
		}
	}
}
