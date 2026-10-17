package structure

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

const mcpConfigReader = "internal/looppreflight/mcpconfig.go"

func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNoTrackedFileRegistersAnMCPServer(t *testing.T) {
	t.Parallel()
	tracked, err := repostate.TrackedTree(filepath.Join("..", "..", ".."), ".")
	if err != nil {
		t.Fatal(err)
	}
	if found := MCPConfigFiles(tracked); len(found) > 0 {
		t.Errorf("tracked MCP config files %v; the repo rule is CLI only (docs/conventions/cli-only-no-mcp.md)", found)
	}

	writers, err := MCPRegistrars(filepath.Join("..", ".."), ".")
	if err != nil {
		t.Fatal(err)
	}
	writers = slices.DeleteFunc(writers, func(rel string) bool {
		return filepath.Dir(rel) == filepath.Join("test", "structure") || filepath.ToSlash(rel) == mcpConfigReader
	})
	if len(writers) > 0 {
		t.Errorf("non-test Go files that register an MCP server: %v; reach the service through a CLI", writers)
	}
}

func TestTheMCPConfigReaderOnlyReads(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	var writes []string
	for importPath, members := range map[string][]string{
		"os":            {"WriteFile", "Create", "OpenFile"},
		"encoding/json": {"Marshal", "MarshalIndent", "NewEncoder"},
	} {
		uses, err := SelectorUses(root, mcpConfigReader, importPath, members...)
		if err != nil {
			t.Fatal(err)
		}
		writes = append(writes, uses...)
	}
	if len(writes) > 0 {
		t.Errorf("%s names the mcpServers key so preflight can warn; it must never write a config: %v", mcpConfigReader, writes)
	}
	importers, err := ExecImporters(root, filepath.Dir(mcpConfigReader))
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(importers, func(rel string) bool { return filepath.ToSlash(rel) == mcpConfigReader }) {
		t.Errorf("%s imports os/exec or sysexec; the reader must only read files", mcpConfigReader)
	}
}

func TestMCPConfigFiles_MatchesTheBaseNameAtAnyDepth(t *testing.T) {
	t.Parallel()

	got := MCPConfigFiles([]string{".mcp.json", "a/b/.mcp.json", "docs/mcp.json.md", "x.mcp.json"})

	if want := []string{".mcp.json", "a/b/.mcp.json"}; !slices.Equal(got, want) {
		t.Errorf("MCPConfigFiles = %v, want %v", got, want)
	}
}

func TestMCPRegistrars_FlagsEachWayToRegisterAServer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixture(t, root, "keyed/w.go", "package keyed\n\nvar k = `{\"mcpServers\": {}}`\n")
	writeFixture(t, root, "addcmd/w.go", "package addcmd\n\nvar c = \"claude mcp add trail\"\n")
	writeFixture(t, root, "agycmd/w.go", "package agycmd\n\nvar c = \"agy mcp list\"\n")
	writeFixture(t, root, "argv/w.go", "package argv\n\nimport \"os/exec\"\n\nvar c = exec.Command(\"claude\", \"mcp\", \"add\")\n")
	writeFixture(t, root, "flag/w.go", "package flag\n\nvar f = []string{\"--mcp-config\", \"x\"}\n")
	writeFixture(t, root, "word/w.go", "package word\n\nvar w = \"mcp\"\n")
	writeFixture(t, root, "keyed/w_test.go", "package keyed\n\nvar t = \"claude mcp add\"\n")

	got, err := MCPRegistrars(root, ".")

	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		got[i] = filepath.ToSlash(got[i])
	}
	slices.Sort(got)
	want := []string{"addcmd/w.go", "agycmd/w.go", "argv/w.go", "flag/w.go", "keyed/w.go"}
	if !slices.Equal(got, want) {
		t.Errorf("MCPRegistrars = %v, want %v (a bare \"mcp\" word without a process runner is not a registration)", got, want)
	}
}

func TestMCPRegistrars_ReportsAParseError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixture(t, root, "bad/w.go", "package bad\n\nvar = \n")

	if _, err := MCPRegistrars(root, "."); err == nil {
		t.Error("MCPRegistrars on an unparsable file returned no error")
	}
}
