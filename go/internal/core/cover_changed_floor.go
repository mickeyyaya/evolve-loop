package core

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var coverChangedProfiler = scopedCoverProfile

func coverChangedFailures(ctx context.Context, in ReviewInput, paths []string) []string {
	stage := coverChangedStage(in.ProjectRoot)
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
		return coverChangedReport(coverChangedResult(stage, "the tests of the changed packages fail under -tags integration, so no changed function is proven", []string{floorFailureDiagnostic(out)}))
	case coverStatusPlumbingError:
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not make a coverage profile (%s); `make -C go cover-changed` and CI still judge coverage\n", out)
		return nil
	}
	ws := covchanged.Workspace{ModuleDir: moduleDir, Run: gitRunner}
	v, err := ws.Check(ctx, covchanged.Options{Base: floorBase(in), Profile: profile, Tags: covchanged.DefaultTags})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not measure the change (%v); `make -C go cover-changed` and CI still judge coverage\n", err)
		return nil
	}
	headline := fmt.Sprintf("%d of %d changed function(s) are below 100%% statement coverage (go test -tags integration -coverprofile); test each one", len(v.Refused), v.Changed)
	return coverChangedReport(coverChangedResult(stage, headline, v.Problems()))
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
		if info, err := os.Stat(filepath.Join(moduleDir, filepath.FromSlash(rel))); err == nil && info.IsDir() {
			out = append(out, pkg)
		}
	}
	return out
}

func coverChangedStage(projectRoot string) config.Stage {
	return floorStage(projectRoot, "cover_changed", "cover-changed check", func(p policy.Policy) string { return p.CoverChangedConfig().Stage })
}
