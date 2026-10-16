//go:build darwin || linux

package reportdoc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidReference_SingleSourcedStrictness(t *testing.T) {
	cases := []struct {
		reference string
		want      bool
	}{
		{"a/../../b", false},
		{"x:y", false},
		{"b\x00d", false},
		{"", false},
		{".", false},
		{"..", false},
		{"../up", false},
		{"/abs", false},
		{"a\\b", false},
		{"ok/path.md", true},
		{"docs/adr/0001.md", true},
	}
	for _, tc := range cases {
		if got := ValidReference(tc.reference); got != tc.want {
			t.Errorf("ValidReference(%q) = %v, want %v", tc.reference, got, tc.want)
		}
	}
}

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
