package digest

import (
	"strings"
	"testing"
)

func TestProjectDigest_AnOpeningTagWithNoTerminatorIsAHardError(t *testing.T) {
	t.Parallel()
	got, err := ProjectDigest([]byte("<!-- digest:role=scout\nbody never opens a block\n"), "scout")
	if err == nil || !strings.Contains(err.Error(), "malformed marker") {
		t.Fatalf("want a malformed-marker error, got digest=%q err=%v", got, err)
	}
	if got != nil {
		t.Errorf("an error must not carry a partial digest, got %q", got)
	}
}

func TestProjectDigest_ARoleWithNoBlockGetsANonNilEmptyDigest(t *testing.T) {
	t.Parallel()
	got, err := ProjectDigest([]byte("<!-- digest:role=builder -->\nbuild only\n<!-- /digest -->\n"), "scout")
	if err != nil {
		t.Fatalf("ProjectDigest: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("want a non-nil empty digest, never the full source, got %#v", got)
	}
}
