package prompts

import (
	"strings"
	"testing"
)

func TestStripOnDemandSections_Adversarial(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "suffix-without-space-not-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index(no-space)\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n## Reference Index(no-space)\n\n- ref\n",
		},
		{
			name: "multiple-headings-strip-from-first",
			body: "# Agent\n\nBody.\n\n## Reference Index\n\nfirst\n\n## Reference Index (Layer 3, on-demand)\n\nsecond\n",
			want: "# Agent\n\nBody.\n\n",
		},
		{
			name: "bare-heading-at-eof-no-trailing-newline",
			body: "Body.\n## Reference Index",
			want: "Body.\n",
		},
		{
			name: "production-heading-at-eof-no-trailing-newline",
			body: "Body.\n## Reference Index (Layer 3, on-demand)",
			want: "Body.\n",
		},
		{
			name: "lowercase-heading-not-stripped",
			body: "# Agent\n\nBody.\n\n## reference index\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n## reference index\n\n- ref\n",
		},
		{
			name: "tab-separator-not-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index\t(Legacy)\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n## Reference Index\t(Legacy)\n\n- ref\n",
		},
		{
			name: "non-standard-suffix-stripped",
			body: "# Agent\n\nBody.\n\n## Reference Index (Legacy)\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n",
		},
		{
			name: "h3-heading-not-stripped",
			body: "# Agent\n\nBody.\n\n### Reference Index\n\n- ref\n",
			want: "# Agent\n\nBody.\n\n### Reference Index\n\n- ref\n",
		},
		{
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
