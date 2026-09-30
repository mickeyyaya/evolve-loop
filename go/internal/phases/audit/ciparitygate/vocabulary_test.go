package ciparitygate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type typedConst struct{ typ, name, value string }

func TestVocabulary_TypedConstsAreTheRenderedClosedSets(t *testing.T) {
	sets := map[string][]string{"gateStep": asStrings(gateSteps), "gateCause": asStrings(gateCauses), "lockReason": asStrings(lockReasons)}
	for typ, values := range sets {
		seen := map[string]bool{}
		for _, v := range values {
			if seen[v] {
				t.Errorf("%s: %q listed twice", typ, v)
			}
			seen[v] = true
		}
	}
	decls, conversions := scanVocabulary(t, sets)
	if len(decls) != len(gateSteps)+len(gateCauses)+len(lockReasons) {
		t.Errorf("declared %d typed consts, the sets list %d", len(decls), len(gateSteps)+len(gateCauses)+len(lockReasons))
	}
	for _, d := range decls {
		if !contains(sets[d.typ], d.value) {
			t.Errorf("%s %s = %q is declared but not in the rendered set", d.typ, d.name, d.value)
		}
	}
	if len(conversions) != 0 {
		t.Errorf("production code converts a literal into a vocabulary type — declare the const instead: %v", conversions)
	}
	docs := signalcenter.RegisteredCodes()[signalcenter.ModuleAudit]
	for code, want := range map[signalcenter.Code]string{
		CodeGateStepFailed:      "fields.step ∈ " + vocabulary(gateSteps),
		CodeGateFailed:          "fields.cause ∈ " + vocabulary(gateCauses),
		CodeTierLockUnavailable: "fields.reason ∈ " + vocabulary(lockReasons),
	} {
		if doc := docOf(docs, code); !strings.Contains(doc, want) {
			t.Errorf("%s doc must render its set %q; doc: %q", code, want, doc)
		}
	}
	for _, d := range docs {
		if strings.HasPrefix(string(d.Code), "AUDIT_CIPARITY_") && strings.Contains(d.Doc, "|") {
			t.Errorf("%s doc carries a raw pipe — it lands in a markdown table cell unescaped (signalcenter/render.go:29): %q", d.Code, d.Doc)
		}
	}
	for field, writers := range map[string]int{`"step", `: 1, `"cause", `: 1, `"reason", `: 1} {
		if got := productionOccurrences(t, field); got != writers {
			t.Errorf("fields.%s has %d production writers, want %d (the typed producer only)", strings.Trim(field, `", `), got, writers)
		}
	}
}

func scanVocabulary(t *testing.T, types map[string][]string) (decls []typedConst, conversions []string) {
	t.Helper()
	fset := token.NewFileSet()
	for _, name := range productionFiles(t) {
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				decls = append(decls, typedConstsOf(v, types)...)
			case *ast.CallExpr:
				if id, ok := v.Fun.(*ast.Ident); ok && types[id.Name] != nil {
					conversions = append(conversions, name+":"+strconv.Itoa(fset.Position(v.Pos()).Line)+" "+id.Name+"(…)")
				}
			}
			return true
		})
	}
	return decls, conversions
}

func productionFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	return names
}

func typedConstsOf(spec *ast.ValueSpec, types map[string][]string) []typedConst {
	ident, ok := spec.Type.(*ast.Ident)
	if !ok || types[ident.Name] == nil {
		return nil
	}
	var out []typedConst
	for i, name := range spec.Names {
		lit, ok := spec.Values[i].(*ast.BasicLit)
		if !ok {
			continue
		}
		value, _ := strconv.Unquote(lit.Value)
		out = append(out, typedConst{typ: ident.Name, name: name.Name, value: value})
	}
	return out
}

func productionOccurrences(t *testing.T, needle string) int {
	t.Helper()
	n := 0
	for _, name := range productionFiles(t) {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		n += strings.Count(string(src), needle)
	}
	return n
}

func docOf(docs []signalcenter.CodeDoc, code signalcenter.Code) string {
	for _, d := range docs {
		if d.Code == code {
			return d.Doc
		}
	}
	return ""
}

func asStrings[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
