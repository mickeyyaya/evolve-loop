package cyclestate

import "testing"

func TestWithoutDetail_StripsEveryClosedDetailAndTruncatesAtAnUnclosedOne(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no detail", "plain reason", "plain reason"},
		{"one closed detail", "reason [[detail: why ]] tail", "reason tail"},
		{"two closed details", "a [[detail: 1 ]] b [[detail: 2 ]] c", "a b c"},
		{"an unclosed detail drops the rest", "reason [[detail: never closed", "reason"},
		{"a closed detail then an unclosed one", "a [[detail: 1 ]] b [[detail: open", "a b"},
		{"an unclosed detail at the very start", " [[detail: orphan", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := WithoutDetail(tc.in); got != tc.want {
			t.Errorf("%s: WithoutDetail(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
	if got := WithoutDetail(WithDetail("failed", "the long story")); got != "failed" {
		t.Errorf("round trip = %q", got)
	}
}
