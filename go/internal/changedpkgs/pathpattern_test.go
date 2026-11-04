package changedpkgs

import "testing"

func TestPatternOf_NamesTheGoTestPatternOfEachPathUnderGo(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"go/internal/a/x.go":                 "./internal/a/...",
		"./go/internal/a/x.go":               "./internal/a/...",
		"go/internal/b/embed.tmpl":           "./internal/b/...",
		"go/x.go":                            "./...",
		"go/go.mod":                          "./...",
		"go/go.sum":                          "./...",
		"go/vendor/modules.txt":              "./...",
		"go/vendor/example.org/dep/d.go":     "./...",
		"go/internal/c/testdata/golden.json": "./internal/c/...",
		"go/internal/c/testdata/fixture.go":  "./internal/c/...",
		"go/internal/c/testdata/x/y/f.go":    "./internal/c/...",
		"go/testdata/x.json":                 "./...",
		"go\\internal\\a\\x.go":              "./internal/a/...",
	}
	for file, want := range cases {
		got, ok := PatternOf(file)
		if !ok || got != want {
			t.Errorf("PatternOf(%q) = %q, %v; want %q", file, got, ok, want)
		}
	}
}

func TestPatternOf_RefusesAPathOutsideGoAndAnUnsafeOne(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"docs/readme.md", "", "go", "go/internal/a b/x.go", "gox/a.go"} {
		if got, ok := PatternOf(file); ok {
			t.Errorf("PatternOf(%q) = %q, want no pattern", file, got)
		}
	}
}
