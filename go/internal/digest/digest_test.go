package digest

import (
	"strings"
	"testing"
)

// TestProjectDigest is the projector's own package-level regression suite
// (scout-report.md cycle 1391, Task 1, verifiableBy). The full behavioral
// pins for each acceptance criterion live in
// go/acs/cycle1391/predicates_test.go; this file covers the same contract
// at the package level plus one additional case (multi-role membership).
func TestProjectDigest(t *testing.T) {
	t.Run("extracts only the tagged role's content", func(t *testing.T) {
		source := []byte("Untagged intro.\n" +
			"<!-- digest:role=scout -->\nScout body.\n<!-- /digest -->\n" +
			"<!-- digest:role=build -->\nBuild body.\n<!-- /digest -->\n" +
			"Untagged tail.\n")

		got, err := ProjectDigest(source, "scout")
		if err != nil {
			t.Fatalf("ProjectDigest returned error: %v", err)
		}
		out := string(got)
		if !strings.Contains(out, "Scout body.") {
			t.Errorf("missing own role's content; got %q", out)
		}
		if strings.Contains(out, "Build body.") || strings.Contains(out, "Untagged") {
			t.Errorf("leaked non-matching content; got %q", out)
		}
	})

	t.Run("multi-role marker matches every listed role", func(t *testing.T) {
		source := []byte("<!-- digest:role=scout,build -->\nShared body.\n<!-- /digest -->\n")

		for _, role := range []string{"scout", "build"} {
			got, err := ProjectDigest(source, role)
			if err != nil {
				t.Fatalf("ProjectDigest(%q) returned error: %v", role, err)
			}
			if !strings.Contains(string(got), "Shared body.") {
				t.Errorf("role %q should match a multi-role marker listing it; got %q", role, got)
			}
		}

		got, err := ProjectDigest(source, "ship")
		if err != nil {
			t.Fatalf("ProjectDigest(ship) returned error: %v", err)
		}
		if len(strings.TrimSpace(string(got))) != 0 {
			t.Errorf("role=ship is not listed on the marker, want empty digest, got %q", got)
		}
	})

	t.Run("byte reduction on an excluded-heavy fixture", func(t *testing.T) {
		excluded := strings.Repeat("Cross-cutting content scout never acts on.\n", 40)
		source := []byte("<!-- digest:role=build -->\n" + excluded + "<!-- /digest -->\n" +
			"<!-- digest:role=scout -->\nScout needs only this line.\n<!-- /digest -->\n")

		got, err := ProjectDigest(source, "scout")
		if err != nil {
			t.Fatalf("ProjectDigest returned error: %v", err)
		}
		if len(got) >= len(source)/2 {
			t.Errorf("len(digest)=%d not < 50%% of len(source)=%d", len(got), len(source))
		}
	})

	t.Run("no matching block yields empty, not full source", func(t *testing.T) {
		source := []byte("<!-- digest:role=scout -->\nScout only.\n<!-- /digest -->\n")

		got, err := ProjectDigest(source, "ship")
		if err != nil {
			t.Fatalf("ProjectDigest returned error: %v", err)
		}
		if len(strings.TrimSpace(string(got))) != 0 {
			t.Errorf("want empty digest for unmatched role, got %q", got)
		}
	})

	t.Run("unterminated marker is a hard error", func(t *testing.T) {
		source := []byte("<!-- digest:role=scout -->\nnever closes\n")

		if _, err := ProjectDigest(source, "scout"); err == nil {
			t.Errorf("want non-nil error for unterminated marker, got nil")
		}
	})

	t.Run("source with no markers yields empty digest", func(t *testing.T) {
		got, err := ProjectDigest([]byte("plain text, no markers at all\n"), "scout")
		if err != nil {
			t.Fatalf("ProjectDigest returned error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("want empty digest for a source with no markers, got %q", got)
		}
	})
}
