package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var driverLayerHomes = append(slices.Clone(familyNameHomes),
	"internal/llmroute/",
	"internal/cliroute/",
	"internal/policy/",
)

var assignmentTypes = []string{"PhaseRequest", "PhasePlanEntry", "MintSpec"}

var cliSpecificsAllowlist = map[string]int{
	"cmd/evolve/cmd_bridge.go:emitProbeJSON driver-branch":           1,
	"cmd/evolve/routing_run.go:overrideAllowed driver-branch":        1,
	"internal/cli/phasecmd/phases.go:scaffoldProfileJSON schema-key": 1,
	"internal/llmcalls/aggregate.go:lessPerformance driver-branch":   1,
	"internal/resolvellm/resolvellm.go:Result.JSON schema-key":       1,
}

func TestNoCLISpecificsAboveTheDriver(t *testing.T) {
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	found, scanned := cliSpecificSites(t, module)
	if scanned < 500 {
		t.Fatalf("scanned only %d flow files under %s — the guard lost its corpus", scanned, module)
	}
	for site, n := range found {
		if allowed := cliSpecificsAllowlist[site]; n > allowed {
			t.Errorf("%s: %d CLI specific(s) above the driver, %d allowed: keep the assignment neutral and let the bridge or the routing data pick the driver (I-17, ADR-0154)", site, n, allowed)
		}
	}
	for site, allowed := range cliSpecificsAllowlist {
		if n := found[site]; n < allowed {
			t.Errorf("%s: the allowlist permits %d CLI specific(s) but only %d remain: shrink cliSpecificsAllowlist to %d", site, allowed, n, n)
		}
	}
}

func TestCLISpecificSites_FindsAnAssignmentFieldASchemaKeyAndADriverBranch(t *testing.T) {
	module := t.TempDir()
	writeGoFile(t, module, filepath.Join("internal", "core", "req.go"), `package core

type PhaseRequest struct {
	Tier       string
	RoutingCLI string
	Family     string `+"`json:\"cli\"`"+`
}

type Other struct {
	CLI string
}

const schema = `+"`"+`{"phase":"x","cli":"<cli>"}`+"`"+`

func pick(cli, model string, failedCLI string) bool {
	if cli == "" || model == "x" {
		return false
	}
	switch Family(cli) {
	case "x":
	}
	return failedCLI != cli
}

func route(r Request) bool {
	return r.Driver.String() == "codex-tmux" || r.Driver == Bind("codex-tmux") || r.Name.String() == "x"
}

func Family(s string) string { return s }
`)
	writeGoFile(t, module, filepath.Join("internal", "bridge", "driver.go"), `package bridge

func same(cli, other string) bool { return cli == other }
`)
	writeGoFile(t, module, filepath.Join("internal", "core", "req_test.go"), `package core

func fixture(cli string) bool { return cli == "codex" }
`)
	found, scanned := cliSpecificSites(t, module)
	want := map[string]int{
		"internal/core/req.go:PhaseRequest assignment-field": 2,
		"internal/core/req.go:schema schema-key":             1,
		"internal/core/req.go:pick driver-branch":            2,
		"internal/core/req.go:route driver-branch":           2,
	}
	if scanned != 1 || !reflect.DeepEqual(found, want) {
		t.Fatalf("two assignment fields, one schema key, a switch and a comparison; an empty check, the drivers and the tests exempt: scanned=%d\n got %v\nwant %v", scanned, found, want)
	}
}

func cliSpecificSites(t *testing.T, module string) (map[string]int, int) {
	t.Helper()
	found := map[string]int{}
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
		rel = filepath.ToSlash(rel)
		if slices.ContainsFunc(driverLayerHomes, func(home string) bool { return strings.HasPrefix(rel, home) }) {
			return nil
		}
		scanned++
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		for _, decl := range file.Decls {
			for _, site := range declCLISpecifics(decl) {
				found[rel+":"+site]++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found, scanned
}

func declCLISpecifics(decl ast.Decl) []string {
	var out []string
	name := declName(decl)
	ast.Inspect(decl, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeSpec:
			out = append(out, assignmentCLIFields(n)...)
		case *ast.BasicLit:
			if n.Kind == token.STRING && strings.Contains(n.Value, `"cli":`) {
				out = append(out, name+" schema-key")
			}
		case *ast.BinaryExpr:
			if (n.Op == token.EQL || n.Op == token.NEQ) && !isEmptyString(n.X) && !isEmptyString(n.Y) && (isDriverIdentity(n.X) || isDriverIdentity(n.Y)) {
				out = append(out, name+" driver-branch")
			}
		case *ast.SwitchStmt:
			if n.Tag != nil && isDriverIdentity(n.Tag) {
				out = append(out, name+" driver-branch")
			}
		}
		return true
	})
	return out
}

func assignmentCLIFields(spec *ast.TypeSpec) []string {
	st, ok := spec.Type.(*ast.StructType)
	if !ok || !slices.Contains(assignmentTypes, spec.Name.Name) {
		return nil
	}
	var out []string
	for _, field := range st.Fields.List {
		for _, id := range field.Names {
			if strings.Contains(id.Name, "CLI") || field.Tag != nil && strings.Contains(field.Tag.Value, `json:"cli`) {
				out = append(out, spec.Name.Name+" assignment-field")
			}
		}
	}
	return out
}

func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) == 1 {
			return receiverName(d.Recv.List[0].Type) + "." + d.Name.Name
		}
		return d.Name.Name
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				return s.Name.Name
			case *ast.ValueSpec:
				return s.Names[0].Name
			}
		}
	}
	return "file"
}

func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return "recv"
}

func isEmptyString(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && (lit.Value == `""` || lit.Value == "``")
}

func isDriverIdentity(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "String" {
			return isDriverIdentity(sel.X)
		}
		return slices.Contains([]string{"Family", "BaseCLI", "Bind"}, terminalName(e.Fun))
	case *ast.Ident, *ast.SelectorExpr:
		name := strings.ToLower(terminalName(e))
		return strings.HasSuffix(name, "cli") || strings.HasSuffix(name, "driver")
	case *ast.ParenExpr:
		return isDriverIdentity(e.X)
	}
	return false
}

func terminalName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}
