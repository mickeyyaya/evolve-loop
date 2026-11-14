package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/coversplit"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

type BuildFloorCheckFn func(ctx context.Context, in ReviewInput) []string

func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {
	in = in.withFloorAdded()
	out := RemovalClaimFailures(ctx, in)
	out = append(out, PlaceholderTokenFailures(ctx, in)...)
	paths := changedFloorPaths(ctx, in)
	out = append(out, personaBudgetFailures(ctx, in.Worktree, paths)...)
	out = append(out, docsFloorChecks(ctx, in, paths)...)
	out = append(out, commentFloorFailures(ctx, in)...)
	out = append(out, sizeFloorFailures(ctx, in)...)
	out = append(out, coverChangedFailures(ctx, in, paths)...)
	out = append(out, floorListRegressions(ctx, in)...)
	return append(out, changedPackageFloorChecks(ctx, in, paths)...)
}

func ProtectedSurfaceFloorChecks(member func(string) bool) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" {
			return nil
		}
		base := floorBase(in)
		return protectedSurfaceFailures(changedWorktreePathsSince(ctx, in.Worktree, base, "--no-renames"), member, base)
	}
}

func protectedSurfaceFailures(paths []string, member func(string) bool, base string) []string {
	if member == nil {
		return nil
	}
	var out []string
	for _, p := range paths {
		if member(p) {
			out = append(out, fmt.Sprintf("protected control-plane path changed: %s — a cycle may not edit the gate/metric/guard/contract that grades it (ADR-0064; ship refuses this diff). Restore it: `git checkout %s -- %s` (or delete it if it is new), then reshape the change so it does not need the path. If the task cannot be completed without changing it, still restore it and record that in build-report.md — the item is console work.", p, base, p))
		}
	}
	return out
}

func explanationDocumentationFailures(ctx context.Context, in ReviewInput) []string {
	if in.ExplanationDocumentationVersion == 0 {
		return nil
	}
	return explanationdocs.CheckBuild(ctx, explanationdocs.CycleBinding{
		ProjectRoot:     in.ProjectRoot,
		Worktree:        in.Worktree,
		Workspace:       in.Workspace,
		BaseSHA:         in.WorktreeBaseSHA,
		Cycle:           in.Cycle,
		RunID:           in.RunID,
		ContractVersion: in.ExplanationDocumentationVersion,
		OnOmission:      in.residuals.noteOmission,
	})
}

func docsFloorChecks(ctx context.Context, in ReviewInput, paths []string) []string {
	var cfg docsfloor.Config
	steStage := policy.Policy{}.DocsFloorConfig().SteStage
	if in.ProjectRoot != "" {
		floors := policy.LoadOrDefaults(filepath.Join(in.ProjectRoot, ".evolve", "policy.json"), in.errOut()).DocsFloorConfig()
		cfg.Stage, steStage = floors.Stage, floors.SteStage
	}
	failures := steLintFloor(ctx, in.Worktree, floorBase(in), steStage, paths, in.errOut())
	v := docsfloor.Evaluate(cfg, docsfloor.Input{
		ArchitectureLabeled: docsfloor.IsArchitectureClass(paths),
		ChangedFiles:        paths,
	})
	if v.Status == docsfloor.StatusWarn {
		fmt.Fprintf(in.errOut(), "[docs-floor] WARN: %s\n", v.Reason)
	}
	return failures
}

func changedFloorPaths(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	return ChangedWorktreePathsSinceBase(ctx, in.Worktree, in.WorktreeBaseSHA)
}

func ChangedWorktreePathsSinceBase(ctx context.Context, worktree, baseSHA string) []string {
	if baseSHA != "" {
		return changedWorktreePathsSince(ctx, worktree, baseSHA)
	}
	return changedWorktreePaths(ctx, worktree)
}

