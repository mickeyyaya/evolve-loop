package bridge

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"
)

func findDoctorArgvLiterals(src []byte) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "doctor.go", src, 0)
	if err != nil {
		return []string{"parse error: " + err.Error()}
	}
	var offenders []string
	banned := map[string]bool{
		"-p":                   true,
		"--model":              true,
		"-m":                   true,
		"Reply only: PROBE_OK": true,
		"PROBE_OK":             true,
		"haiku":                true,
		"sonnet":               true,
		"opus":                 true,
		"exec":                 true,
	}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if banned[val] {
			pos := fset.Position(lit.Pos())
			offenders = append(offenders, fmt.Sprintf("%d:%s", pos.Line, val))
		}
		return true
	})
	return offenders
}

func TestDoctorSource_HasNoLLMArgvLiteral(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("doctor.go")
	if err != nil {
		t.Fatalf("read doctor.go: %v", err)
	}
	offenders := findDoctorArgvLiterals(src)
	if len(offenders) > 0 {
		t.Errorf("doctor.go contains LLM argv literals: %v", offenders)
	}
}

func TestDoctorArgvGuard_FlagsAFixtureWithALiteral(t *testing.T) {
	t.Parallel()
	const fixture = "package bridge\n\nfunc badProbe() []string {\n\treturn []string{\"-p\", \"Reply only: PROBE_OK\", \"--model\", \"haiku\"}\n}\n"
	offenders := findDoctorArgvLiterals([]byte(fixture))
	if len(offenders) == 0 {
		t.Fatal("guard failed to flag fixture with LLM argv literals")
	}
}
