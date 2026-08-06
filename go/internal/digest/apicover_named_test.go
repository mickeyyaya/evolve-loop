package digest

import (
	"strings"
	"testing"
)

// TestAPICoverNamedExports names and EXERCISES the one exported symbol of
// this package (ADR-0069 new-package graduation): ProjectDigest, through
// the same happy-path/negative-path/error-path shape its real consumer
// (go/internal/systemprompt.Resolve, via a pre-generated digest file) relies
// on — a role's tagged content survives, another role's content is
// excluded, an unmatched role yields empty (not full source), and a
// malformed marker is a hard error.
func TestAPICoverNamedExports(t *testing.T) {
	source := []byte("<!-- digest:role=scout -->\nScout content.\n<!-- /digest -->\n" +
		"<!-- digest:role=build -->\nBuild content.\n<!-- /digest -->\n")

	got, err := ProjectDigest(source, "scout")
	if err != nil {
		t.Fatalf("ProjectDigest returned error: %v", err)
	}
	if !strings.Contains(string(got), "Scout content.") {
		t.Errorf("ProjectDigest(source, %q) = %q, want it to contain the role's own content", "scout", got)
	}
	if strings.Contains(string(got), "Build content.") {
		t.Errorf("ProjectDigest(source, %q) = %q, must not leak another role's content", "scout", got)
	}

	empty, err := ProjectDigest(source, "ship")
	if err != nil {
		t.Fatalf("ProjectDigest returned error: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("ProjectDigest(source, %q) = %q, want empty for an unmatched role", "ship", empty)
	}

	if _, err := ProjectDigest([]byte("<!-- digest:role=scout -->\nno closing marker\n"), "scout"); err == nil {
		t.Errorf("ProjectDigest with an unterminated marker: error = nil, want non-nil")
	}
}
