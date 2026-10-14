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
	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/docsfloor"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

type BuildFloorCheckFn func(ctx context.Context, in ReviewInput) []string

func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {
	out := RemovalClaimFailures(ctx, in)
	out = append(out, PlaceholderTokenFailures(ctx, in)...)
	paths := changedFloorPaths(ctx, in)
	out = append(out, personaBudgetFailures(ctx, in.Worktree, paths)...)
	docsFloorWarn(in, paths)
	out = append(out, commentFloorFailures(ctx, in)...)
	out = append(out, coverChangedFailures(ctx, in, paths)...)
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
	})
}

func docsFloorWarn(in ReviewInput, paths []string) {
	var cfg docsfloor.Config
	steStage := policy.Policy{}.DocsFloorConfig().SteStage
	if in.ProjectRoot != "" {
		if p, err := policy.Load(filepath.Join(in.ProjectRoot, ".evolve", "policy.json")); err == nil {
			cfg.Stage = p.DocsFloorConfig().Stage
			steStage = p.DocsFloorConfig().SteStage
		}
	}
	steLintWarn(in.Worktree, steStage, paths, in.errOut())
	v := docsfloor.Evaluate(cfg, docsfloor.Input{
		ArchitectureLabeled: docsfloor.IsArchitectureClass(paths),
		ChangedFiles:        paths,
	})
	if v.Status == docsfloor.StatusWarn {
		fmt.Fprintf(in.errOut(), "[docs-floor] WARN: %s\n", v.Reason)
	}
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
	pkgs := changedGoTestPackages(paths)
	moduleDir := codequality.ModuleDir(in.Worktree)
	pkgs = buildTagVisiblePackages(ctx, moduleDir, pkgs, in.errOut())
	taggedFails := addedTaggedTestFailures(ctx, in, moduleDir)
	if len(pkgs) == 0 {
		return floorLinesFor(nil, taggedFails, nil)
	}
	enforcedSet := map[string]bool{}
	if enforceBytes, err := os.ReadFile(filepath.Join(moduleDir, ".apicover-enforce")); err == nil {
		for _, p := range ciparity.IntersectEnforced(pkgs, enforceBytes) {
			enforcedSet[p] = true
		}
	}
	plain := make([]string, 0, len(pkgs))
	enforced := make([]string, 0, len(enforcedSet))
	for _, p := range pkgs {
		if enforcedSet[p] {
			enforced = append(enforced, p)
		} else {
			plain = append(plain, p)
		}
	}
	fails := runBuildSelfCheck(ctx, moduleDir, plain, buildSelfCheckRunner)
	namingFails := apicoverNamingFailures(ctx, moduleDir, enforced, paths)
	removeBuildSelfCheckArtifact(in.Worktree)
	if len(fails) > 0 {
		writeBuildSelfCheckArtifact(in.Worktree, fails)
	}
	return floorLinesFor(fails, taggedFails, namingFails)
}

func floorLinesFor(fails, taggedFails []selfCheckFailure, namingFails []string) []string {
	out := make([]string, 0, len(fails)+len(taggedFails)+len(namingFails))
	for _, f := range fails {
		out = append(out, fmt.Sprintf("%s: unit tests FAIL\n%s", f.Pkg, floorFailureDiagnostic(f.Output)))
	}
	for _, f := range taggedFails {
		out = append(out, fmt.Sprintf("%s: unit tests FAIL\n%s", f.Pkg, floorFailureDiagnostic(f.Output)))
	}
	out = append(out, namingFails...)
	return out
}

func addedTaggedTestFailures(ctx context.Context, in ReviewInput, moduleDir string) []selfCheckFailure {
	if in.Worktree == "" || in.WorktreeBaseSHA == "" {
		return nil
	}
	files, ok := changedpkgs.ChangedFilesChecked(in.Worktree, in.WorktreeBaseSHA)
	if !ok {
		return nil
	}
	groups, excluded, err := addedtests.Groups(in.Worktree, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: added-test grouping failed (%v) — tag-gated added packages not run at the floor; the ship backstop still stands\n", err)
		return nil
	}
	for _, p := range excluded {
		fmt.Fprintf(os.Stderr, "[build-floor] EXCLUDED %s (requires_tmux or another build constraint unavailable on this host)\n", p)
	}
	var fails []selfCheckFailure
	for _, g := range groups {
		if len(g.Tags) == 0 {
			continue
		}
		for _, pkg := range g.Packages {
			if out, ok := buildSelfCheckTaggedRunner(ctx, moduleDir, pkg, g.Tags); !ok {
				fails = append(fails, selfCheckFailure{Pkg: pkg + " (-tags " + strings.Join(g.Tags, ",") + ")", Output: out})
			}
		}
	}
	return fails
}

const floorFailureDiagnosticMax = 400

func floorFailureDiagnostic(output string) string {
	if len(output) <= floorFailureDiagnosticMax {
		return output
	}
	return "…" + output[len(output)-floorFailureDiagnosticMax:]
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

func apicoverNamingFailures(ctx context.Context, moduleDir string, enforced []string, changedPaths []string) []string {
	if len(enforced) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(enforced))
	for _, p := range enforced {
		dirs = append(dirs, filepath.Join(moduleDir, strings.TrimPrefix(p, "./")))
	}
	changedByDir := changedFileBasenamesByDir(moduleDir, dirs, changedPaths)
	coverFunc, testOut, status := scopedCoverFunc(ctx, moduleDir, enforced)
	if coverFunc != "" {
		defer func() { _ = os.RemoveAll(filepath.Dir(coverFunc)) }()
	}
	switch status {
	case coverStatusTestsFailed:
		head := testOut
		if len(head) > 600 {
			head = head[:600] + "…"
		}
		return []string{fmt.Sprintf("enforced package tests FAIL (coverage run doubles as their selfcheck):\n%s", head)}
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
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	return append(append(args, "-coverprofile", profile), pkgs...)
}

func scopedCoverFunc(ctx context.Context, moduleDir string, pkgs []string) (path, output string, status int) {
	profile, out, status := scopedCoverProfile(ctx, moduleDir, pkgs, nil)
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

func scopedCoverProfile(ctx context.Context, moduleDir string, pkgs, tags []string) (profile, output string, status int) {
	if release, lerr := verifylock.AcquireWithin(ctx, filepath.Dir(moduleDir), coverLockWait, os.Stderr); lerr == nil {
		defer release()
	} else {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: verification single-flight unavailable (%v) — running unserialized\n", lerr)
	}
	tmpDir, err := os.MkdirTemp("", "buildfloor-cover-*")
	if err != nil {
		return "", err.Error(), coverStatusPlumbingError
	}
	profile = filepath.Join(tmpDir, "cover.out")
	cmd := sysexec.Command(ctx, "go", coverTestArgs(profile, pkgs, tags)...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return profile, string(out), coverStatusTestsFailed
	}
	return profile, "", coverStatusOK
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
