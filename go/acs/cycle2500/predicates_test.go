//go:build acs

package cycle2500

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const funcSource = "package fx\n\nfunc Exported() int { return 1 }\n"

const constSource = "package fx\n\nconst ExportedConst = 3\n"

const funcCoverLine = 3

const typeSource = "package fx\n\ntype Shape interface{ Area() int }\n"

const typeAndFuncSource = "package fx\n\ntype Shape struct{}\n\nfunc Exported() int { return 1 }\n"

type apicoverRun struct {
	code   int
	stdout string
	stderr string
}

func runApicover(t *testing.T, source, testSource string, coverFunc string) apicoverRun {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":      "module example.com/fx\n\ngo 1.23\n",
		"pkg.go":      source,
		"pkg_test.go": testSource,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-enforce"}
	if coverFunc != "" {
		coverPath := filepath.Join(t.TempDir(), "cover.func")
		line := fmt.Sprintf("example.com/fx/pkg.go:%d:\t%s\t100.0%%\n", funcCoverLine, coverFunc)
		if err := os.WriteFile(coverPath, []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, "-cover", coverPath)
	}
	args = append(args, dir)
	var out, errBuf bytes.Buffer
	code := apicover.Main(args, &out, &errBuf)
	return apicoverRun{code: code, stdout: out.String(), stderr: errBuf.String()}
}

func requireUnnamed(t *testing.T, r apicoverRun, symbol string) {
	t.Helper()
	if r.code != 1 {
		t.Errorf("apicover -enforce exit=%d, want 1 (%s must be reported unnamed)\nstdout:\n%s\nstderr:\n%s", r.code, symbol, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "UNCOVERED") || !strings.Contains(r.stdout, symbol) {
		t.Errorf("report must list %s under UNCOVERED; got:\n%s", symbol, r.stdout)
	}
	if !strings.Contains(r.stdout, "0 covered") {
		t.Errorf("summary must show 0 covered; got:\n%s", r.stdout)
	}
}

func requireNamed(t *testing.T, r apicoverRun, symbol string) {
	t.Helper()
	if r.code != 0 {
		t.Errorf("apicover -enforce exit=%d, want 0 (%s is named)\nstdout:\n%s\nstderr:\n%s", r.code, symbol, r.stdout, r.stderr)
	}
	if strings.Contains(r.stdout, "UNCOVERED") || strings.Contains(r.stdout, "FALSE-GREEN") {
		t.Errorf("%s must not be reported as a problem; got:\n%s", symbol, r.stdout)
	}
	if !strings.Contains(r.stdout, "1 covered, 0 uncovered") {
		t.Errorf("summary must show 1 covered, 0 uncovered; got:\n%s", r.stdout)
	}
}

func TestC2500_001_PackageLevelBlankVarDoesNotNameFunc(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\nvar _ = Exported\n\nfunc TestNothing(t *testing.T) {}\n"
	requireUnnamed(t, runApicover(t, funcSource, testSource, "Exported"), "Exported")
}

func TestC2500_002_BlankAssignInsideTestDoesNotNameFunc(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {\n\t_ = Exported\n}\n"
	requireUnnamed(t, runApicover(t, funcSource, testSource, "Exported"), "Exported")
}

func TestC2500_003_CallingFuncNamesIt(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\nfunc TestExported(t *testing.T) {\n\tif Exported() != 1 {\n\t\tt.Fatal(\"want 1\")\n\t}\n}\n"
	requireNamed(t, runApicover(t, funcSource, testSource, "Exported"), "Exported")
}

func TestC2500_004_BlankVarDoesNotNameConst(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\nvar _ = ExportedConst\n\nfunc TestNothing(t *testing.T) {}\n"
	requireUnnamed(t, runApicover(t, constSource, testSource, ""), "ExportedConst")
}

func TestC2500_005_AssertingConstValueNamesIt(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\nfunc TestConst(t *testing.T) {\n\tif ExportedConst != 3 {\n\t\tt.Fatal(\"want 3\")\n\t}\n}\n"
	requireNamed(t, runApicover(t, constSource, testSource, ""), "ExportedConst")
}

func TestC2500_006_CallOnlyInOneFileStillNamesWhenAnotherFileUsesBlankVar(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":        "module example.com/fx\n\ngo 1.23\n",
		"pkg.go":        funcSource,
		"blank_test.go": "package fx\n\nvar _ = Exported\n",
		"call_test.go":  "package fx\n\nimport \"testing\"\n\nfunc TestCall(t *testing.T) {\n\tExported()\n}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	coverPath := filepath.Join(t.TempDir(), "cover.func")
	line := fmt.Sprintf("example.com/fx/pkg.go:%d:\tExported\t100.0%%\n", funcCoverLine)
	if err := os.WriteFile(coverPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	if code := apicover.Main([]string{"-enforce", "-cover", coverPath, dir}, &out, &errBuf); code != 0 {
		t.Errorf("a call in any test file names the symbol; exit=%d\nstdout:\n%s\nstderr:\n%s", code, out.String(), errBuf.String())
	}
}

func TestC2500_007_PackageLevelTypedBlankVarNamesItsType(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\ntype impl struct{}\n\nfunc (impl) Area() int { return 0 }\n\nvar fake impl\n\nvar _ Shape = fake\n\nfunc TestNothing(t *testing.T) {}\n"
	requireNamed(t, runApicover(t, typeSource, testSource, ""), "Shape")
}

func TestC2500_008_TypedBlankVarInsideTestNamesItsTypeButNotItsValue(t *testing.T) {
	testSource := "package fx\n\nimport \"testing\"\n\nfunc TestShape(t *testing.T) {\n\tbuilt := make(chan int)\n\tvar _ Shape = built\n\t_ = Exported\n}\n"
	r := runApicover(t, typeAndFuncSource, testSource, "Exported")
	if r.code != 1 {
		t.Errorf("apicover -enforce exit=%d, want 1 (Exported is only blank-bound)\nstdout:\n%s", r.code, r.stdout)
	}
	if !strings.Contains(r.stdout, "func    Exported") || strings.Contains(r.stdout, "type    Shape") {
		t.Errorf("Exported must be UNCOVERED and Shape must be named; got:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "1 covered, 1 uncovered") {
		t.Errorf("summary must show 1 covered, 1 uncovered; got:\n%s", r.stdout)
	}
}

func TestC2500_009_TypedBlankAssertionInEnforcedPackagesNamesTheirTypes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	var out, errBuf bytes.Buffer
	apicover.Main([]string{
		filepath.Join(root, "go", "internal", "scopedelta"),
		filepath.Join(root, "go", "internal", "mergegate"),
	}, &out, &errBuf)
	if errBuf.Len() != 0 {
		t.Fatalf("apicover stderr: %s", errBuf.String())
	}
	for _, symbol := range []string{"ClosureRule", "Executor"} {
		if strings.Contains(out.String(), "type    "+symbol) {
			t.Errorf("%s is named by a typed blank assertion in the package tests and must not be UNCOVERED; got:\n%s", symbol, out.String())
		}
	}
	if !strings.Contains(out.String(), "exported") {
		t.Errorf("report must cover both packages; got:\n%s", out.String())
	}
}
