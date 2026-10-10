package explanationdocs

import (
	"strings"
	"testing"
)

func TestIsPlainPath_RefusesEverySpellingThatAFilesystemCanAlias(t *testing.T) {
	cases := map[string]bool{
		"docs/guide.md":                 true,
		"go/internal/a/a.go":            true,
		"a/" + strings.Repeat("b", 250): true,
		"café.md":                       false,
		"config/app.yaml:stream":        false,
		`config\app.yaml`:               false,
		"CONFIG/APP~1.YAM":              false,
		"config./app.yaml":              false,
		"config/app.yaml ":              false,
		"config/./app.yaml":             false,
		"a/" + strings.Repeat("b", 251): false,
	}
	for p, want := range cases {
		if got := IsPlainPath(p); got != want {
			t.Errorf("IsPlainPath(%q) = %v, want %v", p, got, want)
		}
	}
}
