package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLaneScope_ReturnsEachStateOfTheFile(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		write   func(t *testing.T, path string)
		wantIDs []string
		wantErr string
	}{
		{"absent", func(*testing.T, string) {}, nil, ""},
		{"valid", func(t *testing.T, p string) { mustWrite(t, p, `{"todo_ids":["a","b"]}`) }, []string{"a", "b"}, ""},
		{"corrupt", func(t *testing.T, p string) { mustWrite(t, p, `{bad`) }, nil, "lane-scope.json malformed"},
		{"unreadable", func(t *testing.T, p string) {
			if err := os.Mkdir(p, 0o755); err != nil {
				t.Fatal(err)
			}
		}, nil, "lane-scope.json unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			tc.write(t, filepath.Join(ws, LaneScopeFile))

			scope, err := ReadLaneScope(ws)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v, want none", err)
			}
			if tc.wantIDs == nil {
				if scope != nil {
					t.Fatalf("scope = %+v, want nil for an absent file", scope)
				}
				return
			}
			if scope == nil || strings.Join(scope.TodoIDs, ",") != strings.Join(tc.wantIDs, ",") {
				t.Fatalf("scope = %+v, want ids %v", scope, tc.wantIDs)
			}
		})
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadLaneScope_AnEmptyWorkspaceReadsNothingFromTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, LaneScopeFile), `{"todo_ids":["stray"]}`)
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	if scope := loadLaneScope(""); scope != nil {
		t.Errorf("scope = %+v, want nil: an empty workspace names no file", scope)
	}
}
