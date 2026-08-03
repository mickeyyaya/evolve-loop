package ciparity

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIntersectEnforced(t *testing.T) {
	enforce := []byte("# comment line\n" +
		"./internal/config\n" +
		"\n" +
		"./internal/router\n" +
		"  ./internal/policy  \n" + // leading/trailing space tolerated
		"./internal/bridge/panestream\n")

	cases := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			name:    "touched enforced packages are returned (strip /... and dedupe+sort)",
			changed: []string{"./internal/router/...", "./internal/config/...", "./internal/router/..."},
			want:    []string{"./internal/config", "./internal/router"},
		},
		{
			name:    "a touched package NOT in the enforce list is dropped",
			changed: []string{"./internal/router/...", "./internal/notenforced/..."},
			want:    []string{"./internal/router"},
		},
		{
			name:    "whitespace-padded enforce line still matches",
			changed: []string{"./internal/policy/..."},
			want:    []string{"./internal/policy"},
		},
		{
			name:    "no touched package is enforced => nil (caller skips apicover)",
			changed: []string{"./internal/notenforced/...", "./cmd/evolve/..."},
			want:    nil,
		},
		{
			name:    "empty changed set => nil (best-effort locator returned nothing)",
			changed: nil,
			want:    nil,
		},
		{
			name:    "comment and blank lines in the enforce file are never matched",
			changed: []string{"./comment/...", "./..."},
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IntersectEnforced(tc.changed, enforce)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("IntersectEnforced(%v) = %v, want %v", tc.changed, got, tc.want)
			}
		})
	}
}

// TestPackageDirHasProductionGoFiles — the shared graduation predicate both
// seams consult (build-entry abort + audit offender list). The test-only row
// is the cycles-1223/1224/1228 halt class; the recursive-pattern and
// unreadable-dir rows pin the two documented non-guessing directions.
func TestPackageDirHasProductionGoFiles(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel, body string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("internal/testonly/testonly_test.go", "package testonly\n")
	mk("internal/mixed/mixed_test.go", "package mixed\n")
	mk("internal/mixed/mixed.go", "package mixed\n\nfunc Exported() int { return 1 }\n")

	for _, tt := range []struct {
		pkg  string
		want bool
	}{
		{"./internal/testonly", false}, // vacuous obligation — must not flag
		{"./internal/mixed", true},     // real surface — obligation stands
		{"./internal/absent", false},   // unreadable dir — fail-open, backstops judge
		{"./internal/...", true},       // recursive pattern — stays flagged, never guessed
	} {
		if got := PackageDirHasProductionGoFiles(dir, tt.pkg); got != tt.want {
			t.Errorf("PackageDirHasProductionGoFiles(%q) = %v, want %v", tt.pkg, got, tt.want)
		}
	}
}
