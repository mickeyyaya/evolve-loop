package repocontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModulePath_ReadsTheDeclaredModuleAndYieldsNothingWhenThereIsNone(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name string
		path string
		want string
	}{
		{"plain", write("a.mod", "module example.com/m\n\ngo 1.23\n"), "example.com/m"},
		{"quoted", write("b.mod", "module \"example.com/q\"\n"), "example.com/q"},
		{"after a comment line", write("c.mod", "// header\nmodule example.com/c // trailing\n"), "example.com/c"},
		{"no module directive", write("d.mod", "go 1.23\nrequire x v1\n"), ""},
		{"a bare module keyword", write("e.mod", "module\n"), ""},
		{"empty file", write("f.mod", ""), ""},
		{"missing file", filepath.Join(dir, "absent.mod"), ""},
	}
	for _, tc := range cases {
		if got := modulePath(tc.path); got != tc.want {
			t.Errorf("%s: modulePath = %q, want %q", tc.name, got, tc.want)
		}
	}
}
