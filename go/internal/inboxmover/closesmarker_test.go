package inboxmover

import (
	"reflect"
	"testing"
)

func TestClosesInboxIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single marker line",
			body: "# Build Report\n\nDid the work.\n\nCloses-Inbox: schema-aligned-salvage-layer\n",
			want: []string{"schema-aligned-salvage-layer"},
		},
		{
			name: "comma separated ids on one line",
			body: "Closes-Inbox: alpha-item, beta.item, gamma_item\n",
			want: []string{"alpha-item", "beta.item", "gamma_item"},
		},
		{
			name: "markdown bullet and backticked id",
			body: "## Handoff\n\n- Closes-Inbox: `consumption-rides-landing-ship`\n* Closes-Inbox: second-item\n",
			want: []string{"consumption-rides-landing-ship", "second-item"},
		},
		{
			name: "marker is case insensitive and CRLF tolerant",
			body: "closes-inbox: lower-item\r\nCLOSES-INBOX: shouty-item\r\n",
			want: []string{"lower-item", "shouty-item"},
		},
		{
			name: "space-indented marker is quoted text, not a claim",
			body: "  Closes-Inbox: indented-item\n",
			want: nil,
		},
		{
			name: "tab-indented marker is quoted text, not a claim",
			body: "\tCloses-Inbox: tabbed-item\n",
			want: nil,
		},
		{
			name: "nested bullet marker is quoted text, not a claim",
			body: "- parent\n  - Closes-Inbox: nested-item\n",
			want: nil,
		},
		{
			name: "marker inside a backtick fence is quoted text",
			body: "```\nCloses-Inbox: fenced-item\n```\n",
			want: nil,
		},
		{
			name: "marker inside a tilde fence is quoted text",
			body: "~~~text\nCloses-Inbox: tilde-item\n~~~\n",
			want: nil,
		},
		{
			name: "marker inside an unclosed fence stays quoted to the end",
			body: "```\nCloses-Inbox: unclosed-item\n",
			want: nil,
		},
		{
			name: "fence closer must match the opener character",
			body: "```\n~~~\nCloses-Inbox: still-fenced\n```\n",
			want: nil,
		},
		{
			name: "info-string fence opener still opens a fence",
			body: "```markdown\n- Closes-Inbox: fenced-bullet\n```\n",
			want: nil,
		},
		{
			name: "column-zero marker after a closed fence is a claim",
			body: "```\nCloses-Inbox: fenced-item\n```\nCloses-Inbox: real-item\n",
			want: []string{"real-item"},
		},
		{
			name: "column-zero marker before a fence is a claim",
			body: "Closes-Inbox: real-item\n```\nCloses-Inbox: fenced-item\n```\n",
			want: []string{"real-item"},
		},
		{
			name: "column-zero bullet marker is still a claim",
			body: "- Closes-Inbox: bullet-item\n* Closes-Inbox: star-item\n+ Closes-Inbox: plus-item\n",
			want: []string{"bullet-item", "star-item", "plus-item"},
		},
		{
			name: "blockquoted marker is quoted text",
			body: "> Closes-Inbox: quoted-item\n",
			want: nil,
		},
		{
			name: "duplicates collapse, first-seen order preserved",
			body: "Closes-Inbox: b-item, a-item\nCloses-Inbox: a-item\nCloses-Inbox: b-item, c-item\n",
			want: []string{"b-item", "a-item", "c-item"},
		},
		{
			name: "prose mention mid-sentence is not a marker",
			body: "This landing Closes-Inbox: nothing-really, per the convention.\n",
			want: nil,
		},
		{
			name: "documentation of the convention is not a marker",
			body: "Emit a line reading \"Closes-Inbox: <id>\" when the landing fully closes an item.\n",
			want: nil,
		},
		{
			name: "marker with no ids yields nothing",
			body: "Closes-Inbox:\nCloses-Inbox:   \nCloses-Inbox: ,,\n",
			want: nil,
		},
		{
			name: "prose spillover with spaces is not an id",
			body: "Closes-Inbox: the salvage layer item\n",
			want: nil,
		},
		{
			name: "similarly-named markers are not this marker",
			body: "Closes-Inbox-Maybe: nope-item\nCloses: nope-item\nClosesInbox: nope-item\n",
			want: nil,
		},
		{
			name: "empty body",
			body: "",
			want: nil,
		},
		{
			name: "no marker anywhere",
			body: "# Build Report\n\nAll tests green. No inbox item closed.\n",
			want: nil,
		},
		{
			name: "valid ids survive alongside invalid entries on the same line",
			body: "Closes-Inbox: good-item, not an id, other.item\n",
			want: []string{"good-item", "other.item"},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClosesInboxIDs([]byte(tc.body))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ClosesInboxIDs(%q) = %#v, want %#v", tc.body, got, tc.want)
			}
		})
	}
}
