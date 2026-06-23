package publishmirror

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveBuildPrefix_DropsChoreBuildKeepsRest(t *testing.T) {
	in := `{
  "feat": {"description": "a feature"},
  "chore(build)": {"required_paths": ["go/evolve", "go/bin/**"], "description": "binary"},
  "docs": {"description": "docs"}
}`
	out, err := removeBuildPrefix(in)
	if err != nil {
		t.Fatalf("removeBuildPrefix: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if _, present := m["chore(build)"]; present {
		t.Error("chore(build) should have been removed")
	}
	if _, ok := m["feat"]; !ok {
		t.Error("feat entry should be preserved")
	}
	if _, ok := m["docs"]; !ok {
		t.Error("docs entry should be preserved")
	}
}

func TestRemoveBuildPrefix_AbsentKeyIsNoError(t *testing.T) {
	in := `{"feat": {"description": "x"}}`
	out, err := removeBuildPrefix(in)
	if err != nil {
		t.Fatalf("absent chore(build) should not error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(m) != 1 {
		t.Errorf("want 1 entry preserved, got %d", len(m))
	}
}

func TestRemoveBuildPrefix_InvalidJSONErrors(t *testing.T) {
	if _, err := removeBuildPrefix("{not json"); err == nil {
		t.Fatal("invalid JSON should error")
	}
}

func TestReadStagedFiles_ReadsListedSkipsMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.md"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, _, err := readStagedFiles(dir, []string{"a.md", "sub/b.md", "gone.md"})
	if err != nil {
		t.Fatalf("readStagedFiles: %v", err)
	}
	if files["a.md"] != "alpha" || files["sub/b.md"] != "beta" {
		t.Errorf("unexpected contents: %+v", files)
	}
	if _, ok := files["gone.md"]; ok {
		t.Error("missing file should be skipped, not included")
	}
}

func TestReadStagedFiles_SymlinkScansTargetNotFollowed(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "skills", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Mimics .agents/skills/x -> skills/x (a symlink to a directory).
	if err := os.Symlink("skills/x", filepath.Join(dir, "link")); err != nil {
		t.Skip("symlinks unsupported on this platform")
	}
	files, _, err := readStagedFiles(dir, []string{"link"})
	if err != nil {
		t.Fatalf("readStagedFiles must not follow a symlink-to-dir: %v", err)
	}
	if files["link"] != "skills/x" {
		t.Errorf("symlink content should be its target path, got %q", files["link"])
	}
}

func TestReadStagedFiles_SymlinkToPIIPathIsScannable(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/Users/alice/secret", filepath.Join(dir, "bad")); err != nil {
		t.Skip("symlinks unsupported on this platform")
	}
	files, _, err := readStagedFiles(dir, []string{"bad"})
	if err != nil {
		t.Fatal(err)
	}
	if v := Scan(files, nil); len(v) == 0 {
		t.Error("a symlink targeting a /Users path must be caught by the sanitizer")
	}
}

func TestReadStagedFiles_SkipsNonRegular(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, skipped, err := readStagedFiles(dir, []string{"adir"})
	if err != nil {
		t.Fatalf("readStagedFiles: %v", err)
	}
	if _, ok := files["adir"]; ok {
		t.Error("a directory must not be scanned as text")
	}
	if len(skipped) != 1 || skipped[0] != "adir" {
		t.Errorf("directory should be reported in skipped, got %v", skipped)
	}
}

func TestReadStagedFiles_SkipsBinary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bin"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	files, skipped, err := readStagedFiles(dir, []string{"bin"})
	if err != nil {
		t.Fatalf("readStagedFiles: %v", err)
	}
	if _, ok := files["bin"]; ok {
		t.Error("binary file (NUL byte) should be skipped from the text scan")
	}
	if len(skipped) != 1 || skipped[0] != "bin" {
		t.Errorf("binary file should be reported in skipped, got %v", skipped)
	}
}
