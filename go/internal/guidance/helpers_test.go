package guidance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const agentsFixture = "# AGENTS\n\n<!-- instructions-digest BEGIN -->\n" + L0Begin + " old -->\nold\n" + L0End + "\n<!-- instructions-digest END -->\n"

type ruleSpec struct {
	id, statement, layer, status, extra string
}

func ruleText(s ruleSpec) string {
	if s.statement == "" {
		s.statement = "Rule " + s.id + " holds."
	}
	if s.layer == "" {
		s.layer = "L0"
	}
	if s.status == "" {
		s.status = StatusActive
	}
	return "---\nid: " + s.id + "\nstatement: " + s.statement + "\nlayer: " + s.layer + "\naudience: [all]\nsource: test\nstatus: " + s.status + "\nadded: 2026-10-26\nevidence: a test\n" + s.extra + "---\nWhy: a test.\n"
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureTree(t *testing.T, specs ...ruleSpec) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, AgentsPath, agentsFixture)
	if err := os.MkdirAll(filepath.Join(root, "guidance", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range specs {
		writeFile(t, root, BodyPath(s.id), ruleText(s))
	}
	return root
}

func builtTree(t *testing.T, specs ...ruleSpec) string {
	t.Helper()
	root := fixtureTree(t, specs...)
	rules, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := Outputs(root, rules)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, outs); err != nil {
		t.Fatal(err)
	}
	return root
}

func hasProblem(problems []string, prefix, fragment string) bool {
	for _, p := range problems {
		if strings.HasPrefix(p, prefix) && strings.Contains(p, fragment) {
			return true
		}
	}
	return false
}
