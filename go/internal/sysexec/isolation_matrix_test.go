package sysexec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/isolation"
)

var llmLaunchSitesOutsideTheBridge = []string{
	"internal/cliupdate/exec.go",
	"internal/looppreflight/versioninventory.go",
	"internal/modelquery/exec.go",
}

func sysexecUses(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var uses []string
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, isIdent := sel.X.(*ast.Ident); isIdent && id.Name == "sysexec" {
			uses = append(uses, fmt.Sprintf("%s sysexec.%s", fset.Position(sel.Pos()), sel.Sel.Name))
		}
		return true
	})
	return uses, nil
}

func TestSysexecUses_FindsEachSelectorOfThePackage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	if err := writeFile(path, "package x\n\nvar run sysexec.RunFunc\nvar cmd = sysexec.Command(nil, \"agy\")\nvar other = strings.TrimSpace\n"); err != nil {
		t.Fatal(err)
	}
	uses, err := sysexecUses(path)
	if err != nil || len(uses) != 2 {
		t.Fatalf("uses = %v, err = %v; want the RunFunc type and the Command call", uses, err)
	}
	if _, err := sysexecUses(filepath.Join(dir, "absent.go")); err == nil {
		t.Fatal("an absent launch site must fail, not report no spawn")
	}
}

func TestIsolationMatrix_N2_NoLLMSpawnOutsideTheBridgeLaunchAPI(t *testing.T) {
	t.Parallel()
	var spawns []string
	for _, rel := range llmLaunchSitesOutsideTheBridge {
		uses, err := sysexecUses(filepath.Join("..", "..", rel))
		if err != nil {
			t.Fatal(err)
		}
		spawns = append(spawns, uses...)
	}
	var failure error
	if len(spawns) > 0 {
		failure = fmt.Errorf("%d process start(s) of an LLM binary outside internal/bridge, first %s", len(spawns), spawns[0])
	}
	note, err := isolation.Judge("N2", failure)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(note)
}
