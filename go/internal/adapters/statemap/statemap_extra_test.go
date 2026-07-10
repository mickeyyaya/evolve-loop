package statemap

// statemap_extra_test.go — apicover DoD coverage for the WriteStateMap
// primitive (named + happy-path + the error branches) and ReadStateMap's
// null-JSON edge. The behavioral contract for ReadStateMap/UpdateStateMap lives
// in statemap_test.go; this file only lifts coverage on paths that file's
// higher-level assertions do not reach.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteStateMap_TempFileFailures covers the write/sync/close error branches
// by seaming the temp-file syscalls (the same white-box technique
// adapters/storage.writeJSONAtomic uses for these otherwise-untriggerable
// paths). Each case asserts the correct error surfaces AND that no file is left
// at the target path.
func TestWriteStateMap_TempFileFailures(t *testing.T) {
	boom := errors.New("boom")
	saved := writeHooks
	t.Cleanup(func() { writeHooks = saved })

	cases := []struct {
		name  string
		apply func()
		want  string
	}{
		{"write", func() { writeHooks.write = func(*os.File, []byte) (int, error) { return 0, boom } }, "write tmp"},
		{"sync", func() { writeHooks.sync = func(*os.File) error { return boom } }, "sync tmp"},
		{"close", func() { writeHooks.close = func(*os.File) error { return boom } }, "close tmp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeHooks = saved // reset before each case
			tc.apply()
			path := filepath.Join(t.TempDir(), "state.json")
			err := WriteStateMap(path, map[string]any{"k": "v"})
			if err == nil {
				t.Fatalf("expected %s error, got nil", tc.name)
			}
			if !errors.Is(err, boom) {
				t.Errorf("error must wrap the seamed failure: %v", err)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("failed write must not leave a file at the target path")
			}
		})
	}
}

// TestWriteStateMap_RoundTrip names WriteStateMap directly and asserts the
// atomic write is readable back with every field intact.
func TestWriteStateMap_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json") // exercises MkdirAll
	want := map[string]any{"expected_ship_sha": "abc", "n": float64(2)}
	if err := WriteStateMap(path, want); err != nil {
		t.Fatalf("WriteStateMap: %v", err)
	}
	got, err := ReadStateMap(path)
	if err != nil {
		t.Fatalf("ReadStateMap: %v", err)
	}
	if got["expected_ship_sha"] != "abc" || got["n"] != float64(2) {
		t.Fatalf("round trip = %+v", got)
	}
}

// TestWriteStateMap_MkdirError covers the MkdirAll failure branch: the parent
// path component is a regular file, so a directory cannot be created under it.
func TestWriteStateMap_MkdirError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "iamafile")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	if err := WriteStateMap(filepath.Join(blocker, "state.json"), map[string]any{"k": "v"}); err == nil {
		t.Fatal("WriteStateMap under a regular-file parent must error")
	}
}

// TestWriteStateMap_MarshalError covers the json.MarshalIndent failure branch:
// a value the encoder cannot represent (a func).
func TestWriteStateMap_MarshalError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := WriteStateMap(path, map[string]any{"bad": func() {}}); err == nil {
		t.Fatal("unmarshalable value must produce a marshal error")
	}
}

// TestWriteStateMap_CreateTempError covers the CreateTemp failure branch: the
// target directory exists but is read-only, so a temp file cannot be created.
func TestWriteStateMap_CreateTempError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatalf("mkdir ro: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := WriteStateMap(filepath.Join(dir, "state.json"), map[string]any{"k": "v"}); err == nil {
		t.Skip("read-only dir still writable (running as root?) — CreateTemp branch not exercised")
	}
}

// TestWriteStateMap_RenameError covers the os.Rename failure branch: the target
// path is an existing (non-empty) directory, so renaming the temp file over it
// fails.
func TestWriteStateMap_RenameError(t *testing.T) {
	target := filepath.Join(t.TempDir(), "state.json")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed child: %v", err)
	}
	if err := WriteStateMap(target, map[string]any{"k": "v"}); err == nil {
		t.Fatal("rename over a non-empty directory must error")
	}
}

// TestReadStateMap_NullJSON covers the `m == nil` normalization branch: a
// literal JSON `null` unmarshals to a nil map, which must be normalized to an
// empty non-nil map.
func TestReadStateMap_NullJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("null"), 0o644); err != nil {
		t.Fatalf("seed null: %v", err)
	}
	m, err := ReadStateMap(path)
	if err != nil {
		t.Fatalf("ReadStateMap(null): %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("null JSON must normalize to empty non-nil map, got %#v", m)
	}
}
