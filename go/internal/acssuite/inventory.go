package acssuite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// executeCompleteGoScope discovers the compiler-selected test sources before
// execution. TestMain cannot hide a declared predicate by filtering m.Run or
// exiting early, and GOOS/build-tag exclusions remain the Go tool's decision.
func executeCompleteGoScope(ctx context.Context, moduleDir, pattern string, env []string) (string, error) {
	expected, err := declaredPredicateTests(ctx, moduleDir, pattern, env)
	if err != nil {
		return "", err
	}
	raw, execErr := defaultGoExec(ctx, moduleDir, pattern, env)
	for _, line := range strings.Split(raw, "\n") {
		var ev goEvent
		if json.Unmarshal([]byte(line), &ev) == nil && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip") {
			delete(expected, ev.Package+"/"+ev.Test)
		}
	}
	if len(expected) != 0 {
		missing := make([]string, 0, len(expected))
		for key := range expected {
			missing = append(missing, key)
		}
		slices.Sort(missing)
		return raw, fmt.Errorf("predicate execution omitted declared tests %s: %w", strings.Join(missing, ", "), errors.Join(execErr, errIncompleteInventory))
	}
	return raw, execErr
}

var errIncompleteInventory = errors.New("incomplete predicate inventory")

func declaredPredicateTests(ctx context.Context, moduleDir, pattern string, env []string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "-tags", "acs", pattern)
	cmd.Dir, cmd.Env, cmd.WaitDelay = moduleDir, env, 30*time.Second
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("discover compiled predicate inventory %s: %w", pattern, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	expected := map[string]bool{}
	for {
		var pkg struct {
			Dir, ImportPath           string
			TestGoFiles, XTestGoFiles []string
		}
		if err := dec.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode predicate inventory: %w", err)
		}
		for _, name := range append(pkg.TestGoFiles, pkg.XTestGoFiles...) {
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(pkg.Dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, fmt.Errorf("parse predicate inventory: %w", err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" || !strings.HasPrefix(fn.Name.Name, "Test") {
					continue
				}
				suffix := strings.TrimPrefix(fn.Name.Name, "Test")
				if first, _ := utf8.DecodeRuneInString(suffix); suffix != "" && unicode.IsLower(first) {
					continue
				}
				expected[pkg.ImportPath+"/"+fn.Name.Name] = true
			}
		}
	}
	if len(expected) == 0 {
		return nil, fmt.Errorf("predicate scope %s has no declared tests", pattern)
	}
	return expected, nil
}
