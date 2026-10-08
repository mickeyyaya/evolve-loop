package ipcenv_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func declaredKeyConsts(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	fset := token.NewFileSet()
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, name := range spec.Names {
				if lit, ok := spec.Values[i].(*ast.BasicLit); ok && name.IsExported() && lit.Kind == token.STRING {
					value, _ := strconv.Unquote(lit.Value)
					keys = append(keys, value)
				}
			}
			return true
		})
	}
	slices.Sort(keys)
	return keys
}

func TestProtocolKeys_AreEveryKeyThePackageDeclaresAndEachIsScrubbed(t *testing.T) {
	declared := declaredKeyConsts(t)
	listed := slices.Sorted(slices.Values(ipcenv.ProtocolKeys()))
	if !slices.Equal(declared, listed) {
		t.Fatalf("ProtocolKeys() = %v, declared key constants = %v: a key added to the package must join the set every consumer refuses to forward", listed, declared)
	}
	for _, key := range listed {
		if got := ipcenv.Scrub([]string{key + "=x"}); len(got) != 0 {
			t.Errorf("Scrub kept protocol key %s", key)
		}
	}
}

func TestProtocolKeys_ReturnsAFreshSlice(t *testing.T) {
	keys := ipcenv.ProtocolKeys()
	keys[0] = "MUTATED"
	if ipcenv.ProtocolKeys()[0] == "MUTATED" {
		t.Error("ProtocolKeys shares its backing array; a caller could rewrite the protocol set")
	}
}

func TestTmuxSocketKey_IsTheBridgeSocketChannel(t *testing.T) {
	if ipcenv.TmuxSocketKey != "EVOLVE_TMUX_SOCKET" {
		t.Errorf("TmuxSocketKey = %q, want EVOLVE_TMUX_SOCKET, the socket name the loop exports to its bridge subprocesses", ipcenv.TmuxSocketKey)
	}
}

func TestDispatchIDKey_IsTheDispatchTag(t *testing.T) {
	if ipcenv.DispatchIDKey != "EVOLVE_DISPATCH_ID" {
		t.Errorf("DispatchIDKey = %q, want EVOLVE_DISPATCH_ID, the tag the bridge sets on each process of a dispatch", ipcenv.DispatchIDKey)
	}
}
