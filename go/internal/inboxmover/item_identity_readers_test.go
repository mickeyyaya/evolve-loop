package inboxmover

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func writeIdentityItem(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestItemIDAt_IsTheLoadersID(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct{ file, body, want string }{
		"recorded":  {"a.json", `{"id":"named"}`, "named"},
		"id-less":   {"no-id.json", `{"title":"x"}`, "no-id"},
		"sanitized": {"bell.json", `{"id":"bell\u0007item"}`, "bell item"},
		"malformed": {"bad.json", `{`, ""},
	}
	for name, tc := range cases {
		path := filepath.Join(dir, tc.file)
		writeIdentityItem(t, path, tc.body)
		if got := itemIDAt(path); got != tc.want {
			t.Errorf("%s: itemIDAt = %q, want %q", name, got, tc.want)
		}
	}
	if got := itemIDAt(filepath.Join(dir, "missing.json")); got != "" {
		t.Errorf("an unreadable file has no id, got %q", got)
	}
}

func TestResolveContinuationForScope_AdoptsAnIDLessClaimByTheLoadersID(t *testing.T) {
	inbox := t.TempDir()
	writeIdentityItem(t, filepath.Join(inboxbatch.ProcessingCycleDir(inbox, 5), "no-id.json"),
		`{"continuation":{"snapshot_sha":"abc","cycle":4}}`)

	got := ResolveContinuationForScope(Options{InboxDir: inbox, Stderr: os.Stderr}, 5, []string{"no-id"})

	if got == nil || got.SnapshotSHA != "abc" {
		t.Fatalf("a claim in this lane's scope by the loader's id must be adopted, got %+v", got)
	}
}
