// statefile_extra_test.go — error-branch and type-coercion coverage for
// the map-based state helpers that the round-trip happy-path tests miss.
package ship

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestWriteStateMap_MkdirError(t *testing.T) {
	root := t.TempDir()
	// A regular file where writeStateMap will try to MkdirAll a directory.
	blocker := filepath.Join(root, "blocker")
	mustWrite(t, blocker, "i am a file\n")
	err := writeStateMap(filepath.Join(blocker, "state.json"), map[string]any{"k": "v"})
	if err == nil {
		t.Fatal("want mkdir error when the parent path is a file")
	}
}

func TestStateInt_IntAndJSONNumberCoercion(t *testing.T) {
	if v, ok := stateInt(map[string]any{"n": 7}, "n"); !ok || v != 7 {
		t.Errorf("int value: got (%d,%v), want (7,true)", v, ok)
	}
	if v, ok := stateInt(map[string]any{"n": json.Number("42")}, "n"); !ok || v != 42 {
		t.Errorf("json.Number: got (%d,%v), want (42,true)", v, ok)
	}
	if _, ok := stateInt(map[string]any{"n": json.Number("NaN")}, "n"); ok {
		t.Error("unparseable json.Number must report ok=false")
	}
	if _, ok := stateInt(map[string]any{"n": "string"}, "n"); ok {
		t.Error("non-numeric value must report ok=false")
	}
}

func TestReadStateMap_NullJSON_YieldsEmptyMap(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	mustWrite(t, p, "null")
	m, err := readStateMap(p)
	if err != nil || m == nil || len(m) != 0 {
		t.Errorf("JSON null must yield empty non-nil map; got (%v, %v)", m, err)
	}
}

func TestReadStateMap_PathIsDirectory_ReadError(t *testing.T) {
	if _, err := readStateMap(t.TempDir()); err == nil {
		t.Fatal("reading a directory path must return a (non-ErrNotExist) error")
	}
}
