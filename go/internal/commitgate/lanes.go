package commitgate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

var nodeExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

func lookPathDefault(tool string) (string, error) { return exec.LookPath(tool) }

func (o Options) fileExists(rel string) bool {
	_, err := os.Stat(filepath.Join(o.RepoRoot, rel))
	return err == nil
}

func (o Options) existingFilesWithExt(files []string, ext string) []string {
	var out []string
	for _, f := range filesWithExt(files, ext) {
		if o.fileExists(f) {
			out = append(out, f)
		}
	}
	return out
}

func (o Options) findUp(rel, marker string) string {
	d := filepath.Dir(filepath.Join(o.RepoRoot, rel))
	for d != "" && d != string(filepath.Separator) {
		if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

func (o Options) laneGo(ctx context.Context, files []string, res *Result) int {
	gofiles := o.existingFilesWithExt(files, "go")
	if len(gofiles) == 0 {
		return ExitPass
	}
	if code := o.ensureTool("go", "", "install Go from https://go.dev/dl", res); code != ExitPass {
		return code
	}

	if code := o.checkGofmt(ctx, gofiles, res); code != ExitPass {
		return code
	}

	type pkgKey struct{ mod, pkg string }
	keySet := map[pkgKey]bool{}
	for _, f := range gofiles {
		mod := o.findUp(f, "go.mod")
		if mod == "" {
			res.log("go: no go.mod above %s", f)
			return ExitFail
		}
		fdir := filepath.Dir(filepath.Join(o.RepoRoot, f))
		rel, err := filepath.Rel(mod, fdir)
		if err != nil {
			res.log("go: cannot relativize %s", f)
			return ExitFail
		}
		relPkg := "./" + rel
		if rel == "." {
			relPkg = "./."
		}
		isACSPredicatePackage := strings.HasPrefix(relPkg, "./acs/")
		if isACSPredicatePackage {
			continue
		}
		keySet[pkgKey{mod, relPkg}] = true
	}

	byMod := map[string][]string{}
	for k := range keySet {
		byMod[k.mod] = append(byMod[k.mod], k.pkg)
	}
	mods := make([]string, 0, len(byMod))
	for m := range byMod {
		mods = append(mods, m)
	}
	sort.Strings(mods)

	glc := false
	for _, mod := range mods {
		pkgs := byMod[mod]
		sort.Strings(pkgs)
		if out, ok := o.runCmd(ctx, mod, "go", append([]string{"vet"}, pkgs...)...); !ok {
			res.log("go vet failed in %s\n%s", mod, out)
			return ExitFail
		}
		if o.have("golangci-lint") {
			if out, ok := o.runCmd(ctx, mod, "golangci-lint", append([]string{"run"}, pkgs...)...); !ok {
				res.log("golangci-lint failed\n%s", out)
				return ExitFail
			}
			glc = true
		}
		if out, ok := o.runCmd(ctx, mod, "go", append([]string{"test"}, pkgs...)...); !ok {
			res.log("go test failed in %s\n%s", mod, out)
			return ExitFail
		}
	}
	res.pass("go:vet")
	if glc {
		res.pass("go:golangci-lint")
	}
	res.pass("go:test")
	return ExitPass
}

func (o Options) checkGofmt(ctx context.Context, gofiles []string, res *Result) int {
	var unformatted []string
	for _, f := range gofiles {
		out, ok := o.runCmd(ctx, o.RepoRoot, "gofmt", "-s", "-l", filepath.Join(o.RepoRoot, f))
		if !ok {
			res.log("go: gofmt -s -l could not check %s\n%s", f, out)
			return ExitFail
		}
		if strings.TrimSpace(out) != "" {
			unformatted = append(unformatted, f)
		}
	}
	if len(unformatted) > 0 {
		res.log("go: gofmt -s needs: %s", strings.Join(unformatted, " "))
		return ExitFail
	}
	res.pass("go:gofmt")
	return ExitPass
}

func (o Options) lanePython(ctx context.Context, files []string, res *Result) int {
	pyfiles := o.existingFilesWithExt(files, "py")
	if len(pyfiles) == 0 {
		return ExitPass
	}
	if code := o.ensureTool("ruff", "python3 -m pip install --user ruff", "pip install ruff", res); code != ExitPass {
		return code
	}
	args := make([]string, 0, len(pyfiles)+1)
	args = append(args, "check")
	for _, f := range pyfiles {
		args = append(args, filepath.Join(o.RepoRoot, f))
	}
	if out, ok := o.runCmd(ctx, o.RepoRoot, "ruff", args...); !ok {
		res.log("ruff failed\n%s", out)
		return ExitFail
	}
	res.pass("python:ruff")

	var tests []string
	for _, f := range pyfiles {
		if isPyTest(f) {
			tests = append(tests, filepath.Join(o.RepoRoot, f))
		}
	}
	if len(tests) > 0 {
		if code := o.ensureTool("pytest", "python3 -m pip install --user pytest", "pip install pytest", res); code != ExitPass {
			return code
		}
		if out, ok := o.runCmd(ctx, o.RepoRoot, "pytest", append([]string{"-q"}, tests...)...); !ok {
			res.log("pytest failed\n%s", out)
			return ExitFail
		}
		res.pass("python:pytest")
	}
	return ExitPass
}

func isPyTest(path string) bool {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	if !strings.HasSuffix(base, ".py") {
		return false
	}
	stem := strings.TrimSuffix(base, ".py")
	return strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test")
}

func (o Options) laneNode(ctx context.Context, files []string, res *Result) int {
	var nfiles []string
	for _, f := range files {
		if slices.ContainsFunc(nodeExts, func(ext string) bool { return strings.HasSuffix(f, ext) }) && o.fileExists(f) {
			nfiles = append(nfiles, f)
		}
	}
	if len(nfiles) == 0 {
		return ExitPass
	}
	var name string
	var prefix []string
	switch {
	case o.have("eslint"):
		name = "eslint"
	case o.have("npx"):
		name, prefix = "npx", []string{"eslint"}
	default:
		res.log("eslint/npx absent. Install manually: npm install")
		return ExitToolMissing
	}
	args := append(append([]string{}, prefix...), nfiles...)
	if out, ok := o.runCmd(ctx, o.RepoRoot, name, args...); !ok {
		res.log("eslint failed\n%s", out)
		return ExitFail
	}
	res.pass("node:eslint")
	return ExitPass
}

func (o Options) laneRust(ctx context.Context, files []string, res *Result) int {
	rfiles := o.existingFilesWithExt(files, "rs")
	if len(rfiles) == 0 {
		return ExitPass
	}
	if code := o.ensureTool("cargo", "", "install Rust via https://rustup.rs", res); code != ExitPass {
		return code
	}
	seen := map[string]bool{}
	var crates []string
	for _, f := range rfiles {
		md := o.findUp(f, "Cargo.toml")
		if md == "" || seen[md] {
			continue
		}
		seen[md] = true
		crates = append(crates, md)
	}
	sort.Strings(crates)
	for _, md := range crates {
		if out, ok := o.runCmd(ctx, md, "cargo", "fmt", "--check"); !ok {
			res.log("cargo checks failed in %s\n%s", md, out)
			return ExitFail
		}
		if out, ok := o.runCmd(ctx, md, "cargo", "clippy", "--", "-D", "warnings"); !ok {
			res.log("cargo checks failed in %s\n%s", md, out)
			return ExitFail
		}
		if out, ok := o.runCmd(ctx, md, "cargo", "test"); !ok {
			res.log("cargo checks failed in %s\n%s", md, out)
			return ExitFail
		}
	}
	if len(crates) > 0 {
		res.pass("rust:fmt")
		res.pass("rust:clippy")
		res.pass("rust:test")
	}
	return ExitPass
}
