package guards

import (
	"slices"
	"testing"
)

func TestProtectedSurfaceManifest_CoversTheReviewRoutingSurface(t *testing.T) {
	for _, p := range []string{
		"go/internal/reviewclass/classify.go",
		"go/cmd/evolve/cmd_review_class.go",
		".github/workflows/go.yml",
		"agents/evolve-diff-code-reviewer.md",
		".evolve/profiles/diff-code-reviewer.json",
	} {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false: the review routing table, its verb, the CI workflows and the diff reviewers decide which review a change gets, so no lane may edit them", p)
		}
	}
	for _, p := range []string{"agents/evolve-scout.md", ".evolve/profiles/scout.json", "go/cmd/evolve/cmd_review.go"} {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true: the review routing entries must stay narrow", p)
		}
	}
}

func TestSurfaceClassesOf_NamesEachGuaranteeClassOfAPathOnce(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []GuaranteeClass
	}{
		{"go/internal/bridge/sandbox_paths.go", []GuaranteeClass{SandboxGuarantee}},
		{"/wt/go/internal/guards/integrity_surface.go", []GuaranteeClass{GuardGuarantee, ManifestGuarantee}},
		{"go/internal/guards/role.go", []GuaranteeClass{GuardGuarantee}},
		{"README.md", nil},
		{"", nil},
	} {
		if got := SurfaceClassesOf(tc.path); !slices.Equal(got, tc.want) {
			t.Errorf("SurfaceClassesOf(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
