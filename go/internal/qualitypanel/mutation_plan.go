package qualitypanel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

var killGuardPaths = []string{
	"go/internal/proctree/",
	"go/internal/gc/",
	"go/internal/bridge/dispatch_reap.go",
}

var signalCallNames = map[string]bool{"Kill": true, "Killpg": true, "Signal": true}

type mutationPlan struct {
	targets  []MutationTarget
	excluded []MutationSkip
	refused  []MutationSkip
}

func planMutation(fns []ChangedFunction) mutationPlan {
	var plan mutationPlan
	for _, fn := range fns {
		switch target, verdict, reason := classifyFunction(fn); verdict {
		case verdictExcluded:
			plan.excluded = append(plan.excluded, MutationSkip{Function: fn, Reason: reason})
		case verdictRefused:
			plan.refused = append(plan.refused, MutationSkip{Function: fn, Reason: reason})
		default:
			plan.targets = append(plan.targets, target)
		}
	}
	return plan
}

type verdict int

const (
	verdictMeasure verdict = iota
	verdictExcluded
	verdictRefused
)

func classifyFunction(fn ChangedFunction) (MutationTarget, verdict, string) {
	if guard := killGuardPathOf(fn.File); guard != "" {
		return MutationTarget{}, verdictExcluded, "the path " + guard + " is a kill guard"
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fn.File, fn.Source, parser.SkipObjectResolution)
	if err != nil {
		return MutationTarget{}, verdictRefused, "the source does not parse: " + err.Error()
	}
	if callsSignal(file) {
		return MutationTarget{}, verdictExcluded, "the file holds a call that sends a signal to a process"
	}
	decl := findFunction(file, fn.Name)
	if decl == nil {
		return MutationTarget{}, verdictRefused, "the file has no function named " + fn.Name
	}
	return MutationTarget{Package: fn.Package, File: fn.File, Name: fn.Name, StartLine: fset.Position(decl.Pos()).Line, EndLine: fset.Position(decl.End()).Line}, verdictMeasure, ""
}

func killGuardPathOf(file string) string {
	slashed := filepath.ToSlash(file)
	for _, guard := range killGuardPaths {
		if strings.HasPrefix(slashed, guard) {
			return guard
		}
	}
	return ""
}

func callsSignal(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && signalCallNames[sel.Sel.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

func findFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}
