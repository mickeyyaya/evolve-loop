package acssuite

// verdict_provenance_test.go — cycle-1434 (ADR-0072 halt): a verdict minted
// under the WRONG state root red'd 3 predicates the correct-root run showed
// green, and the artifact recorded nothing about which roots it was minted
// under — the misdiagnosis was invisible from the file. Every verdict now
// stamps suite_root/project_root; readers treat ABSENCE as "unstamped"
// (pre-stamp verdicts stay honored), never as a mismatch.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_StampsSuiteAndProjectRoot(t *testing.T) {
	root := t.TempDir()
	raw := goStream(goLine(acsPkgBase+"cycle9", "TestC9_001_Ok", "pass"))
	v, err := Run(Options{Root: root, ProjectRoot: "/plane", Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	if v.SuiteRoot != root || v.ProjectRoot != "/plane" {
		t.Errorf("stamps suite_root=%q project_root=%q, want %q / %q", v.SuiteRoot, v.ProjectRoot, root, "/plane")
	}

	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	path, err := WriteVerdict(evolveDir, v)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["suite_root"] != root || got["project_root"] != "/plane" {
		t.Errorf("written verdict roots = %v / %v, want %q / %q", got["suite_root"], got["project_root"], root, "/plane")
	}

	// Unstamped shape (empty ProjectRoot — the caller's-env inherit mode):
	// the keys must be ABSENT, not empty strings, so pre-stamp readers and
	// mismatch checks both see "unstamped".
	v2, err := Run(Options{Root: root, Cycle: 9, GoExec: seamGo(raw, nil)})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v2)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["project_root"]; ok {
		t.Error("empty project_root serialized — omitempty lost; absence must mean unstamped")
	}
}
