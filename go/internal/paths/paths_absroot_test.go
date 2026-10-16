package paths

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestAbsoluteRoot(t *testing.T) {
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"dot_becomes_cwd", ".", abs(".")},
		{"relative_is_absolutized", "sub/dir", abs("sub/dir")},
		{"already_absolute_is_idempotent", "/tmp/evolve-x", "/tmp/evolve-x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warned := false
			got := AbsoluteRoot("--project-root", tc.in, func(string) { warned = true })
			if got != tc.want {
				t.Errorf("AbsoluteRoot(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("AbsoluteRoot(%q) = %q, not absolute", tc.in, got)
			}
			if warned {
				t.Errorf("warn fired on success for %q", tc.in)
			}
		})
	}
}

func TestAbsoluteRoot_WarnsAndReturnsInputOnError(t *testing.T) {
	orig := absFn
	defer func() { absFn = orig }()
	absFn = func(string) (string, error) { return "", errors.New("getwd failed") }

	var msg string
	got := AbsoluteRoot("--project-root", "rel/path", func(m string) { msg = m })
	if got != "rel/path" {
		t.Errorf("on Abs error: got %q, want the original input %q", got, "rel/path")
	}
	if msg == "" {
		t.Error("expected a WARN message on Abs error, got none")
	}
}
