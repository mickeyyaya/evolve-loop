//go:build acs

package norebuild

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var allowedBuildDirs = []string{
	"internal/releasepipeline",
}

type buildSite struct {
	RelPath string
	Line    int
}

func TestNoNewRuntimeExecutableBuildSites(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	sites, err := findExecutableBuildSites(goDir)
	if err != nil {
		t.Fatalf("scan for build sites: %v", err)
	}

	for _, s := range sites {
		if !isAllowed(s.RelPath) {
			t.Errorf("runtime executable-build site outside allowlist: %s:%d — "+
				"the one-binary invariant forbids a second first-party executable rebuilt "+
				"at runtime (fold it into the evolve binary, cf. apicover S1). If this is a "+
				"legitimate operator-only release path, add its dir to allowedBuildDirs.", s.RelPath, s.Line)
		}
	}
}

const wantAllowlistedSites = 1

func TestReleasepipelineStillHasTheAllowlistedSite(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	sites, err := findExecutableBuildSites(goDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	var allowed []buildSite
	for _, s := range sites {
		if isAllowed(s.RelPath) {
			allowed = append(allowed, s)
		}
	}
	if len(allowed) != wantAllowlistedSites {
		t.Fatalf("found %d build sites in allowlisted dirs %v, want exactly %d: %+v — "+
			"if the detector found zero it silently broke (guard passes vacuously); if it "+
			"found more, a new build site is hiding under a whole-dir allowlist (site-pin it "+
			"or bump wantAllowlistedSites deliberately).", len(allowed), allowedBuildDirs, wantAllowlistedSites, allowed)
	}
}

func TestDetector_MutationProof(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "exec.Command go build -o",
			src:  "package p\nimport \"os/exec\"\nfunc f() { _ = exec.Command(\"go\", \"build\", \"-o\", \"bin/x\", \"./cmd/x\") }\n",
			want: true,
		},
		{
			name: "sysexec-style go build -o args",
			src:  "package p\nfunc run(a ...string) {}\nfunc f() { run(\"go\", \"build\", \"-o\", \"bin/x\") }\n",
			want: true,
		},
		{
			name: "append([]string{...}) build -o idiom",
			src:  "package p\nfunc run(a ...string) {}\nfunc f() { run(\"go\", append([]string{\"build\", \"-o\", \"bin/x\"}, \"./cmd/x\")...) }\n",
			want: true,
		},
		{
			name: "shell string go build -o",
			src:  "package p\nfunc f() { s := \"go build -o bin/x ./cmd/x\"; _ = s }\n",
			want: true,
		},
		{
			name: "comment mentioning go build -o is NOT a site",
			src:  "package p\n// deleted the runtime `go build -o bin/apicover` in S1\nfunc f() {}\n",
			want: false,
		},
		{
			name: "go build without -o (compile check) is NOT an executable-build site",
			src:  "package p\nimport \"os/exec\"\nfunc f() { _ = exec.Command(\"go\", \"build\", \"./...\") }\n",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fileHasBuildSite(t, tc.src)
			if got != tc.want {
				t.Errorf("fileHasBuildSite = %v, want %v for:\n%s", got, tc.want, tc.src)
			}
		})
	}
}

func fileHasBuildSite(t *testing.T, src string) bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return len(sitesInFile(fset, f)) > 0
}

func findExecutableBuildSites(goDir string) ([]buildSite, error) {
	fset := token.NewFileSet()
	var out []buildSite
	for _, sub := range []string{"internal", "cmd", "pkg"} {
		root := filepath.Join(goDir, sub)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if perr != nil {
				return perr
			}
			rel, rerr := filepath.Rel(goDir, path)
			if rerr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)
			for _, s := range sitesInFile(fset, f) {
				out = append(out, buildSite{RelPath: rel, Line: s})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func sitesInFile(fset *token.FileSet, f *ast.File) []int {
	var lines []int
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if exprsHaveBuildDashO(node.Args) {
				lines = append(lines, fset.Position(node.Pos()).Line)
			}
		case *ast.CompositeLit:
			if exprsHaveBuildDashO(node.Elts) {
				lines = append(lines, fset.Position(node.Pos()).Line)
			}
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				if v, err := strconv.Unquote(node.Value); err == nil && strings.Contains(v, "go build -o") {
					lines = append(lines, fset.Position(node.Pos()).Line)
				}
			}
		}
		return true
	})
	return lines
}

func exprsHaveBuildDashO(exprs []ast.Expr) bool {
	var hasBuild, hasDashO bool
	for _, a := range exprs {
		lit, ok := a.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			continue
		}
		switch v {
		case "build":
			hasBuild = true
		case "-o":
			hasDashO = true
		}
	}
	return hasBuild && hasDashO
}

func isAllowed(relPath string) bool {
	for _, dir := range allowedBuildDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+"/") {
			return true
		}
	}
	return false
}
