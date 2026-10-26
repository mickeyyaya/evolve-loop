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
	"internal/practicewatch/",
	"internal/preflight/",
	"internal/looppreflight/",
	"internal/capability/",
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
	return literalsMatching(t, path, rel, func(value string) bool { return slices.Contains(familyNames, value) })
}

func literalsMatching(t *testing.T, path, rel string, matches func(string) bool) []string {
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
		if value, err := strconv.Unquote(lit.Value); err == nil && matches(value) {
			out = append(out, rel+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+" "+lit.Value)
		}
		return true
	})
	return out
}

var parserPackages = []string{
	"internal/bridge/panestream",
	"internal/bridge/clicontrol",
	"internal/tokenusage",
	"internal/attemptpostmortem",
}

func TestParserPackagesNameNoFamily(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	names, scanned := parserFamilyNames(t, module)
	if scanned < 20 {
		t.Fatalf("scanned only %d parser files under %s — the guard lost its corpus", scanned, module)
	}
	for _, n := range names {
		t.Errorf("%s names a CLI family in a parser or collector package: key the implementation by a format id and let the manifest field parsers, collectors or transcript pick it (I-6, X9)", n)
	}
}

func TestParserFamilyNames_FindsASwitchCaseAMapKeyAndAPrefixOfAFamily(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "tokenusage", "pick.go"), `package tokenusage

import "strings"

var byFamily = map[string]int{"codex": 1}

func pick(d string) bool {
	switch d {
	case "ollama-tmux":
		return true
	}
	return strings.HasPrefix(d, "claude-")
}

const format = "claude-jsonl-v1"
`)
	writeGoFile(t, module, filepath.Join("internal", "tokenusage", "pick_test.go"), `package tokenusage

const fixture = "agy"
`)
	names, scanned := parserFamilyNames(t, module)
	if scanned != 1 || len(names) != 3 {
		t.Fatalf("a map key, a case and a prefix; format ids and tests exempt: scanned=%d %v", scanned, names)
	}
	for i, want := range []string{`:5 "codex"`, `:9 "ollama-tmux"`, `:12 "claude-"`} {
		if !strings.HasSuffix(names[i], want) {
			t.Errorf("finding %d = %q, want the suffix %q", i, names[i], want)
		}
	}
}

func parserFamilyNames(t *testing.T, module string) ([]string, int) {
	t.Helper()
	var names []string
	scanned := 0
	for _, pkg := range parserPackages {
		files, err := filepath.Glob(filepath.Join(module, filepath.FromSlash(pkg), "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			rel, _ := filepath.Rel(module, path)
			scanned++
			names = append(names, literalsMatching(t, path, rel, isFamilyOrPrefix)...)
		}
	}
	return names, scanned
}

func isFamilyOrPrefix(value string) bool {
	return slices.Contains(familyNames, value) || slices.Contains(familyNames, strings.TrimSuffix(value, "-")) && strings.HasSuffix(value, "-")
}
