package inboxmover

// closesmarker_test.go — RED contract for `ClosesInboxIDs`, the builder-authored
// closure marker parser (cycle 1452, inbox item consumption-rides-landing-ship
// weight 0.92).
//
// Why a marker and not diff inference: `connects_to` in an inbox item is a HINT,
// not an acceptance predicate, so inferring closure from touched paths would
// consume items an unrelated diff happened to brush past. Silent over-consumption
// is strictly worse than the under-consumption we have today (data loss vs. a
// wasted bookkeeping cycle), so closure must be an explicit, line-anchored,
// auditor-checkable assertion by the Builder.
//
// The contract this file freezes (doNotModifyTests):
//
//  1. A line whose first non-blank content — after an optional markdown bullet
//     (`-` / `*` / `+`) — is `Closes-Inbox:` (marker matched case-insensitively)
//     contributes its comma-separated ids.
//  2. Ids are trimmed of whitespace and surrounding backticks, and must match
//     `[A-Za-z0-9._-]+`; anything else on the line is dropped, not guessed at.
//  3. Result is deduped, first-seen order preserved; nil when nothing matched.
//  4. NOT line-anchored ⇒ NOT a marker. Prose that merely mentions the marker
//     mid-sentence contributes nothing. This is the anti-false-positive half and
//     the reason a substring/regex-anywhere implementation cannot pass.

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
			name: "marker is case insensitive, indented, and CRLF tolerant",
			body: "  closes-inbox: indented-item\r\nCLOSES-INBOX: shouty-item\r\n",
			want: []string{"indented-item", "shouty-item"},
		},
		{
			name: "duplicates collapse, first-seen order preserved",
			body: "Closes-Inbox: b-item, a-item\nCloses-Inbox: a-item\nCloses-Inbox: b-item, c-item\n",
			want: []string{"b-item", "a-item", "c-item"},
		},
		// --- negative / adversarial half: none of these may consume anything ---
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
