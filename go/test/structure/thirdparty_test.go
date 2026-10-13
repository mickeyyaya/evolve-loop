package structure

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var thirdPartyHomes = map[string]Home{
	"gopkg.in/yaml.v3":   {Dir: "internal/adapters/yamlcodec"},
	"pgregory.net/rapid": {TestOnly: true},
}

const fixtureGoMod = "module example.com/m\n\ngo 1.23\n\nrequire pgregory.net/rapid v1.1.0\n\nrequire (\n\tgopkg.in/yaml.v3 v3.0.1\n\texample.org/indirect v1.0.0 // indirect\n)\n"

func TestEveryThirdPartyImportHasExactlyOneHome(t *testing.T) {
	t.Parallel()
	violations, err := ThirdPartyViolations(filepath.Join("..", ".."), thirdPartyHomes)
	if err != nil {
		t.Fatalf("ThirdPartyViolations = %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("%d import(s) outside their declared home:\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

func TestRequiredModules_ListsDirectRequiresInBothForms(t *testing.T) {
	t.Parallel()

	got := RequiredModules([]byte(fixtureGoMod))

	want := []string{"gopkg.in/yaml.v3", "pgregory.net/rapid"}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("RequiredModules = %v, want %v (indirect requires are not imported directly)", got, want)
	}
}

func fixtureViolations(t *testing.T, files map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	tree := map[string]string{"go.mod": fixtureGoMod}
	for path, body := range files {
		tree[path] = body
	}
	writeTree(t, root, tree)
	violations, err := ThirdPartyViolations(root, thirdPartyHomes)
	if err != nil {
		t.Fatalf("ThirdPartyViolations = %v", err)
	}
	return violations
}

func TestThirdPartyViolations_AcceptsTheDeclaredHomes(t *testing.T) {
	t.Parallel()

	violations := fixtureViolations(t, map[string]string{
		"internal/adapters/yamlcodec/y.go":        "package y\nimport _ \"gopkg.in/yaml.v3\"\n",
		"internal/adapters/yamlcodec/sub/deep.go": "package s\nimport _ \"gopkg.in/yaml.v3\"\n",
		"internal/other/o_test.go":                "package o\nimport _ \"pgregory.net/rapid\"\nimport _ \"gopkg.in/yaml.v3\"\n",
		"vendor/x/x.go":                           "package x\nimport _ \"gopkg.in/yaml.v3\"\n",
		"internal/other/testdata/t.go":            "package t\nimport _ \"gopkg.in/yaml.v3\"\n",
	})

	if len(violations) != 0 {
		t.Errorf("violations = %v, want none", violations)
	}
}

func TestThirdPartyViolations_RejectsADirectYamlImportOutsideTheAdapter(t *testing.T) {
	t.Parallel()

	violations := fixtureViolations(t, map[string]string{
		"internal/faillearn/f.go": "package f\nimport \"gopkg.in/yaml.v3\"\nvar _ = yaml.Marshal\n",
	})

	if len(violations) != 1 || !strings.Contains(violations[0], "internal/faillearn/f.go") || !strings.Contains(violations[0], "gopkg.in/yaml.v3") {
		t.Errorf("violations = %v, want one naming internal/faillearn/f.go and the module", violations)
	}
}

func TestThirdPartyViolations_RejectsASubpackageImportOutsideTheAdapter(t *testing.T) {
	t.Parallel()

	violations := fixtureViolations(t, map[string]string{
		"cmd/x/main.go": "package main\nimport _ \"gopkg.in/yaml.v3/internal/x\"\n",
	})

	if len(violations) != 1 {
		t.Errorf("violations = %v, want the module's subpackage import rejected", violations)
	}
}

func TestThirdPartyViolations_RejectsRapidInANonTestFile(t *testing.T) {
	t.Parallel()

	violations := fixtureViolations(t, map[string]string{
		"internal/prop/p.go": "package p\nimport _ \"pgregory.net/rapid\"\n",
	})

	if len(violations) != 1 || !strings.Contains(violations[0], "internal/prop/p.go") || !strings.Contains(violations[0], "test files only") {
		t.Errorf("violations = %v, want rapid rejected as test-only", violations)
	}
}

func TestThirdPartyViolations_RejectsAYamlAdapterSiblingPrefix(t *testing.T) {
	t.Parallel()

	violations := fixtureViolations(t, map[string]string{
		"internal/adapters/yamlcodecx/y.go": "package y\nimport _ \"gopkg.in/yaml.v3\"\n",
	})

	if len(violations) != 1 {
		t.Errorf("violations = %v, want a sibling directory sharing the name prefix rejected", violations)
	}
}

func TestThirdPartyViolations_RejectsARequiredModuleWithoutAHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": fixtureGoMod})

	violations, err := ThirdPartyViolations(root, map[string]Home{"gopkg.in/yaml.v3": {Dir: "internal/adapters/yamlcodec"}})

	if err != nil || len(violations) != 1 || !strings.Contains(violations[0], "pgregory.net/rapid") {
		t.Errorf("ThirdPartyViolations = %v, %v, want the unlisted require reported", violations, err)
	}
}

func TestThirdPartyViolations_IgnoresStdlibAndModuleLocalImports(t *testing.T) {
	t.Parallel()

	violations := fixtureViolations(t, map[string]string{
		"a/a.go": "package a\nimport _ \"os\"\nimport _ \"example.com/m/internal/b\"\n",
	})

	if len(violations) != 0 {
		t.Errorf("violations = %v, want none", violations)
	}
}

func TestThirdPartyViolations_ReportsMissingGoModAndUnparsableSource(t *testing.T) {
	t.Parallel()
	if _, err := ThirdPartyViolations(t.TempDir(), thirdPartyHomes); err == nil {
		t.Error("missing go.mod: err = nil, want the read error")
	}
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": fixtureGoMod, "a/a.go": "package a\nimport (\n"})

	if _, err := ThirdPartyViolations(root, thirdPartyHomes); err == nil {
		t.Error("unparsable source: err = nil, want the parse error")
	}
}

func TestThirdPartyViolations_ReportsAnUnreadableDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTree(t, root, map[string]string{"go.mod": fixtureGoMod, "locked/a.go": "package a\n"})
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	if _, err := ThirdPartyViolations(root, thirdPartyHomes); err == nil {
		t.Error("unreadable directory: err = nil, want the walk error")
	}
}
