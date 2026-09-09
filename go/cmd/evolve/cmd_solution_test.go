package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSolutionCheck — ADR-0099 slice 2: `evolve solution check <dir>` is the
// eval [code] grader and the agent's self-check for a document deliverable;
// it runs the same engine as the build floor and the audit gate. Exit 0 =
// well-formed, 1 = violations (printed one per line), 2 = usage.
func TestSolutionCheck(t *testing.T) {
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
