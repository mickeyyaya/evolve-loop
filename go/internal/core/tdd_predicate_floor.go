package core

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/testdup"
)

const tddFloorReasonHead = "tdd handoff floor: "

type tddPredicateFloor struct{}

func withMandatoryReviewers(next DeliverableReviewer) DeliverableReviewer {
	return chainReviewer{tddPredicateFloor{}, withMandatoryExplanationReviewer(next)}
}

func (tddPredicateFloor) Review(_ context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseTDD) || in.Worktree == "" || in.Cycle <= 0 {
		return ReviewResult{Approve: true}
	}
	moduleDir := filepath.Join(in.Worktree, "go")
	groups, err := tddHelperCopyGroups(moduleDir, in.Cycle)
	if err != nil {
		fmt.Fprintf(in.errOut(), "[tdd-floor] WARN helper-copy check skipped: %v; the ship ratchet still counts the copies\n", err)
		return ReviewResult{Approve: true}
	}
	if len(groups) == 0 {
		return ReviewResult{Approve: true}
	}
	kit := loadTddKit(moduleDir)
	failures := make([]string, len(groups))
	lines := make([]string, len(groups))
	for i, g := range groups {
		failures[i] = strings.Join(g.Sites, " = ")
		lines[i] = failures[i] + " — " + tddCopyRemedy(g, in.Cycle, kit)
	}
	reason := fmt.Sprintf("%s%d helper body copy group(s) in go/acs/cycle%d/ — no later phase can change these files, so fix them now: use pkg/acsassert (go/acs/README.md §Predicate kit) or the helper named on each line instead of a copy (docs/architecture/test-system.md §6.4, §6.5):%s%s",
		tddFloorReasonHead, len(groups), in.Cycle, floorFailureSeparator, strings.Join(lines, floorFailureSeparator))
	fmt.Fprintf(in.errOut(), "[tdd-floor] REJECT: %s\n", reason)
	return ReviewResult{Approve: false, Retry: true, Reason: reason, Failures: failures}
}

func tddHelperCopyGroups(moduleDir string, cycle int) ([]testdup.Group, error) {
	raw, err := os.ReadFile(filepath.Join(moduleDir, testdup.BaselineRelPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	baseline, err := testdup.ParseBaseline(raw)
	if err != nil {
		return nil, err
	}
	report, err := testdup.Scan(moduleDir, baseline.Date)
	if err != nil {
		return nil, err
	}
	return testdup.GroupsUnder(report, fmt.Sprintf("acs/cycle%d", cycle)), nil
}

type tddKit struct {
	forms   map[string]string
	testing map[string]bool
	bodies  map[string]string
}

func newTddKit() tddKit {
	return tddKit{forms: map[string]string{}, testing: map[string]bool{}, bodies: map[string]string{}}
}

func loadTddKit(moduleDir string) tddKit {
	dir := filepath.Join(moduleDir, "pkg", "acsassert")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return newTddKit()
	}
	kit := newTddKit()
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return newTddKit()
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return newTddKit()
		}
		kit.absorb(file)
		kit.absorbBodies(fset, file, src)
	}
	for _, form := range kit.forms {
		kit.testing[form] = true
	}
	return kit
}

func (k tddKit) absorbBodies(fset *token.FileSet, file *ast.File, src []byte) {
	for fn, body := range testdup.NormalizedBodies(fset, file, src) {
		if k.testing[fn] {
			k.bodies[body] = "acsassert." + fn
		}
	}
}

func (k tddKit) absorb(file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.IsExported() && takesTesting(d) {
				k.testing[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				k.absorbForms(spec)
			}
		}
	}
}

func takesTesting(fn *ast.FuncDecl) bool {
	params := fn.Type.Params.List
	if len(params) == 0 {
		return false
	}
	return slices.Contains([]string{"*testing.T", "testing.TB"}, types.ExprString(params[0].Type))
}

func (k tddKit) absorbForms(spec ast.Spec) {
	value, ok := spec.(*ast.ValueSpec)
	if !ok || len(value.Names) != 1 || value.Names[0].Name != "TestingForms" || len(value.Values) != 1 {
		return
	}
	lit, ok := value.Values[0].(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, elt := range lit.Elts {
		pair, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, form := stringLit(pair.Key), stringLit(pair.Value)
		if key != "" && form != "" {
			k.forms[key] = form
		}
	}
}

func stringLit(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	text, _ := strconv.Unquote(lit.Value)
	return text
}

func tddCopyRemedy(g testdup.Group, cycle int, kit tddKit) string {
	cycleDir := fmt.Sprintf("acs/cycle%d", cycle)
	if !slices.ContainsFunc(g.Sites, func(site string) bool { return siteDir(site) != cycleDir }) {
		return "call the existing helper"
	}
	const cause = "a test helper in another package cannot be imported: "
	var forms, direct []string
	for _, call := range g.Wraps {
		if form, ok := kit.forms[call]; ok {
			forms = append(forms, "acsassert."+form)
		} else if kit.testing[call] {
			direct = append(direct, "acsassert."+call)
		}
	}
	sort.Strings(forms)
	forms = slices.Compact(forms)
	switch {
	case kit.bodies[g.Body] != "":
		return cause + "call " + kit.bodies[g.Body]
	case len(forms) > 0:
		return cause + "call " + strings.Join(forms, " or ")
	case len(direct) > 0 && len(direct) == len(g.Wraps):
		return cause + "inline it, or call " + strings.Join(direct, " or ") + " directly"
	}
	return cause + "write the statements inside the test function and define no helper function, because a renamed helper is still a copy"
}

func siteDir(site string) string {
	file, _, _ := strings.Cut(site, ":")
	return path.Dir(file)
}
