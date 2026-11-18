package core

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var coverChangedProfiler = scopedCoverProfile

func packageCoverTags(pkg string) []string {
	tags := slices.Clone(covchanged.DefaultTags)
	if tag, ok := ciparity.EnvExclusiveTagOf(pkg); ok {
		tags = append(tags, tag)
	}
	return tags
}

type tagGroup struct {
	tags, pkgs []string
}

func coverTagGroups(pkgs []string) []tagGroup {
	var groups []tagGroup
	for _, pkg := range pkgs {
		tags := packageCoverTags(pkg)
		i := slices.IndexFunc(groups, func(g tagGroup) bool { return slices.Equal(g.tags, tags) })
		if i < 0 {
			groups = append(groups, tagGroup{tags: tags})
			i = len(groups) - 1
		}
		groups[i].pkgs = append(groups[i].pkgs, pkg)
	}
	return groups
}

func unionTags(groups []tagGroup) []string {
	var tags []string
	for _, g := range groups {
		for _, tag := range g.tags {
			if !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func coverChangedTestsFailedHeadline(tags []string) string {
	return "the tests of the changed packages fail under -tags " + strings.Join(tags, ",") + ", so no changed function is proven"
}

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
	noteCoverEnvExclusive(pkgs, in.errOut())
	profile, out, status, failedTags := coverChangedProfiler(ctx, in, moduleDir, pkgs, covchanged.DefaultTags)
	if profile != "" {
		defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()
	}
	switch status {
	case coverStatusTestsFailed:
		lines := in.coverRedLines(ctx, out, covchanged.TagFlags(failedTags))
		return coverChangedReport(coverChangedResult(stage, coverChangedTestsFailedHeadline(failedTags), lines))
	case coverStatusPlumbingError:
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not make a coverage profile (%s); `make -C go cover-changed` and CI still judge coverage\n", out)
		return coverChangedReport(coverChangedResult(stage, coverChangedTestsFailedHeadline(covchanged.DefaultTags), unitRunFailures(ctx, in, moduleDir, withoutEnvExclusive(pkgs, in.errOut()))))
	}
	failures := coverChangedFunctionFailures(ctx, in, coverMeasure{stage: stage, moduleDir: moduleDir, profile: profile, pkgs: pkgs})
	floors := baseAndTreeList(ctx, in, moduleDir, ".cover-strict")
	return append(failures, coverStrictFailures(stage, moduleDir, profile, pkgs, floors, newBlockNoter(in.Worktree, moduleDir, in.Protected))...)
}

func unitRunFailures(ctx context.Context, in ReviewInput, moduleDir string, pkgs []string) []string {
	visible := buildTagVisiblePackages(ctx, moduleDir, pkgs, in.errOut())
	fails := in.retakeSelfCheckFailures(runBuildSelfCheck(ctx, moduleDir, visible, buildSelfCheckRunner), func(pkg string) (string, bool) {
		return buildSelfCheckRunner(ctx, moduleDir, pkg)
	})
	var details []string
	for _, f := range in.laneFailures(ctx, fails) {
		details = append(details, floorFailureDiagnostic(f.Output))
	}
	return details
}

type coverMeasure struct {
	stage              config.Stage
	moduleDir, profile string
	pkgs               []string
}

func coverPackageSet(pkgs []string) covchanged.PackageSet {
	set := covchanged.PackageSet{}
	for _, p := range pkgs {
		set[strings.TrimPrefix(p, "./")] = true
	}
	return set
}

func coverChangedFunctionFailures(ctx context.Context, in ReviewInput, m coverMeasure) []string {
	stage, moduleDir := m.stage, m.moduleDir
	ws := covchanged.Workspace{ModuleDir: moduleDir, Run: gitRunner}
	groups := coverTagGroups(m.pkgs)
	v := covchanged.Verdict{Refused: []covchanged.Func{}}
	for _, g := range groups {
		gv, err := ws.Check(ctx, covchanged.Options{Base: floorBase(in), Profile: m.profile, Tags: g.tags, Packages: coverPackageSet(g.pkgs)})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not measure the change (%v); `make -C go cover-changed` and CI still judge coverage\n", err)
			return nil
		}
		v.Changed += gv.Changed
		v.Refused = append(v.Refused, gv.Refused...)
	}
	prefix := moduleRepoPrefix(in.Worktree, moduleDir)
	base := baseNames{read: gitBaseReader(ctx, in, prefix), warn: in.errOut()}
	refused, unmeasured := splitNotMeasured(v.Refused, base.has)
	headline := fmt.Sprintf("%d of %d changed function(s) are below 100%% statement coverage (go test -tags %s -coverprofile); test each one", len(refused), v.Changed, strings.Join(unionTags(groups), ","))
	failures := coverChangedReport(coverChangedResult(stage, headline, covchanged.Verdict{Refused: refused}.Problems()))
	in.residuals.add(coverResiduals(failures, refused, prefix)...)
	in.noteNotMeasured(unmeasured, prefix)
	return failures
}

const coverStrictHeadline = "changed package(s) listed in go/.cover-strict are below their floor; test each uncovered block below, unless its note says a test cannot reach it"

func coverStrictFailures(stage config.Stage, moduleDir, profile string, pkgs []string, floors []byte, noter blockNoter) []string {
	results, err := coverStrictRunner(moduleDir, profile, pkgs, floors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN cover-changed could not check go/.cover-strict (%v); `make -C go cover-strict` and CI still judge it\n", err)
		return nil
	}
	var lines []string
	for _, r := range results {
		if !r.OK() {
			lines = append(lines, r.Line())
			lines = append(lines, noter.notes(profile, r.Pkg)...)
		}
	}
	return coverChangedReport(coverChangedResult(stage, coverStrictHeadline, lines))
}

func changedStrictResults(moduleDir, profilePath string, pkgs []string, data []byte) ([]coverstrict.Result, error) {
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
	scoped, exclusive, _ := ciparity.TierScope(changedGoTestPackages(paths))
	pkgs := append(scoped, exclusive...)
	slices.Sort(pkgs)
	for _, pkg := range pkgs {
		rel := strings.TrimPrefix(pkg, "./")
		if slices.Contains(strings.Split(path.Clean(rel), "/"), "testdata") {
			continue
		}
		if buildsUnderCoverTags(filepath.Join(moduleDir, filepath.FromSlash(rel)), packageCoverTags(pkg)) {
			out = append(out, pkg)
		}
	}
	return out
}

func buildsUnderCoverTags(dir string, tags []string) bool {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return false
	}
	ctxt := build.Default
	ctxt.BuildTags = tags
	_, err := ctxt.ImportDir(dir, 0)
	var noGo *build.NoGoError
	return !errors.As(err, &noGo)
}

func coverChangedStage(projectRoot string, warn io.Writer) config.Stage {
	return floorStage(projectRoot, "cover_changed", "cover-changed check", func(p policy.Policy) string { return p.CoverChangedConfig().Stage }, warn)
}
