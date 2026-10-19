package evidence

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

var ErrUnresolved = errors.New("evidence does not resolve")

type Source struct {
	Root     string
	Cycle    int
	Commands []string
}

var testPrefixes = []string{"Test", "Fuzz", "Benchmark"}

func Resolve(src Source, ev handoff.Evidence) error {
	var ok bool
	switch ev.Kind {
	case "path":
		ok = pathResolves(src.Root, ev.Ref)
	case "test":
		ok = testResolves(filepath.Join(src.Root, "go"), ev.Ref)
	case "predicate":
		ok = predicateResolves(src, ev.Ref)
	case "command":
		ok = ev.Ref != "" && slices.Contains(src.Commands, ev.Ref)
	}
	if !ok {
		return fmt.Errorf("%w: %s %q", ErrUnresolved, ev.Kind, ev.Ref)
	}
	return nil
}

func pathResolves(root, ref string) bool {
	rel, lineText, found := cutLast(ref)
	if !found {
		return false
	}
	line, err := strconv.Atoi(lineText)
	if err != nil || line < 1 {
		return false
	}
	body, ok := readLocal(root, rel)
	return ok && line <= lineCount(body)
}

func lineCount(body []byte) int {
	n := strings.Count(string(body), "\n")
	if len(body) > 0 && body[len(body)-1] != '\n' {
		n++
	}
	return n
}

func testResolves(goDir, ref string) bool {
	file, name := splitRef(ref)
	if !isTestName(name) {
		return false
	}
	if file != "" {
		return fileDeclares(filepath.Dir(goDir), file, name)
	}
	found := false
	_ = filepath.WalkDir(goDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, "_test.go") && declares(path, name) {
			found = true
		}
		return nil
	})
	return found
}

func predicateResolves(src Source, ref string) bool {
	file, name := splitRef(ref)
	if !isTestName(name) {
		return false
	}
	pkgRel := "go/acs/cycle" + strconv.Itoa(src.Cycle)
	if file != "" {
		return filepath.ToSlash(filepath.Dir(filepath.FromSlash(file))) == pkgRel &&
			fileDeclares(src.Root, file, name)
	}
	entries, err := os.ReadDir(filepath.Join(src.Root, filepath.FromSlash(pkgRel)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") &&
			declares(filepath.Join(src.Root, filepath.FromSlash(pkgRel), entry.Name()), name) {
			return true
		}
	}
	return false
}

func fileDeclares(root, rel, name string) bool {
	if !strings.HasSuffix(rel, "_test.go") || !filepath.IsLocal(filepath.FromSlash(rel)) {
		return false
	}
	return declares(filepath.Join(root, filepath.FromSlash(rel)), name)
}

func declares(path, name string) bool {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return false
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return true
		}
	}
	return false
}

func isTestName(name string) bool {
	for _, prefix := range testPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func readLocal(root, rel string) ([]byte, bool) {
	if rel == "" || !filepath.IsLocal(filepath.FromSlash(rel)) {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	return body, err == nil
}

func splitRef(ref string) (file, name string) {
	if head, tail, found := cutLast(ref); found {
		return head, tail
	}
	return "", ref
}

func cutLast(ref string) (head, tail string, found bool) {
	i := strings.LastIndex(ref, ":")
	if i < 0 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}