func changedPackageFloorChecks(ctx context.Context, in ReviewInput, paths []string) []string {
	if in.Worktree == "" {
		return nil
	}
	pkgs := floorTestPackages(paths, in.errOut())
	moduleDir := codequality.ModuleDir(in.Worktree)
	pkgs = buildTagVisiblePackages(ctx, moduleDir, pkgs, in.errOut())
	taggedFails := addedTaggedTestFailures(ctx, in, moduleDir)
	if len(pkgs) == 0 {
		return floorLinesFor(nil, taggedFails, nil)
	}
	enforcedSet := enforcedPackages(ctx, in, moduleDir, pkgs)
	plain := make([]string, 0, len(pkgs))
	enforced := make([]string, 0, len(enforcedSet))
	for _, p := range pkgs {
		if enforcedSet[p] {
			enforced = append(enforced, p)
		} else {
			plain = append(plain, p)
		}
	}
	fails := in.retakeSelfCheckFailures(runBuildSelfCheck(ctx, moduleDir, plain, buildSelfCheckRunner), func(pkg string) (string, bool) {
		return buildSelfCheckRunner(ctx, moduleDir, pkg)
	})
	fails = in.laneFailures(ctx, fails)
	namingFails := apicoverNamingFailures(ctx, in, moduleDir, enforced, paths)
	removeBuildSelfCheckArtifact(in.Worktree)
	if len(fails) > 0 {
		writeBuildSelfCheckArtifact(in.Worktree, fails)
	}
	return floorLinesFor(fails, taggedFails, namingFails)
}

const floorTestsFailMarker = ": unit tests FAIL"

func floorLinesFor(fails, taggedFails []selfCheckFailure, namingFails []string) []string {
	out := make([]string, 0, len(fails)+len(taggedFails)+len(namingFails))
	for _, f := range fails {
		out = append(out, fmt.Sprintf("%s"+floorTestsFailMarker+"\n%s", f.Pkg, floorFailureDiagnostic(f.Output)))
	}
	for _, f := range taggedFails {
		out = append(out, fmt.Sprintf("%s"+floorTestsFailMarker+"\n%s", f.Pkg, floorFailureDiagnostic(f.Output)))
	}
	out = append(out, namingFails...)
	return out
}

func addedTaggedTestFailures(ctx context.Context, in ReviewInput, moduleDir string) []selfCheckFailure {
	var fails []selfCheckFailure
	for _, g := range in.added.groups {
		if len(g.Tags) == 0 {
			continue
		}
		for _, pkg := range withoutEnvExclusive(g.Packages, in.errOut()) {
			run := func() (string, bool) { return buildSelfCheckTaggedRunner(ctx, moduleDir, pkg, g.Tags) }
			label := pkg + " (-tags " + strings.Join(g.Tags, ",") + ")"
			if out, ok := run(); !ok {
				if out, ok = in.retakeFloorRun(label, out, []string{pkg}, run); !ok {
					fails = append(fails, selfCheckFailure{Pkg: label, Output: out})
				}
			}
		}
	}
	return fails
}

const floorFailureDiagnosticMax = 400

func floorFailureDiagnostic(output string) string {
	if correction, rest := splitCorrection(output); correction != "" {
		return joinCorrection(correction, floorFailureDiagnostic(rest))
	}
	if len(output) <= floorFailureDiagnosticMax {
		return output
	}
	var head string
	if lines, _ := goTestFailureLines(output); len(lines) > 0 {
		head = strings.Join(lines, "\n") + "\n"
	}
	tail := floorFailureDiagnosticMax - len(head)
	if tail <= 0 {
		return strings.ToValidUTF8(head[:floorFailureDiagnosticMax], "") + "…"
	}
	return head + "…" + strings.ToValidUTF8(output[len(output)-tail:], "")
}

