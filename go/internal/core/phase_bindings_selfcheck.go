package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type selfCheckFailure struct {
	Pkg    string `json:"pkg"`
	Output string `json:"output"`
}

var buildSelfCheckRunner = realGoUnitTest

var buildSelfCheckTaggedRunner = realGoUnitTestTagged

func changedWorktreePathsSince(ctx context.Context, worktree, baseSHA string, diffFlags ...string) []string {
	var out []string
	if diff, code, err := gitCapture(ctx, worktree, append([]string{"diff", baseSHA, "--name-only", "-z"}, diffFlags...)...); err == nil && code == 0 {
		out = append(out, nulSeparated(diff)...)
	}
	if oth, code, err := gitCapture(ctx, worktree, "ls-files", "--others", "--exclude-standard", "-z"); err == nil && code == 0 {
		out = append(out, nulSeparated(oth)...)
	}
	return out
}

func nulSeparated(out string) []string {
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func changedGoTestPackages(paths []string) []string {
	seen := map[string]bool{}
	var pkgs []string
	for _, p := range paths {
		if !strings.HasPrefix(p, "go/") || !strings.HasSuffix(p, ".go") {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(path.Dir(p), "go"), "/")
		pkg := "./" + rel
		if !seen[pkg] {
			seen[pkg] = true
			pkgs = append(pkgs, pkg)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

func runBuildSelfCheck(ctx context.Context, moduleDir string, pkgs []string, run func(context.Context, string, string) (string, bool)) []selfCheckFailure {
	var fails []selfCheckFailure
	for _, pkg := range pkgs {
		if out, ok := run(ctx, moduleDir, pkg); !ok {
			fails = append(fails, selfCheckFailure{Pkg: pkg, Output: out})
		}
	}
	return fails
}

func unitTestArgs(pkg string) []string {
	return []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, pkg}
}

func taggedTestArgs(pkg string, tags []string) []string {
	return []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout, "-tags", strings.Join(tags, ","), pkg}
}

func realGoUnitTestTagged(ctx context.Context, moduleDir, pkg string, tags []string) (output string, passed bool) {
	cmd := sysexec.Command(ctx, "go", taggedTestArgs(pkg, tags)...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func realGoUnitTest(ctx context.Context, moduleDir, pkg string) (output string, passed bool) {
	if _, err := os.Stat(filepath.Join(moduleDir, pkg)); errors.Is(err, fs.ErrNotExist) {
		return fmt.Sprintf("%s: package directory removed — nothing to unit-test\n", pkg), true
	}
	cmd := sysexec.Command(ctx, "go", unitTestArgs(pkg)...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	out, err := cmd.CombinedOutput()
	s := string(out)
	return s, err == nil || goTestExcludedByBuildTags(s)
}

func goTestExcludedByBuildTags(output string) bool {
	return strings.Contains(output, "build constraints exclude all Go files") &&
		!strings.Contains(output, "imports ")
}

func (o *Orchestrator) buildSelfCheck(ctx context.Context, worktree string) {
	if worktree == "" {
		return
	}
	removeBuildSelfCheckArtifact(worktree)
	pkgs := changedGoTestPackages(changedWorktreePaths(ctx, worktree))
	if len(pkgs) == 0 {
		return
	}
	fails := runBuildSelfCheck(ctx, codequality.ModuleDir(worktree), pkgs, buildSelfCheckRunner)
	if len(fails) == 0 {
		return
	}
	names := make([]string, len(fails))
	for i, f := range fails {
		names[i] = f.Pkg
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-selfcheck: %d changed package(s) FAIL unit tests before audit — the build self-report is not green; audit is the backstop: %s\n", len(fails), strings.Join(names, ", "))
	writeBuildSelfCheckArtifact(worktree, fails)
}

func removeBuildSelfCheckArtifact(worktree string) {
	_ = os.Remove(filepath.Join(worktree, ".evolve", "build-selfcheck.json"))
}

func writeBuildSelfCheckArtifact(worktree string, fails []selfCheckFailure) {
	dir := filepath.Join(worktree, ".evolve")
	dst := filepath.Join(dir, "build-selfcheck.json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[selfcheck] WARN could not persist build-selfcheck artifact %s: %v\n", dst, err)
		return
	}
	data, err := json.MarshalIndent(fails, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[selfcheck] WARN could not encode build-selfcheck artifact %s: %v\n", dst, err)
		return
	}
	if err := os.WriteFile(dst, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[selfcheck] WARN could not write build-selfcheck artifact %s: %v\n", dst, err)
	}
}
