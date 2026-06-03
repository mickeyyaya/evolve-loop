package changedpkgs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileToPackage(t *testing.T) {
	cases := []struct {
		file   string
		want   string
		wantOK bool
	}{
		{"go/internal/foo/bar.go", "./internal/foo/...", true},
		{"go/internal/foo/bar_test.go", "./internal/foo/...", true},
		{"go/cmd/evolve/cmd_x.go", "./cmd/evolve/...", true},
		{"./go/internal/a/b.go", "./internal/a/...", true},
		{"go/main.go", "./...", true},      // module-root file
		{"internal/foo/bar.go", "", false}, // not under go/ → rejected (under-scope, never misroute)
		{"acs/lib/helper.go", "", false},   // a .go file outside the module → rejected
		{"docs/readme.md", "", false},
		{"acs/cycle-1/001-x.sh", "", false},
		{"CLAUDE.md", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := FileToPackage(c.file)
		if got != c.want || ok != c.wantOK {
			t.Errorf("FileToPackage(%q) = (%q,%v), want (%q,%v)", c.file, got, ok, c.want, c.wantOK)
		}
	}
}

func TestChangedPackages(t *testing.T) {
	t.Run("dedupes + sorts across thrusts", func(t *testing.T) {
		dir := t.TempDir()
		handoff := filepath.Join(dir, "handoff-build.json")
		body := `{"thrusts":[
		  {"files_modified":["go/internal/foo/a.go","go/internal/foo/b.go"],"files_new":["go/internal/bar/c.go"]},
		  {"files_modified":["go/internal/foo/d.go"],"files_new":["docs/x.md"]}
		]}`
		if err := os.WriteFile(handoff, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		got := ChangedPackages(handoff)
		want := []string{"./internal/bar/...", "./internal/foo/..."}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ChangedPackages() = %v, want %v", got, want)
		}
	})

	t.Run("missing file → nil", func(t *testing.T) {
		if got := ChangedPackages(filepath.Join(t.TempDir(), "absent.json")); got != nil {
			t.Errorf("missing handoff should yield nil, got %v", got)
		}
	})

	t.Run("malformed json → nil", func(t *testing.T) {
		dir := t.TempDir()
		handoff := filepath.Join(dir, "h.json")
		_ = os.WriteFile(handoff, []byte("{not json"), 0o644)
		if got := ChangedPackages(handoff); got != nil {
			t.Errorf("malformed handoff should yield nil, got %v", got)
		}
	})

	t.Run("no go files → nil", func(t *testing.T) {
		dir := t.TempDir()
		handoff := filepath.Join(dir, "h.json")
		_ = os.WriteFile(handoff, []byte(`{"thrusts":[{"files_modified":["docs/a.md","x.sh"]}]}`), 0o644)
		if got := ChangedPackages(handoff); got != nil {
			t.Errorf("non-go changes should yield nil, got %v", got)
		}
	})
}
