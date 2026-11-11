package gatehermetic

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

type toolchain struct {
	g       Graph
	pkg     string
	pi      *pkgInfo
	f       *facts
	rules   Rules
	key     string
	verbIdx int
}

func ToolchainAudit(g Graph, pkg string, rules Rules) ([]Refusal, error) {
	pi := g.pkgs[pkg]
	if pi == nil {
		return nil, fmt.Errorf("gatehermetic: the loaded program has no package %s", pkg)
	}
	cmd, verbIdx, err := toolchainFunc(pi, rules.Toolchain)
	if err != nil {
		return nil, err
	}
	tc := toolchain{g: g, pkg: pkg, pi: pi, f: scanPackage(&g, pi, &rules, cmd), rules: rules, key: cmd.FullName(), verbIdx: verbIdx}
	out := auditRefusals(g, pkg, tc.f, classMtime, classGitDir, classHistory)
	out = append(out, tc.execs()...)
	out = append(out, tc.verbs()...)
	out = append(out, tc.fields()...)
	out = append(out, tc.imports()...)
	sortRefusals(out)
	return out, nil
}

func toolchainFunc(pi *pkgInfo, tc ToolchainFunc) (*types.Func, int, error) {
	fn, _ := pi.pkg.Types.Scope().Lookup(tc.Name).(*types.Func)
	if fn == nil {
		return nil, 0, fmt.Errorf("gatehermetic: the package %s has no function %s", pi.pkg.Path, tc.Name)
	}
	idx := paramIndex(fn.Type().(*types.Signature), tc.Verb)
	if idx < 0 {
		return nil, 0, fmt.Errorf("gatehermetic: the function %s.%s has no parameter %q", pi.pkg.Path, tc.Name, tc.Verb)
	}
	return fn, idx, nil
}

func violation(pkg, class, site string) Refusal {
	return Refusal{Root: Root{Package: pkg}, Class: class, Site: site}
}

func (tc toolchain) site(pos token.Pos) string { return tc.g.program.Fset.Position(pos).String() }

func (tc toolchain) execs() []Refusal {
	var out []Refusal
	inside := 0
	for _, e := range tc.f.execs {
		site := tc.site(e.pos)
		switch {
		case e.fn != tc.key:
			out = append(out, violation(tc.pkg, "toolchain-exec", site))
		case inside > 0 || !e.known || !slices.ContainsFunc(tc.rules.Allowed, func(c Command) bool { return c.Name == e.prog }):
			out = append(out, violation(tc.pkg, "toolchain-exec", site))
			inside++
		default:
			inside++
		}
	}
	return out
}

func (tc toolchain) verbs() []Refusal {
	var out []Refusal
	for _, w := range tc.f.watched {
		switch {
		case w.call == nil:
			out = append(out, violation(tc.pkg, "toolchain-value", tc.site(w.pos)))
		case !tc.verbAllowed(w.call):
			out = append(out, violation(tc.pkg, "toolchain-verb", tc.site(w.pos)))
		}
	}
	return out
}

func (tc toolchain) fields() []Refusal {
	var out []Refusal
	for _, u := range tc.f.fields {
		if u.fn != tc.key {
			out = append(out, violation(tc.pkg, "toolchain-field", tc.site(u.pos)))
		}
	}
	return out
}

func (tc toolchain) imports() []Refusal {
	var out []Refusal
	for _, imp := range tc.pi.pkg.Types.Imports() {
		p := tc.g.program.Packages[imp.Path()]
		if (p == nil || !p.Standard) && !slices.Contains(tc.rules.Exempt, imp.Path()) {
			out = append(out, violation(tc.pkg, "toolchain-import", imp.Path()))
		}
	}
	return out
}

func (tc toolchain) verbAllowed(call *ast.CallExpr) bool {
	if tc.verbIdx >= len(call.Args) {
		return false
	}
	verb, ok := constString(tc.pi.pkg.Info, call.Args[tc.verbIdx])
	return ok && slices.ContainsFunc(tc.rules.Allowed, func(c Command) bool { return c.Verb == verb })
}
