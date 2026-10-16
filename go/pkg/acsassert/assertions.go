// Package acsassert is the assertion DSL ACS predicates write against.
package acsassert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func deepEqualOrPanic(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

type TB interface {
	Errorf(format string, args ...any)
	Helper()
}

var ErrSubprocessNotFound = errors.New("acsassert: subprocess binary not found")

func movedHint(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return " (the file may have moved or been renamed; repoint the predicate at its current location)"
	}
	return ""
}

func FileExists(tb TB, path string) bool {
	tb.Helper()
	if _, err := os.Stat(path); err != nil {
		tb.Errorf("FileExists(%q): %v%s", path, err, movedHint(err))
		return false
	}
	return true
}

func FileContains(tb TB, path, substring string) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("FileContains(%q): %v%s", path, err, movedHint(err))
		return false
	}
	if !strings.Contains(string(raw), substring) {
		tb.Errorf("FileContains(%q) missing %q", path, substring)
		return false
	}
	return true
}

func FileNotContains(tb TB, path, substring string) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("FileNotContains(%q): %v%s", path, err, movedHint(err))
		return false
	}
	if strings.Contains(string(raw), substring) {
		tb.Errorf("FileNotContains(%q) unexpectedly contains %q", path, substring)
		return false
	}
	return true
}

func FileMatchesRegex(tb TB, path, pattern string) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("FileMatchesRegex(%q): %v%s", path, err, movedHint(err))
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		tb.Errorf("FileMatchesRegex(%q) bad pattern %q: %v", path, pattern, err)
		return false
	}
	if !re.Match(raw) {
		tb.Errorf("FileMatchesRegex(%q) no match for %q", path, pattern)
		return false
	}
	return true
}

func JSONFieldEquals(tb TB, path, dotPath string, want any) bool {
	tb.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Errorf("JSONFieldEquals(%q): %v%s", path, err, movedHint(err))
		return false
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		tb.Errorf("JSONFieldEquals(%q): invalid JSON: %v", path, err)
		return false
	}
	got, ok := navigateDotPath(doc, dotPath)
	if !ok {
		tb.Errorf("JSONFieldEquals(%q): path %q not found", path, dotPath)
		return false
	}
	if !equalAny(got, want) {
		tb.Errorf("JSONFieldEquals(%q) at %q: got %v (%T), want %v (%T)", path, dotPath, got, got, want, want)
		return false
	}
	return true
}

func equalAny(a, b any) bool {
	defer func() { _ = recover() }()
	return deepEqualOrPanic(a, b)
}

func navigateDotPath(doc any, dotPath string) (any, bool) {
	cur := doc
	if dotPath == "" {
		return cur, true
	}
	for _, key := range strings.Split(dotPath, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, present := m[key]
		if !present {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func SubprocessOutput(name string, args ...string) (stdout, stderr string, code int, err error) {
	if _, lookErr := exec.LookPath(name); lookErr != nil {
		return "", "", -1, fmt.Errorf("%w: %s: %v", ErrSubprocessNotFound, name, lookErr)
	}
	cmd := sysexec.Command(context.Background(), name, args...)
	var sout, serr strings.Builder
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	runErr := cmd.Run()
	stdout = sout.String()
	stderr = serr.String()
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), fmt.Errorf("subprocess %s exited %d", name, exitErr.ExitCode())
	}
	return stdout, stderr, -1, fmt.Errorf("subprocess %s: %w", name, runErr)
}

func AllOf(tb TB, predicates ...func(TB) bool) bool {
	tb.Helper()
	for i, p := range predicates {
		if !p(tb) {
			tb.Errorf("AllOf: predicate[%d] returned false", i)
			return false
		}
	}
	return true
}

func SetupTempProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{".evolve", ".evolve/runs", "docs", "legacy/scripts/lifecycle"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("SetupTempProject mkdir %s: %v", sub, err)
		}
	}
	return dir
}

func RepoRoot(t *testing.T) string {
	t.Helper()
	stdout, _, code, err := SubprocessOutput("git", "rev-parse", "--show-toplevel")
	if err != nil || code != 0 {
		t.Skipf("not in a git work tree: code=%d err=%v", code, err)
	}
	return strings.TrimSpace(stdout)
}

func FileContainsAny(path string, variants ...string) bool {
	hit, _ := FileContainsAnyChecked(path, variants...)
	return hit
}

func FileContainsAnyChecked(path string, variants ...string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("acsassert: read %s: %w%s", path, err, movedHint(err))
	}
	s := string(raw)
	for _, v := range variants {
		if strings.Contains(s, v) {
			return true, nil
		}
	}
	return false, nil
}

func CountOccurrencesAny(path string, variants ...string) int {
	n, _ := CountOccurrencesAnyChecked(path, variants...)
	return n
}

func CountOccurrencesAnyChecked(path string, variants ...string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("acsassert: read %s: %w%s", path, err, movedHint(err))
	}
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		for _, v := range variants {
			if strings.Contains(line, v) {
				count++
				break
			}
		}
	}
	return count, nil
}

func LineContainsAll(path string, needles ...string) bool {
	hit, _ := LineContainsAllChecked(path, needles...)
	return hit
}

func LineContainsAllChecked(path string, needles ...string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("acsassert: read %s: %w%s", path, err, movedHint(err))
	}
	for _, line := range strings.Split(string(raw), "\n") {
		hit := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				hit = false
				break
			}
		}
		if hit {
			return true, nil
		}
	}
	return false, nil
}

func CountInGoFunc(path, funcName string, variants ...string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("acsassert: read %s: %w%s", path, err, movedHint(err))
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		return 0, fmt.Errorf("acsassert: parse %s: %w", path, err)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != funcName {
			continue
		}
		body := string(raw[fset.Position(fd.Pos()).Offset:fset.Position(fd.End()).Offset])
		count := 0
		for _, line := range strings.Split(body, "\n") {
			for _, v := range variants {
				if strings.Contains(line, v) {
					count++
					break
				}
			}
		}
		return count, nil
	}
	return 0, fmt.Errorf("acsassert: function %q not found in %s", funcName, path)
}
