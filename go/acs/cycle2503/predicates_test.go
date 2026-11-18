//go:build acs

package cycle2503

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const inboxcmdPackage = "./internal/cli/inboxcmd"

var (
	buildOnce   sync.Once
	builtBinary string
	buildOutput string
	buildCode   int
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-cycle2503-")
		if err != nil {
			buildCode = -1
			buildOutput = err.Error()
			return
		}
		builtBinary = filepath.Join(dir, "evolve")
		stdout, stderr, code, _ := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "build", "-o", builtBinary, "./cmd/evolve")
		buildCode = code
		buildOutput = stdout + stderr
	})
	if buildCode != 0 {
		t.Fatalf("go build ./cmd/evolve exit %d\n%s", buildCode, acsassert.OutputTail(buildOutput, 40))
	}
	return builtBinary
}

func runEvolve(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBinary(t), args...)
	if code < 0 {
		t.Fatalf("evolve %v did not start: %v", args, err)
	}
	return stdout, stderr, code
}

func nonTestInboxFilesInMain(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve", "cmd_inbox*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, m := range matches {
		if !strings.HasSuffix(m, "_test.go") {
			kept = append(kept, filepath.Base(m))
		}
	}
	slices.Sort(kept)
	return kept
}

func TestC2503_001_TestInboxCmdRunPassesInTheNewPackage(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: inboxcmdPackage,
		Pattern: "^TestInboxCmdRun$",
		Names:   []string{"TestInboxCmdRun"},
	})
}

func TestC2503_002_OnlyTheShimRemainsAmongTheNonTestInboxFilesOfMain(t *testing.T) {
	got := nonTestInboxFilesInMain(t)
	if !slices.Equal(got, []string{"cmd_inbox.go"}) {
		t.Fatalf("non-test cmd_inbox*.go files in go/cmd/evolve = %v, want exactly [cmd_inbox.go]", got)
	}
	shim := filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve", "cmd_inbox.go")
	if !acsassert.FileContains(t, shim, "inboxcmd.Run(") {
		t.Errorf("the shim does not hand over to inboxcmd.Run")
	}
	shimSource, err := os.ReadFile(shim)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(shimSource), "\n"); lines > 40 {
		t.Errorf("the shim has %d lines; a shim is a thin hand-over, not the verb group", lines)
	}
}

func TestC2503_003_TheInboxVerbBehavesTheSameThroughTheProductionBinary(t *testing.T) {
	t.Run("list help is a usage exit", func(t *testing.T) {
		_, stderr, code := runEvolve(t, "inbox", "list", "--help")
		if code != 10 || !strings.Contains(stderr, "usage: evolve inbox list [--status ready|console|waiting] [--kind K] [--route R] [--json]") {
			t.Errorf("inbox list --help: exit %d, stderr:\n%s", code, acsassert.OutputTail(stderr, 12))
		}
	})
	t.Run("group help lists the verbs and exits zero", func(t *testing.T) {
		stdout, _, code := runEvolve(t, "inbox", "--help")
		if code != 0 {
			t.Errorf("inbox --help exit %d, want 0", code)
		}
		for _, verb := range []string{"batches", "list", "route-console", "trailer-check", "ack-fingerprint"} {
			if !strings.Contains(stdout, "evolve inbox "+verb) {
				t.Errorf("inbox --help lacks verb %q:\n%s", verb, stdout)
			}
		}
	})
	t.Run("an unknown verb is a usage error", func(t *testing.T) {
		_, stderr, code := runEvolve(t, "inbox", "nosuchverb")
		if code != 10 || !strings.Contains(stderr, "usage: evolve inbox <verb> ...") {
			t.Errorf("inbox nosuchverb: exit %d, stderr:\n%s", code, acsassert.OutputTail(stderr, 8))
		}
	})
	t.Run("a bad status is a usage error", func(t *testing.T) {
		_, stderr, code := runEvolve(t, "inbox", "list", "--status", "bogus")
		if code != 10 || !strings.Contains(stderr, `--status "bogus" is not one of ready, console, waiting`) {
			t.Errorf("inbox list --status bogus: exit %d, stderr:\n%s", code, acsassert.OutputTail(stderr, 8))
		}
	})
	t.Run("a root without .evolve is an io error that names the root", func(t *testing.T) {
		root := t.TempDir()
		_, stderr, code := runEvolve(t, "inbox", "list", "--project-root", root)
		if code != 2 || !strings.Contains(stderr, "cannot read the project root "+root) {
			t.Errorf("inbox list on a root with no .evolve: exit %d, stderr:\n%s", code, acsassert.OutputTail(stderr, 8))
		}
	})
	t.Run("an empty inbox lists as an empty JSON array", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := runEvolve(t, "inbox", "list", "--json", "--project-root", root)
		if code != 0 || !strings.Contains(stdout, `"schema":"evolve.inbox.list/1"`) || !strings.Contains(stdout, `"data":[]`) {
			t.Errorf("inbox list --json on an empty inbox: exit %d, stdout:\n%s", code, stdout)
		}
	})
	t.Run("the inbox-mover verb still routes", func(t *testing.T) {
		stdout, _, code := runEvolve(t, "inbox-mover", "--help")
		if code != 0 || !strings.Contains(stdout, "Usage: evolve inbox-mover <claim|promote|recover-orphans>") {
			t.Errorf("inbox-mover --help: exit %d, stdout:\n%s", code, acsassert.OutputTail(stdout, 8))
		}
	})
}

