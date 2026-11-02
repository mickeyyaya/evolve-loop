package policy

import "testing"

func TestMatchPaths_SegmentAwareGlobs(t *testing.T) {
	cases := []struct {
		pat, name string
		want      bool
	}{
		{"docs/**", "docs/a/b.md", true},
		{"docs/**", "go/docs/a.md", false},
		{"**/*.md", "README.md", true},
		{"*.md", "docs/a.md", false},
		{"go/*/x.go", "go/a/b/x.go", false},
		{"docs/a.md", "docs", false},
		{"docs/**/x.md", "docs/a/b.md", false},
	}
	for _, c := range cases {
		if got := matchPaths([]string{c.pat}, []string{c.name}); got != c.want {
			t.Errorf("matchPaths(%q, %q) = %v, want %v", c.pat, c.name, got, c.want)
		}
	}
	if !matchPaths(nil, nil) || matchPaths([]string{"docs/**"}, nil) {
		t.Errorf("empty patterns must match every scope and a pattern needs a scope path")
	}
}

func TestValidatePaths_RefusesMalformedGlobAtParse(t *testing.T) {
	if err := (*OverlaysPolicy)(nil).validatePaths(); err != nil {
		t.Fatalf("nil overlays: %v", err)
	}
	if _, err := Parse([]byte(`{"overlays":{"rules":[{"paths":["a/[x"],"skills":["s"]}]}}`)); err == nil {
		t.Fatalf("Parse accepted a malformed paths glob")
	}
	if _, err := Parse([]byte(`{"overlays":{"rules":[{"paths":["a/**/*.md"],"skills":["s"]}]}}`)); err != nil {
		t.Fatalf("Parse refused a valid glob: %v", err)
	}
}
