package prompts

import "testing"

// strip_test.go — RED contract for cycle-256 task `prompt-ondemand-section-strip`.
//
// Agent docs carry a static "## Reference Index" tail (lookup tables, on-demand
// links) that is identical across cycles and re-sent on every dispatch.
// StripOnDemandSections removes that section (heading through EOF) so a compact
// prompt mode can drop the dead weight; a body WITHOUT the heading is returned
// byte-for-byte unchanged. The heading match is LINE-ANCHORED — an inline prose
// mention of "## Reference Index" must NOT trigger a strip (the anti-naive-
// substring guard; a bare strings.Index impl fails the "inline mention" case).

func TestStripOnDemandSections(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "heading after content → strip from heading to EOF",
			body: "# Agent\n\nBody text.\n\n## Reference Index\n\n- link A\n- link B\n",
			want: "# Agent\n\nBody text.\n\n",
		},
		{
			name: "heading at very start → empty result",
			body: "## Reference Index\n- only refs here\n",
			want: "",
		},
		{
			name: "no heading → unchanged byte-for-byte",
			body: "# Agent\n\nJust a body with no reference section.\n",
			want: "# Agent\n\nJust a body with no reference section.\n",
		},
		{
			name: "inline mention is NOT a heading (line-anchored)",
			body: "See the ## Reference Index section below.\nMore body.\n",
			want: "See the ## Reference Index section below.\nMore body.\n",
		},
		{
			name: "empty body → empty",
			body: "",
			want: "",
		},
		{
			name: "content with trailing newline before heading preserved",
			body: "alpha\nbeta\n## Reference Index\ngamma\n",
			want: "alpha\nbeta\n",
		},
		{
			name: "production heading with suffix → strip (Layer 3, on-demand)",
			body: "# Agent\n\nBody text.\n\n## Reference Index (Layer 3, on-demand)\n\n- ref one\n- ref two\n",
			want: "# Agent\n\nBody text.\n\n",
		},
		{
			name: "production heading at start → empty result",
			body: "## Reference Index (Layer 3, on-demand)\n- only refs here\n",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StripOnDemandSections(c.body); got != c.want {
				t.Fatalf("StripOnDemandSections(%q)\n  = %q\nwant %q", c.body, got, c.want)
			}
		})
	}
}
