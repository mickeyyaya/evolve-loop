package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var familyNames = []string{
	"claude", "claude-tmux", "claude-p",
	"codex", "codex-tmux",
	"agy", "agy-tmux", "agy-claude", "agy-claude-tmux",
	"ollama", "ollama-tmux",
	"gemini",
}

var familyNameHomes = []string{
	"internal/bridge/",
	"internal/bridgechain/",
	"internal/llmroute/llmroute.go",
	"internal/modelquery/",
	"internal/detectcli/",
	"internal/setup/",
	"internal/tokenusage/",
	"internal/practicewatch/",
	"internal/preflight/",
	"internal/looppreflight/",
	"internal/capability/",
	"internal/attemptpostmortem/",
	"internal/cli/opscmd/doctor_versions.go",
	"internal/cliroute/cliroutetest/",
	"cmd/evolve-fake-cli/",
	"cmd/evolve/cmd_install.go",
	"cmd/evolve/cmd_skills_publish.go",
	"cmd/evolve/cmd_release_verify_clis.go",
	"cmd/evolve/cmd_models_live.go",
}

func TestFlowCodeNamesNoFamily(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	names, scanned := flowFamilyNames(t, module)
	if scanned < 500 {
		t.Fatalf("scanned only %d flow files under %s — the guard lost its corpus", scanned, module)
	}
	for _, n := range names {
		t.Errorf("%s names a CLI family in flow code: move the value into the routing data (policy routing_defaults.json or cli_routing) and read it from there (I-6, ADR-0154)", n)
	}
}

func TestFlowFamilyNames_FindsAFamilyLiteralOutsideTheDriversAndTheData(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "core", "seat.go"), `package core

const fallback = "claude-tmux"

var prose = "a claude family seat"
`)
	writeGoFile(t, module, filepath.Join("internal", "bridge", "driver_x.go"), `package bridge

const name = "agy-tmux"
`)
	writeGoFile(t, module, filepath.Join("internal", "core", "seat_test.go"), `package core

const fixture = "codex"
`)
	names, scanned := flowFamilyNames(t, module)
	if scanned != 1 || len(names) != 1 || !strings.Contains(names[0], filepath.Join("internal", "core", "seat.go")+":3 \"claude-tmux\"") {
		t.Fatalf("one exact family literal in flow code; prose, drivers and tests exempt: scanned=%d %v", scanned, names)
	}
}

func flowFamilyNames(t *testing.T, module string) ([]string, int) {
	t.Helper()
	var names []string
	scanned := 0
	err := filepath.WalkDir(module, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor" || d.Name() == "acs") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(module, path)
		if slices.ContainsFunc(familyNameHomes, func(home string) bool { return strings.HasPrefix(filepath.ToSlash(rel), home) }) {
			return nil
		}
		scanned++
		names = append(names, familyLiterals(t, path, rel)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names, scanned
}

func familyLiterals(t *testing.T, path, rel string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if value, err := strconv.Unquote(lit.Value); err == nil && slices.Contains(familyNames, value) {
			out = append(out, rel+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+" "+lit.Value)
		}
		return true
	})
	return out
}
