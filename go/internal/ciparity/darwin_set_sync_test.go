package ciparity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncDarwinSet_ChecksThenWritesTheSetFile(t *testing.T) {
	t.Parallel()
	root := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "mac/a_darwin.go": "package mac\n", DarwinSetFile: "stale\n"})

	if _, err := SyncDarwinSet(root, false); !errors.Is(err, ErrDarwinSetStale) {
		t.Fatalf("check of a stale file = %v, want ErrDarwinSetStale", err)
	}
	changed, err := SyncDarwinSet(root, true)
	if err != nil || !changed {
		t.Fatalf("write = (%v, %v), want a change", changed, err)
	}
	got, _ := os.ReadFile(filepath.Join(root, DarwinSetFile))
	if string(got) != DarwinSetText([]string{"./mac"}) {
		t.Fatalf("set file = %q", got)
	}
	if changed, err := SyncDarwinSet(root, false); err != nil || changed {
		t.Errorf("check after write = (%v, %v), want in sync", changed, err)
	}
	if changed, err := SyncDarwinSet(root, true); err != nil || changed {
		t.Errorf("a second write = (%v, %v), want no change", changed, err)
	}
}

func TestSyncDarwinSet_FailsOnAnEmptyTreeAMissingFileAFailedReadAndAFailedWrite(t *testing.T) {
	t.Parallel()
	empty := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "plain/a.go": "package plain\n"})
	if _, err := SyncDarwinSet(empty, true); err == nil {
		t.Error("a tree with no darwin package must fail")
	}
	if _, err := SyncDarwinSet(filepath.Join(t.TempDir(), "missing"), false); err == nil {
		t.Error("an unreadable tree must fail")
	}
	noFile := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "mac/a_darwin.go": "package mac\n"})
	if _, err := SyncDarwinSet(noFile, false); !errors.Is(err, ErrDarwinSetStale) {
		t.Errorf("check with no set file = %v, want ErrDarwinSetStale", err)
	}
	readOnly := writeModule(t, map[string]string{"go.mod": "module m\n\ngo 1.23\n", "mac/a_darwin.go": "package mac\n", DarwinSetFile: "stale\n"})
	if err := os.Chmod(filepath.Join(readOnly, DarwinSetFile), 0o444); err != nil {
		t.Fatal(err)
	}
	if changed, err := SyncDarwinSet(readOnly, true); err == nil || changed {
		t.Errorf("write over a read-only file = (%v, %v), want a write error and no change", changed, err)
	}
	if err := os.Mkdir(filepath.Join(noFile, DarwinSetFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncDarwinSet(noFile, true); err == nil || errors.Is(err, ErrDarwinSetStale) {
		t.Errorf("write over a directory = %v, want a write error", err)
	}
}