func buildTagVisiblePackages(ctx context.Context, moduleDir string, pkgs []string, warn io.Writer) []string {
	if len(pkgs) == 0 {
		return pkgs
	}
	args := append([]string{"list", "-e", "-f", "{{.Dir}}\t{{len .GoFiles}}\t{{len .TestGoFiles}}\t{{len .XTestGoFiles}}"}, pkgs...)
	cmd := sysexec.Command(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(warn, "[build-floor] WARN: go list failed (%v) — build-tag visibility filter skipped this handoff\n", err)
		return pkgs
	}
	empty := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) != 4 {
			continue
		}
		if f[1] == "0" && f[2] == "0" && f[3] == "0" {
			empty[filepath.Clean(f[0])] = true
		}
	}
	if len(empty) == 0 {
		return pkgs
	}
	kept := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		dir := filepath.Clean(filepath.Join(moduleDir, strings.TrimPrefix(p, "./")))
		if empty[dir] {
			fmt.Fprintf(warn, "[build-floor] NOTE: %s has no Go files under the default build tags (build-tagged package) — excluded from the selfcheck run\n", p)
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

func apicoverNamingFailures(ctx context.Context, in ReviewInput, moduleDir string, enforced []string, changedPaths []string) []string {
	if len(enforced) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(enforced))
	for _, p := range enforced {
		dirs = append(dirs, filepath.Join(moduleDir, strings.TrimPrefix(p, "./")))
	}
	changedByDir := changedFileBasenamesByDir(moduleDir, dirs, changedPaths)
	coverFunc, testOut, status := scopedCoverFunc(ctx, in, moduleDir, enforced)
	if coverFunc != "" {
		defer func() { _ = os.RemoveAll(filepath.Dir(coverFunc)) }()
	}
	switch status {
	case coverStatusTestsFailed:
		return []string{fmt.Sprintf("enforced package tests FAIL (coverage run doubles as their selfcheck):\n%s", capKeepingCorrection(testOut, 600))}
	case coverStatusPlumbingError:
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: scoped coverage generation failed (%s) — apicover naming check skipped this handoff; audit/CI gates stay armed\n", testOut)
		return nil
	}
	var buf strings.Builder
	code, rerr := apicover.Run(ctx, apicover.Config{Enforce: true, Dirs: dirs, CoverPath: coverFunc, ChangedFilesByDir: changedByDir}, &buf)
	if rerr != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: apicover measurement failed (%v) — naming check skipped this handoff\n", rerr)
		return nil
	}
	if code == 0 {
		return nil
	}
	report := buf.String()
	if len(report) > 800 {
		report = report[:800] + "…"
	}
	return []string{fmt.Sprintf("apicover naming floor: %d enforced changed package(s) carry unnamed exports — name+exercise them (CI api-coverage-enforce would FAIL):\n%s", len(enforced), report)}
}

const (
	coverStatusOK = iota
	coverStatusTestsFailed
	coverStatusPlumbingError
)

var coverLockWait = verifylock.MaxWait

func coverTestArgs(profile string, pkgs, tags []string) []string {
	args := []string{"test", "-count=1", "-timeout", addedtests.PackageTimeout}
	args = append(args, covchanged.TagFlags(tags)...)
	return lanebudget.Args(append(append(args, "-coverprofile", profile), pkgs...))
}

func scopedCoverFunc(ctx context.Context, in ReviewInput, moduleDir string, pkgs []string) (path, output string, status int) {
	profile, out, status := scopedCoverProfile(ctx, in, moduleDir, pkgs, nil)
	if status != coverStatusOK {
		return strings.TrimSuffix(profile, "cover.out"), out, status
	}
	tmpDir := filepath.Dir(profile)
	funcOut := filepath.Join(tmpDir, "cover.func.txt")
	cmd2 := sysexec.Command(ctx, "go", "tool", "cover", "-func="+profile)
	cmd2.Dir = moduleDir
	cmd2.Env = ipcenv.Scrub(os.Environ())
	fo, err := cmd2.Output()
	if err != nil {
		return tmpDir + "/", "go tool cover: " + err.Error(), coverStatusPlumbingError
	}
	if err := os.WriteFile(funcOut, fo, 0o644); err != nil {
		return tmpDir + "/", err.Error(), coverStatusPlumbingError
	}
	return funcOut, "", coverStatusOK
}

func scopedCoverProfile(ctx context.Context, in ReviewInput, moduleDir string, pkgs, tags []string) (profile, output string, status int) {
	plan, out, status := firstCoverPass(ctx, in, coverPlan{moduleDir: moduleDir, label: strings.Join(pkgs, ","), pkgs: pkgs, tags: tags})
	if status == coverStatusTestsFailed {
		out, status = plan.retake(ctx, in, out)
	}
	return plan.profile, out, status
}

