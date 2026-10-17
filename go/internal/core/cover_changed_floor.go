package core

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var coverChangedProfiler = scopedCoverProfile

const coverChangedTestsFailedHeadline = "the tests of the changed packages fail under -tags integration, so no changed function is proven"

var coverStrictRunner = changedStrictResults

func coverChangedFailures(ctx context.Context, in ReviewInput, paths []string) []string {
	stage := coverChangedStage(in.ProjectRoot, in.errOut())
	if stage == config.StageOff || in.Worktree == "" {
		return nil
	}
	moduleDir := codequality.ModuleDir(in.Worktree)
	pkgs := coverChangedPackages(moduleDir, paths)
	if len(pkgs) == 0 {
		return nil
	}
	profile, out, status := coverChangedProfiler(ctx, moduleDir, pkgs, covchanged.DefaultTags)
	if profile != "" {
		defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()
	}
	switch status {
	case coverStatusTestsFailed:
		if unitRunRefuses(ctx, in, moduleDir, pkgs) {
			return nil
		}
		return coverChangedReport(coverChangedResult(stage, coverChangedTestsFailedHeadline, []string{floorFailureDiagnostic(out)}))
	case coverStatusPlumbingError:
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not make a coverage profile (%s); `make -C go cover-changed` and CI still judge coverage\n", out)
		return nil
	}
	failures := coverChangedFunctionFailures(ctx, in, stage, moduleDir, profile)
	return append(failures, coverStrictFailures(stage, moduleDir, profile, pkgs)...)
}

func unitRunRefuses(ctx context.Context, in ReviewInput, moduleDir string, pkgs []string) bool {
	visible := buildTagVisiblePackages(ctx, moduleDir, pkgs, in.errOut())
	return len(runBuildSelfCheck(ctx, moduleDir, visible, buildSelfCheckRunner)) > 0
}

func coverChangedFunctionFailures(ctx context.Context, in ReviewInput, stage config.Stage, moduleDir, profile string) []string {
	ws := covchanged.Workspace{ModuleDir: moduleDir, Run: gitRunner}
	v, err := ws.Check(ctx, covchanged.Options{Base: floorBase(in), Profile: profile, Tags: covchanged.DefaultTags})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not measure the change (%v); `make -C go cover-changed` and CI still judge coverage\n", err)
		return nil
	}
	headline := fmt.Sprintf("%d of %d changed function(s) are below 100%% statement coverage (go test -tags integration -coverprofile); test each one", len(v.Refused), v.Changed)
	return coverChangedReport(coverChangedResult(stage, headline, v.Problems()))
}

func coverStrictFailures(stage config.Stage, moduleDir, profile string, pkgs []string) []string {
	results, err := coverStrictRunner(moduleDir, profile, pkgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not check go/.cover-strict (%v); `make -C go cover-strict` and CI still judge it\n", err)
		return nil
	}
	var lines []string
	for _, r := range results {
		if !r.OK() {
			lines = append(lines, r.Line())
		}
	}
	return coverChangedReport(coverChangedResult(stage, "changed package(s) listed in go/.cover-strict are below their floor; test them back up to it", lines))
}

func changedStrictResults(moduleDir, profilePath string, pkgs []string) ([]coverstrict.Result, error) {
	data, err := os.ReadFile(filepath.Join(moduleDir, ".cover-strict"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	floors, err := coverstrict.ParseFloors(data)
	if err != nil {
		return nil, err
	}
	var changed []coverstrict.Floor
	for _, f := range floors {
		if slices.Contains(pkgs, "./"+f.Pkg) {
			changed = append(changed, f)
		}
	}
	if len(changed) == 0 {
		return nil, nil
	}
	module, err := covchanged.ModulePath(moduleDir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, err
	}
	profile, err := covchanged.ParseProfile(raw, module)
	if err != nil {
		return nil, err
	}
	return coverstrict.Measure(changed, profile), nil
}

func coverChangedReport(failures []string, warning string) []string {
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	return failures
}

func coverChangedResult(stage config.Stage, headline string, lines []string) (failures []string, warning string) {
	if stage == config.StageOff || len(lines) == 0 {
		return nil, ""
	}
	text := headline + ":\n    " + strings.Join(lines, "\n    ")
	if stage != config.StageEnforce {
		return nil, "[build-floor] WARN cover-changed (shadow): " + text
	}
	return []string{text}, ""
}

func coverChangedPackages(moduleDir string, paths []string) []string {
	var out []string
	for _, pkg := range changedGoTestPackages(paths) {
		rel := strings.TrimPrefix(pkg, "./")
		if slices.Contains(strings.Split(path.Clean(rel), "/"), "testdata") {
			continue
		}
		if buildsUnderCoverTags(filepath.Join(moduleDir, filepath.FromSlash(rel))) {
			out = append(out, pkg)
		}
	}
	return out
}

func buildsUnderCoverTags(dir string) bool {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return false
	}
	ctxt := build.Default
	ctxt.BuildTags = covchanged.DefaultTags
	_, err := ctxt.ImportDir(dir, 0)
	var noGo *build.NoGoError
	return !errors.As(err, &noGo)
}

func coverChangedStage(projectRoot string, warn io.Writer) config.Stage {
	return floorStage(projectRoot, "cover_changed", "cover-changed check", func(p policy.Policy) string { return p.CoverChangedConfig().Stage }, warn)
}
