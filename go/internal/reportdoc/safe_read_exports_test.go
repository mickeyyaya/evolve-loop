//go:build darwin || linux

package reportdoc

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidReference_SingleSourcedStrictness names the exported validator that
// is now the single home of the "is this relative path safe?" belief, shared
// with explanationdocs (architecture review 2026-09-01: the two packages
// previously judged the same string differently).
func TestValidReference_SingleSourcedStrictness(t *testing.T) {
	cases := []struct {
		reference string
		want      bool
	}{
		{"a/../../b", false}, // cleans to ../b — escapes the root
		{"x:y", false},       // colon
		{"b\x00d", false},    // NUL byte
		{"", false},
		{".", false},
		{"..", false},
		{"../up", false},
		{"/abs", false},
		{"a\\b", false}, // backslash
		{"ok/path.md", true},
		{"docs/adr/0001.md", true},
	}
	for _, tc := range cases {
		if got := ValidReference(tc.reference); got != tc.want {
			t.Errorf("ValidReference(%q) = %v, want %v", tc.reference, got, tc.want)
		}
	}
}

// TestOpenRegularNoFollow_RefusesFinalSymlink pins the exported O_NOFOLLOW
// open helper: a final-component symlink must fail to open, a regular file
// must succeed. One home for this helper — explanationdocs deletes its copy
// and calls this one.
func TestOpenRegularNoFollow_RefusesFinalSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenRegularNoFollow(link); err == nil {
		t.Fatal("opening a symlink via OpenRegularNoFollow must fail")
	}
	file, err := OpenRegularNoFollow(target)
	if err != nil {
		t.Fatalf("opening a regular file must succeed: %v", err)
	}
	_ = file.Close()
}
