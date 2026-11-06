package inboxmover_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func putConsumed(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFindConsumedRecord_TwoRecordsOneIDNamesBothPaths(t *testing.T) {
	inbox := t.TempDir()
	c := filepath.Join(inbox, "consumed")
	a := putConsumed(t, c, "a.json", `{"id":"dup"}`)
	b := putConsumed(t, filepath.Join(c, "cycle-1"), "b.json", `{"id":"dup"}`)
	got, err := inboxmover.FindConsumedRecord(inbox, "dup")
	if got != "" || err == nil || !strings.Contains(err.Error(), a) || !strings.Contains(err.Error(), b) {
		t.Fatalf("got (%q, %v), want empty path and error naming %s and %s", got, err, a, b)
	}
}

func TestFindConsumedRecord_OneRecordAndNoneAndNoise(t *testing.T) {
	inbox := t.TempDir()
	c := filepath.Join(inbox, "consumed")
	only := putConsumed(t, c, "only.json", `{"id":"only"}`)
	putConsumed(t, c, "note.txt", `{"id":"only"}`)
	putConsumed(t, c, "bad.json", `not json`)
	if got, err := inboxmover.FindConsumedRecord(inbox, "only"); err != nil || got != only {
		t.Fatalf("got (%q, %v), want (%q, nil)", got, err, only)
	}
	if got, err := inboxmover.FindConsumedRecord(inbox, "absent"); got != "" || err != inboxmover.ErrNotFound {
		t.Fatalf("got (%q, %v), want ErrNotFound", got, err)
	}
}

func TestFindConsumedRecord_MissingConsumedDirIsNotFound(t *testing.T) {
	if _, err := inboxmover.FindConsumedRecord(t.TempDir(), "x"); err != inboxmover.ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}
