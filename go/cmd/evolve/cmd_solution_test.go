package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSolutionCheck(t *testing.T) {
	t.Parallel()
	if lookupCommand("solution") == nil {
		t.Fatal("no `solution` command registered")
	}
	root := t.TempDir()
	reg := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(reg, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reg, "phase-registry.json"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	sol := filepath.Join(root, "solutions", "netflix-margin")
	for rel, body := range map[string]string{
		"options/1-a.md":              "# A\n\n+0.6pp see assumptions-and-evidence.md\n",
		"options/2-b.md":              "# B\n\n+0.5pp see assumptions-and-evidence.md\n",
		"recommendation.md":           "# R\n\n## Options Compared\n\nx\n\n## Recommendation\n\nA, see assumptions-and-evidence.md\n",
		"assumptions-and-evidence.md": "# E\n\n## Assumptions\n\n- A1\n\n## Evidence\n\n- E1\n",
	} {
		p := filepath.Join(sol, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) (int, string) {
		var out, errb bytes.Buffer
		code := dispatch(append([]string{"solution"}, args...), nil, &out, &errb)
		return code, out.String() + errb.String()
	}
	if code, msg := run("check", sol, "--project-root", root); code != 0 {
		t.Fatalf("well-formed solution: exit %d, want 0: %s", code, msg)
	}
	if err := os.Remove(filepath.Join(sol, "options", "2-b.md")); err != nil {
		t.Fatal(err)
	}
	if code, msg := run("check", sol, "--project-root", root); code != 1 || !strings.Contains(msg, "options") {
		t.Fatalf("violation: exit %d, want 1 naming options: %s", code, msg)
	}
	if code, _ := run(); code != 2 {
		t.Fatalf("usage: exit %d, want 2", code)
	}
}

func TestSolutionCheck_TheDefaultRootIsTheWorkingDirectoryAndAnUnknownOneRefuses(t *testing.T) {
	saved := solutionWorkdir
	t.Cleanup(func() { solutionWorkdir = saved })
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "architecture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "architecture", "phase-registry.json"), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	solutionWorkdir = func() (string, error) { return root, nil }
	var out, errb bytes.Buffer
	if code := runSolution([]string{"check", "x"}, nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), "registry warning") || !strings.Contains(errb.String(), filepath.Join(root, "docs", "architecture", "phase-registry.json")) {
		t.Errorf("default root: code = %d, stderr = %q, want 2, the registry warning and the registry of the working directory", code, errb.String())
	}
	solutionWorkdir = func() (string, error) { return "", errors.New("cwd gone") }
	errb.Reset()
	if code := runSolution([]string{"check", "x"}, nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), "cwd gone") {
		t.Errorf("unknown working directory: code = %d, stderr = %q, want 2 and the error", code, errb.String())
	}
	errb.Reset()
	if code := runSolution([]string{"check", "a", "b"}, nil, &out, &errb); code != 2 || !strings.Contains(errb.String(), "usage") {
		t.Errorf("two slugs: code = %d, stderr = %q, want the usage", code, errb.String())
	}
}