func firstCoverPass(ctx context.Context, in ReviewInput, plan coverPlan) (coverPlan, string, int) {
	root := filepath.Dir(plan.moduleDir)
	if release, lerr := verifylock.AcquireWithin(ctx, root, coverLockWait, in.errOut()); lerr == nil {
		defer release()
	} else {
		in.recordUnserializedFloor(root, lerr)
	}
	splits, err := coversplit.Load(plan.moduleDir)
	if err != nil {
		return plan, err.Error(), coverStatusPlumbingError
	}
	tmpDir, err := os.MkdirTemp("", "buildfloor-cover-*")
	if err != nil {
		return plan, err.Error(), coverStatusPlumbingError
	}
	plan.profile, plan.runs = filepath.Join(tmpDir, "cover.out"), coversplit.Plan(plan.pkgs, splits)
	out, status := plan.run(ctx)
	return plan, out, status
}

type coverPlan struct {
	moduleDir, profile, label string
	runs                      []coversplit.Run
	pkgs, tags                []string
}

func (p coverPlan) run(ctx context.Context) (string, int) {
	if len(p.runs) == 1 {
		return runCoverPass(ctx, p.moduleDir, p.profile, p.runs[0].Args(), p.tags)
	}
	return runSplitCoverPasses(ctx, p.moduleDir, p.profile, p.runs, p.tags)
}

func (p coverPlan) retake(ctx context.Context, in ReviewInput, first string) (string, int) {
	status := coverStatusTestsFailed
	out, outcome := in.retakeFloorOutcome(p.label, first, p.pkgs, func() (string, floorRetakeOutcome) {
		var out string
		out, status = p.run(ctx)
		return out, coverRetakeOutcome(status)
	})
	if outcome == floorRetakeNewTestRed {
		return out, coverStatusTestsFailed
	}
	return coverRetakeResult(first, out, status)
}

func coverRetakeOutcome(status int) floorRetakeOutcome {
	switch status {
	case coverStatusOK:
		return floorRetakeGreen
	case coverStatusPlumbingError:
		return floorRetakeUnrunnable
	}
	return floorRetakeRed
}

func runCoverPass(ctx context.Context, moduleDir, profile string, args, tags []string) (string, int) {
	cmd := sysexec.Command(ctx, "go", coverTestArgs(profile, args, tags)...)
	cmd.Dir = moduleDir
	cmd.Env = lanebudget.Env(ipcenv.Scrub(os.Environ()))
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), coverStatusTestsFailed
	}
	return "", coverStatusOK
}

func runSplitCoverPasses(ctx context.Context, moduleDir, profile string, runs []coversplit.Run, tags []string) (string, int) {
	var parts [][]byte
	for i, run := range runs {
		part := fmt.Sprintf("%s.%d", profile, i)
		if out, status := runCoverPass(ctx, moduleDir, part, run.Args(), tags); status != coverStatusOK {
			return out, status
		}
		data, err := os.ReadFile(part)
		if err != nil {
			return err.Error(), coverStatusPlumbingError
		}
		parts = append(parts, data)
	}
	merged, err := coversplit.Merge(parts)
	if err != nil {
		return err.Error(), coverStatusPlumbingError
	}
	if err := os.WriteFile(profile, merged, 0o644); err != nil {
		return err.Error(), coverStatusPlumbingError
	}
	return "", coverStatusOK
}

func changedFileBasenamesByDir(moduleDir string, dirs []string, changedPaths []string) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(dirs))
	worktree := filepath.Dir(moduleDir)
	for _, d := range dirs {
		out[d] = map[string]bool{}
	}
	for _, p := range changedPaths {
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		abs := filepath.Join(worktree, p)
		d := filepath.Dir(abs)
		if set, ok := out[d]; ok {
			set[filepath.Base(p)] = true
		}
	}
	return out
}

func (in ReviewInput) recordUnserializedFloor(root string, lockErr error) {
	lock, _ := verifylock.LockPath(root)
	fmt.Fprintf(in.errOut(), "[build-floor] WARN: verification single-flight unavailable at %s (%v) — running unserialized\n", lock, lockErr)
	if in.Workspace == "" {
		return
	}
	if err := dispatchevents.NewWriter(in.Workspace).EmitVerifySingleFlightUnavailable(in.Cycle, "build-floor", lock, lockErr); err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN: the unserialized run was not recorded as an abnormal event: %v\n", err)
	}
}
