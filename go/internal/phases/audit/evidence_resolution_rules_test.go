package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestOneEvidenceResolves_RejectsEachUnresolvableCitationWithItsReason(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "pkgdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		citation string
		req      core.PhaseRequest
		reason   string
	}{
		{"blank citation names no evidence", "   ", core.PhaseRequest{ProjectRoot: root}, "no evidence"},
		{"citation without a project root cannot resolve", "pkgdir", core.PhaseRequest{}, "no project root"},
		{"a directory is not a regular file", "pkgdir", core.PhaseRequest{ProjectRoot: root}, "is not a regular file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, why := oneEvidenceResolves(tc.citation, tc.req)
			if ok {
				t.Fatalf("oneEvidenceResolves(%q) = true, want false", tc.citation)
			}
			if !strings.Contains(why, tc.reason) {
				t.Errorf("oneEvidenceResolves(%q) reason = %q, want it to contain %q", tc.citation, why, tc.reason)
			}
		})
	}
}
