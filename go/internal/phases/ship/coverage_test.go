package ship

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitNonEmpty(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a\n", 1},
		{"a\nb\nc\n", 3},
		{"\n\na\n\n", 1},
		{"single", 1},
	}
	for _, c := range cases {
		got := splitNonEmpty(c.in)
		if len(got) != c.want {
			t.Errorf("splitNonEmpty(%q): len=%d want %d", c.in, len(got), c.want)
		}
	}
}

func TestExtractIDs(t *testing.T) {
	if got := extractIDs([]byte("not json{")); got != nil {
		t.Errorf("invalid JSON should return nil, got %v", got)
	}
	if got := extractIDs([]byte("{}")); len(got) != 0 {
		t.Errorf("empty doc: got %v", got)
	}
	body := `{
		"top_n": [{"id": "task-a"}, {"id": "task-b"}, {"id": ""}],
		"skip_shipped": [{"task_id": "task-b"}, {"task_id": "task-c"}]
	}`
	got := extractIDs([]byte(body))
	if len(got) != 3 {
		t.Errorf("expected 3 unique IDs, got %v", got)
	}
	wantSet := map[string]bool{"task-a": true, "task-b": true, "task-c": true}
	for _, id := range got {
		if !wantSet[id] {
			t.Errorf("unexpected id: %q", id)
		}
		delete(wantSet, id)
	}
}

func TestIsTerminal_NonFile(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Errorf("bytes.Buffer should not be terminal")
	}
}

func TestIsTerminal_TempFile(t *testing.T) {
	f, err := os.CreateTemp("", "isterminal-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if isTerminal(f) {
		t.Errorf("temp file should not be terminal")
	}
}

func TestCleanExitError(t *testing.T) {
	e := (&cleanExitError{}).Error()
	if e == "" {
		t.Errorf("Error() empty")
	}
}

func TestStateInt(t *testing.T) {
	m := map[string]any{
		"float":   float64(42),
		"int":     int(7),
		"string":  "hello",
		"nil-key": nil,
	}
	if got, _ := stateInt(m, "float"); got != 42 {
		t.Errorf("float: got %d", got)
	}
	if got, _ := stateInt(m, "int"); got != 7 {
		t.Errorf("int: got %d", got)
	}
	if got, _ := stateInt(m, "string"); got != 0 {
		t.Errorf("string: got %d", got)
	}
	if got, _ := stateInt(m, "nil-key"); got != 0 {
		t.Errorf("nil: got %d", got)
	}
	if got, _ := stateInt(m, "missing"); got != 0 {
		t.Errorf("missing: got %d", got)
	}
}

func TestReadStateMap_Missing(t *testing.T) {
	m, err := readStateMap("/nonexistent/state.json")
	if err != nil {
		t.Errorf("expected nil error for missing file (soft path)")
	}
	if m == nil {
		t.Errorf("expected empty map for missing file")
	}
}

func TestReadStateMap_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("not json{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readStateMap(path); err == nil {
		t.Errorf("expected JSON error")
	}
}

func TestReadStateMap_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{"k":"v","n":42}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, err := readStateMap(path)
	if err != nil {
		t.Fatalf("readStateMap: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("k=%v", m["k"])
	}
}

func TestWriteStateMap_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := writeStateMap(path, map[string]any{"a": "b", "c": float64(1)}); err != nil {
		t.Fatalf("write: %v", err)
	}
	m, err := readStateMap(path)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if m["a"] != "b" {
		t.Errorf("a=%v", m["a"])
	}
}

func TestSha256File_Missing(t *testing.T) {
	if _, err := sha256File("/nonexistent/file"); err == nil {
		t.Errorf("expected error")
	}
}

func TestSha256File_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	h, err := sha256File(path)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if h != want {
		t.Errorf("sha=%s want %s", h, want)
	}
}

func TestPluginVersion_Missing(t *testing.T) {
	if got := pluginVersion("/nonexistent/plugin.json"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestPluginVersion_Valid(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"),
		[]byte(`{"name":"evo","version":"11.5.5"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := pluginVersion(dir); got != "11.5.5" {
		t.Errorf("version=%q", got)
	}
}

func TestIntegrityError_Error(t *testing.T) {
	e := &IntegrityError{Msg: "test failure"}
	if e.Error() == "" {
		t.Errorf("Error() empty")
	}
}
