//go:build acs

// Package norebuild is the one-binary S3 durable guard: it fails the per-cycle
// audit (and the acs-durable CI tier) if any NEW runtime executable-build site
// appears in non-test go/internal, go/cmd, or go/pkg code.
//
// Why this guard exists: the one-binary goal is EXACTLY ONE first-party
// executable (evolve), never rebuilt at runtime in deployed (target-repo) mode,
// so running the loop on a locked-down machine needs a single security approval
// per adopted release. Before S1 there were two runtime `go build -o <exe>`
// sites: ciparity.go's apicover rebuild (fired mid-audit whenever a cycle
// touched an enforced package) and releasepipeline.go's release rebuild. S1
// folded apicover into the evolve binary, deleting the first. This predicate
// pins that win: it scans every non-test .go file under go/internal, go/cmd, and
// go/pkg for a `go build -o` executable-build invocation and RED-fails on any
// site outside the allowlist — so a future cycle cannot silently reintroduce a
// second runtime-built executable.
//
// The ONLY allowlisted site is internal/releasepipeline: the operator-invoked
// `evolve release` rebuild of the tracked go/evolve binary, which never fires in
// a target-repo cycle (self-development releases only).
//
// Scope: this forbids BUILDING a first-party executable at runtime, not the mere
// existence of other main packages in source — cmd/apicover (an S1 CI shim),
// cmd/testlatency, cmd/filter-stdout, etc. remain buildable but are never built
// during a deployed cycle, so they are not a violation.
//
// Detection is AST-based (comment-safe): a comment that merely MENTIONS
// `go build -o` (e.g. ciparity.go's "no runtime `go build -o bin/apicover`")
// is not a build site and must not trip the gate — only string-literal command
// args in a real call do.
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

// allowedBuildDirs are the go/-relative directory prefixes permitted to hold a
// runtime `go build -o <exe>` site. Only the operator-invoked release rebuild.
var allowedBuildDirs = []string{
	"internal/releasepipeline",
}

// buildSite is one detected `go build -o <executable>` invocation.
type buildSite struct {
	RelPath string // path relative to go/
	Line    int
}

// TestNoNewRuntimeExecutableBuildSites is the durable guard: every executable
// build site under go/internal, go/cmd, or go/pkg must be in allowedBuildDirs. A
// newly added `go build -o` (or `exec.Command("go","build",…,"-o",…)`) in
// deployed runtime code fails HERE, at ship-gate/CI time.
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

// wantAllowlistedSites is the exact number of executable-build sites the
// detector finds inside allowedBuildDirs today: the single release rebuild in
// internal/releasepipeline (releasepipeline.go's `go build -o evolve`). Pinning
// the COUNT (not just "≥1") means a second, unrelated build site added anywhere
// under an allowlisted dir fails the guard instead of being silently exempted.
// Bump this ONLY with a deliberate, reviewed reason.
const wantAllowlistedSites = 1

// TestReleasepipelineStillHasTheAllowlistedSite is the anti-vacuity + no-hidden-
// site check: the scanner must find EXACTLY the known allowlisted site(s). Zero
// means the detector silently broke (the guard would pass vacuously against any
// regression); more than wantAllowlistedSites means a new build site is hiding
// under the whole-directory allowlist.
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

// TestDetector_MutationProof proves the detector actually bites: it must flag a
// synthetic exec.Command("go","build",…,"-o",…) call AND a `go build -o` shell
// string, while NOT flagging a comment that merely mentions `go build -o` (the
// ciparity.go:258 false-positive class). This is the guard's own red test.
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
			// The exact evasion route: build args hidden in a []string{…} literal
			// spread via append — the idiom ciparity.go already uses for `go list`.
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

// fileHasBuildSite parses src and reports whether the detector finds an
// executable-build site in it — the unit seam TestDetector_MutationProof drives.
func fileHasBuildSite(t *testing.T, src string) bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return len(sitesInFile(fset, f)) > 0
}

// findExecutableBuildSites walks non-test .go files under go/internal, go/cmd,
// and go/pkg and returns every `go build -o <exe>` site (AST-detected,
// comment-safe).
func findExecutableBuildSites(goDir string) ([]buildSite, error) {
	fset := token.NewFileSet()
	var out []buildSite
	// internal + cmd + pkg are the deployed runtime surface (pkg hosts
	// subprocess wrappers like phaseproto/naminguard that runtime code imports),
	// so a build site in any of them must be caught.
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

// sitesInFile returns the line numbers of executable-build sites in f. Three
// signals, all operating on AST nodes (never comments): (1) a call whose direct
// string-literal args include both "build" and "-o" — exec.Command("go","build",
// …,"-o",…) or a sysexec-style run(…,"go","build","-o",…); (2) a []string{…}
// composite literal whose elements include both "build" and "-o" — the
// append([]string{"build","-o",…}, …)… idiom this repo already uses for `go
// list` args (ciparity.go), which would otherwise hide the args from signal 1;
// (3) any string literal containing the substring "go build -o" — a shell form.
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

// exprsHaveBuildDashO reports whether a list of expressions (a call's args or a
// composite literal's elements) includes both the string literals "build" and
// "-o" — the signature of a `go build -o` executable build regardless of the
// callee (exec.Command, a sysexec wrapper) or of the args being wrapped in a
// []string{…} literal.
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

// isAllowed reports whether a go/-relative path is under an allowlisted dir.
func isAllowed(relPath string) bool {
	for _, dir := range allowedBuildDirs {
		if relPath == dir || strings.HasPrefix(relPath, dir+"/") {
			return true
		}
	}
	return false
}
