package atomicwrite

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func assertWritten(t *testing.T, path string, want []byte, mode fs.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s back: %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("%s holds %q, want %q", path, got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Errorf("%s mode = %v, want the requested %v", path, info.Mode().Perm(), mode)
	}
	if names := dirEntryNames(t, filepath.Dir(path)); !slices.Equal(names, []string{filepath.Base(path)}) {
		t.Errorf("%s holds %v after the write, want only %s: a temp file was left behind", filepath.Dir(path), names, filepath.Base(path))
	}
}

func restrictDir(t *testing.T, dir string, mode fs.FileMode) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permission bits")
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestDurable_WritesTheBytesWithTheRequestedMode(t *testing.T) {
	for _, mode := range []fs.FileMode{0o444, 0o600, 0o644} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "not-yet-created", "object")
			payload := []byte("durable payload\n")

			if err := Durable(path, payload, mode); err != nil {
				t.Fatalf("Durable(%v): %v", mode, err)
			}

			assertWritten(t, path, payload, mode)
		})
	}
}

func TestDurable_WritesAZeroLengthPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")

	if err := Durable(path, []byte{}, 0o600); err != nil {
		t.Fatalf("Durable of an empty payload: %v", err)
	}

	assertWritten(t, path, []byte{}, 0o600)
}

func TestDurable_ReplacesAnExistingTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(path, []byte("the previous content, longer than the next\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Durable(path, []byte("replacement\n"), 0o444); err != nil {
		t.Fatalf("Durable over an existing 0644 target: %v", err)
	}
	assertWritten(t, path, []byte("replacement\n"), 0o444)

	if err := Durable(path, []byte("over a read-only target\n"), 0o600); err != nil {
		t.Fatalf("Durable over an existing read-only target: %v", err)
	}
	assertWritten(t, path, []byte("over a read-only target\n"), 0o600)
}

func TestDurable_ARenameFailureLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	targetIsANonEmptyDirectory := filepath.Join(dir, "target")
	occupant := filepath.Join(targetIsANonEmptyDirectory, "occupant")
	if err := os.MkdirAll(targetIsANonEmptyDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(occupant, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Durable(targetIsANonEmptyDirectory, []byte("cannot replace a directory\n"), 0o644)

	if err == nil {
		t.Fatal("Durable over a non-empty directory returned nil, want the rename error")
	}
	if names := dirEntryNames(t, dir); !slices.Equal(names, []string{"target"}) {
		t.Errorf("after a failed rename %s holds %v, want only the untouched target: the temp file must be removed", dir, names)
	}
	if body, rerr := os.ReadFile(occupant); rerr != nil || string(body) != "kept\n" {
		t.Errorf("the directory in the way was modified: %q, %v", body, rerr)
	}
}

func TestDurable_ACreateFailureLeavesNoTempFile(t *testing.T) {
	readOnlyDir := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(readOnlyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	restrictDir(t, readOnlyDir, 0o555)

	err := Durable(filepath.Join(readOnlyDir, "target"), []byte("x"), 0o644)

	if err == nil {
		t.Fatal("Durable into a read-only directory returned nil, want the temp-create error")
	}
	if err := os.Chmod(readOnlyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if names := dirEntryNames(t, readOnlyDir); len(names) != 0 {
		t.Errorf("a failed create left %v behind", names)
	}
}

func TestDurable_ReportsADirectoryItCannotSync(t *testing.T) {
	unreadableDir := filepath.Join(t.TempDir(), "write-and-search-only")
	if err := os.Mkdir(unreadableDir, 0o755); err != nil {
		t.Fatal(err)
	}
	restrictDir(t, unreadableDir, 0o333)

	err := Durable(filepath.Join(unreadableDir, "target"), []byte("durable only once its directory entry is synced\n"), 0o644)

	if err == nil {
		t.Fatal("Durable returned nil although the directory it renamed into cannot be opened to fsync: the directory sync is missing or its error is swallowed")
	}
	if err := os.Chmod(unreadableDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range dirEntryNames(t, unreadableDir) {
		if name != "target" {
			t.Errorf("a failed directory sync left %s behind", name)
		}
	}
}
