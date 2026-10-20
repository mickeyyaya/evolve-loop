package ledgertip

import "testing"

func TestFormat_IsSeqColonHashWithNoNewline(t *testing.T) {
	t.Parallel()
	if got := Format(41, "abc"); got != "41:abc" {
		t.Fatalf("Format = %q, want %q", got, "41:abc")
	}
}
