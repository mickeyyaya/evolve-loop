package cycleoutcome

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const laneScopeWireTag = `json:"todo_ids"`

func writeLaneScopePin(t *testing.T, dir, body string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "lane-scope.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write lane-scope.json: %v", err)
	}
	return dir
}

func TestLaneScopeProjection_OrderedAndFailOpen(t *testing.T) {
	pinned := writeLaneScopePin(t, t.TempDir(), `{"todo_ids":["alpha","beta","gamma"],"goal_hash":"h"}`)
	if got := LaneScopeIDs(pinned); !slices.Equal(got, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("pinned members must project in order; got %v", got)
	}
	malformed := writeLaneScopePin(t, t.TempDir(), "{not json")
	if got := LaneScopeIDs(malformed); len(got) != 0 {
		t.Errorf("a malformed pin must fail open to the empty projection, never a partial set; got %v", got)
	}
	if got := LaneScopeIDs(t.TempDir()); len(got) != 0 {
		t.Errorf("an absent pin must fail open to the empty projection; got %v", got)
	}
	if got := LaneScopeIDs(""); len(got) != 0 {
		t.Errorf("an empty workspace path must fail open to the empty projection; got %v", got)
	}
}

func TestLaneScopeProjection_SingleWireShapeDeclaration(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	var decls []string
	err = filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if name := d.Name(); name == "testdata" || name == "acs" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(body), laneScopeWireTag) {
			rel, relErr := filepath.Rel(moduleRoot, path)
			if relErr != nil {
				rel = path
			}
			decls = append(decls, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", moduleRoot, err)
	}
	slices.Sort(decls)
	if len(decls) != 1 {
		t.Errorf("the lane-scope slug set must have exactly ONE parser; %d non-test files declare %s: %v",
			len(decls), laneScopeWireTag, decls)
	}
}