func TestC2503_004_TheNewPackageIsEnrolledInTheApicoverGateAndNamesEveryExport(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	enforce, err := os.ReadFile(filepath.Join(moduleDir, ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(enforce), "\n"), inboxcmdPackage) {
		t.Errorf("go/.apicover-enforce has no line %q", inboxcmdPackage)
	}
	pkgDir := filepath.Join(moduleDir, "internal", "cli", "inboxcmd")
	exported := exportedSymbols(t, pkgDir)
	for _, want := range []string{"Run", "Deps"} {
		if !slices.Contains(exported, want) {
			t.Errorf("inboxcmd does not export %q", want)
		}
	}
	named, err := os.ReadFile(filepath.Join(pkgDir, "apicover_named_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range exported {
		if !strings.Contains(string(named), symbol) {
			t.Errorf("apicover_named_test.go does not name the exported symbol %q", symbol)
		}
	}
}

var movedCuratorTests = []string{
	"TestCmd_InboxCurate_AnAlreadyShippedVerdictWhoseCommitIsNotOnMainIsNotTrusted",
	"TestCmd_InboxCurate_ANamedPremiseThatIsStillAbsentOnTheTreeStopsTheReroute",
	"TestCmd_InboxCurate_ANamedPremiseThatLandedOnTheTreeSinceTheVerdictIsRerouted",
	"TestCmd_InboxCurate_AnItemWhoseTextNamesAProtectedFileIsNotProposedForALane",
	"TestCmd_InboxCurate_AStaleItemOfTheWave178BoundaryGetsAConsumeOrQuarantineProposalNotAReroute",
	"TestCmd_InboxCurate_TheWave178BoundaryThroughTheProductionCuratorProposesTheNineKeptAndNoneOfTheSeven",
}

func TestC2503_006_TheTestPackageOfMainCompilesAfterTheMove(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "vet", "-tags", "integration", "./cmd/evolve")
	if err != nil && code < 0 {
		t.Fatalf("go vet did not start: %v", err)
	}
	if code != 0 {
		t.Errorf("go vet -tags integration ./cmd/evolve exit %d: a test file of package main still uses a symbol that moved to inboxcmd\n%s", code, acsassert.OutputTail(stdout+stderr, 20))
	}
}

func TestC2503_007_TheCuratorTestsThatNeedTheMovedFixtureRunInTheNewPackage(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: inboxcmdPackage,
		Pattern: "^(" + strings.Join(movedCuratorTests, "|") + ")$",
		Names:   movedCuratorTests,
	})
	mainDir := filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve")
	for _, file := range []string{"cmd_inbox_curator_reroute_test.go", "cmd_inbox_curator_wave178_test.go"} {
		if _, err := os.Stat(filepath.Join(mainDir, file)); err == nil {
			t.Errorf("go/cmd/evolve/%s still exists: it uses inboxcmd internals and must move with them", file)
		}
	}
}

func TestC2503_005_TheMovedVerbsLeaveNoDeadCopyInMain(t *testing.T) {
	mainDir := filepath.Join(acsassert.GoModuleDir(t), "cmd", "evolve")
	moved := []string{"runInboxList", "runInboxShow", "runInboxRank", "runInboxAdd", "runInboxConsume", "runInboxQuarantine", "runInboxTrailerCheck", "runInboxRouteConsole", "runInboxRouteLane", "runInboxClaim", "runInboxSplit", "runInboxCurate"}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, mainDir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Recv == nil && slices.Contains(moved, fn.Name.Name) {
					t.Errorf("%s still defines %s in package main", filepath.Base(fileName), fn.Name.Name)
				}
			}
		}
	}
}

func exportedSymbols(t *testing.T, dir string) []string {
	t.Helper()
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	var names []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				names = append(names, exportedNames(decl)...)
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func exportedNames(decl ast.Decl) []string {
	var names []string
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil && d.Name.IsExported() {
			names = append(names, d.Name.Name)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if s.Name.IsExported() {
					names = append(names, s.Name.Name)
				}
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if n.IsExported() {
						names = append(names, n.Name)
					}
				}
			}
		}
	}
	return names
}
