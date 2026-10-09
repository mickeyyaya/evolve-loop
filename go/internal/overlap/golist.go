package overlap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var tagSets = [][]string{nil, {"integration"}, {"acs"}, {"e2e", "evolve_test_phases"}}

type listedModule struct {
	Path string
	Dir  string
}

type listedPackage struct {
	ImportPath string
	Dir        string
	Module     *listedModule
	Deps       []string

	GoFiles, CgoFiles, IgnoredGoFiles                                 []string
	CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles                  []string
	SwigFiles, SwigCXXFiles, SysoFiles, IgnoredOtherFiles, EmbedFiles []string
	TestGoFiles, XTestGoFiles, TestEmbedFiles, XTestEmbedFiles        []string
}

func (p listedPackage) fileLists() [][]string {
	return [][]string{
		p.GoFiles, p.CgoFiles, p.IgnoredGoFiles,
		p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles,
		p.SwigFiles, p.SwigCXXFiles, p.SysoFiles, p.IgnoredOtherFiles, p.EmbedFiles,
		p.TestGoFiles, p.XTestGoFiles, p.TestEmbedFiles, p.XTestEmbedFiles,
	}
}

type packageSets struct {
	dir   string
	files map[string]bool
	deps  map[string]bool
}

func LoadModule(ctx context.Context, run sysexec.RunFunc, repoRoot string) (Module, error) {
	m := Module{}
	sets := map[string]*packageSets{}
	for _, tags := range tagSets {
		listed, err := goList(ctx, run, filepath.Join(repoRoot, moduleRoot), tags)
		if err != nil {
			return Module{}, err
		}
		for _, p := range listed {
			if err := mergePackage(&m, sets, p); err != nil {
				return Module{}, fmt.Errorf("go list %s: %w", tagArg(tags), err)
			}
		}
	}
	for _, ip := range sortedKeys(keySet(sets)) {
		s := sets[ip]
		m.Packages = append(m.Packages, Package{ImportPath: ip, Dir: s.dir, Files: sortedKeys(s.files), Deps: sortedKeys(s.deps)})
	}
	return m, nil
}

func goList(ctx context.Context, run sysexec.RunFunc, moduleDir string, tags []string) ([]listedPackage, error) {
	args := []string{"list", "-json"}
	if len(tags) > 0 {
		args = append(args, "-tags", tagArg(tags))
	}
	args = append(args, "./...")
	stdout, stderr, code, err := sysexec.Capture(ctx, run, moduleDir, "go", args...)
	if err != nil {
		return nil, fmt.Errorf("go list %s: %w", tagArg(tags), err)
	}
	if code != 0 {
		return nil, fmt.Errorf("go list %s: exit %d: %s", tagArg(tags), code, strings.TrimSpace(stderr))
	}
	var out []listedPackage
	dec := json.NewDecoder(strings.NewReader(stdout))
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return out, nil
		} else if err != nil {
			return nil, fmt.Errorf("go list %s: decode: %w", tagArg(tags), err)
		}
		out = append(out, p)
	}
}

func tagArg(tags []string) string {
	return strings.Join(tags, ",")
}

func mergePackage(m *Module, sets map[string]*packageSets, p listedPackage) error {
	if p.Module == nil {
		return fmt.Errorf("%s: no module", p.ImportPath)
	}
	dir, err := repoRelativeDir(p.Module.Dir, p.Dir)
	if err != nil {
		return fmt.Errorf("%s: %w", p.ImportPath, err)
	}
	m.Path = p.Module.Path
	s, ok := sets[p.ImportPath]
	if !ok {
		s = &packageSets{dir: dir, files: map[string]bool{}, deps: map[string]bool{}}
		sets[p.ImportPath] = s
	}
	for _, list := range p.fileLists() {
		for _, f := range list {
			s.files[path.Join(dir, f)] = true
		}
	}
	for _, d := range p.Deps {
		if d == m.Path || strings.HasPrefix(d, m.Path+"/") {
			s.deps[d] = true
		}
	}
	return nil
}

func repoRelativeDir(moduleDir, dir string) (string, error) {
	rel, err := filepath.Rel(moduleDir, dir)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("dir %s is outside the module %s", dir, moduleDir)
	}
	return path.Join(moduleRoot, filepath.ToSlash(rel)), nil
}

func keySet[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}
