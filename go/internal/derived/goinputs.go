package derived

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	skillcheckPackage = "./internal/skillcheck"
	codeRegistration  = "RegisterCode("
	moduleDepsFormat  = "{{with .Module}}{{if .Main}}{{$.Dir}}\t{{.Dir}}{{end}}{{end}}"
)

func ResolveGoInputs(ctx context.Context, worktree string) (GoInputDirs, error) {
	registrars, err := CodeRegistrarDirs(os.DirFS(worktree))
	if err != nil {
		return nil, err
	}
	closure, err := skillcheckClosure(ctx, worktree)
	if err != nil {
		return GoInputDirs{CodeRegistrars: registrars}, err
	}
	return GoInputDirs{CodeRegistrars: registrars, SkillcheckClosure: closure}, nil
}

func CodeRegistrarDirs(fsys fs.FS) ([]string, error) {
	seen := map[string]bool{}
	err := fs.WalkDir(fsys, "go", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isProductionGoFile(p) {
			return err
		}
		raw, err := fs.ReadFile(fsys, p)
		if bytes.Contains(raw, []byte(codeRegistration)) {
			seen[path.Dir(p)] = true
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("scan the code registrars: %w", err)
	}
	return sortedKeys(seen), nil
}

func isProductionGoFile(p string) bool {
	return strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go")
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func skillcheckClosure(ctx context.Context, worktree string) ([]string, error) {
	cmd := sysexec.Command(ctx, "go", "list", "-deps", "-f", moduleDepsFormat, skillcheckPackage)
	cmd.Dir = filepath.Join(worktree, "go")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list the closure of %s: %w: %s", skillcheckPackage, err, strings.TrimSpace(stderr.String()))
	}
	return moduleDirs(string(out)), nil
}

func moduleDirs(listing string) []string {
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(listing), "\n") {
		dir, moduleDir, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		rel, _ := filepath.Rel(moduleDir, dir)
		dirs = append(dirs, path.Join("go", filepath.ToSlash(rel)))
	}
	sort.Strings(dirs)
	return dirs
}
