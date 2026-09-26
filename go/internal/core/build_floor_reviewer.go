package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

// BuildFloorCheckFn runs deterministic, LLM-free build-floor checks for a completed build and returns the failures.
type BuildFloorCheckFn func(ctx context.Context, in ReviewInput) []string

type buildFloorReviewer struct {
	checks BuildFloorCheckFn
}

// NewBuildFloorReviewer wraps an injected check engine; a nil engine approves with a WARN, so roots can wire it unconditionally.
func NewBuildFloorReviewer(checks BuildFloorCheckFn) DeliverableReviewer {
	return &buildFloorReviewer{checks: checks}
}

func (r *buildFloorReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) {
		return ReviewResult{Approve: true}
	}
	if r.checks == nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: no deterministic check engine wired — failing open (downstream gates stay armed)\n")
		return ReviewResult{Approve: true}
	}
	failures := r.checks(ctx, in)
	if len(failures) == 0 {
		return ReviewResult{Approve: true}
	}
	reason := fmt.Sprintf("build handoff floor: %d deterministic check failure(s) — fix these exactly before handoff:\n  %s",
		len(failures), strings.Join(failures, "\n  "))
	fmt.Fprintf(os.Stderr, "[build-floor] REJECT: %s\n", reason)
	return ReviewResult{Approve: false, Retry: true, Reason: reason}
}

// DefaultBuildFloorChecks is the production deterministic engine of the build handoff floor.
func DefaultBuildFloorChecks(ctx context.Context, in ReviewInput) []string {
	// Only changedPackageFloorChecks is package-driven; the rest run here because it skips a diff with no Go packages.
	out := RemovalClaimFailures(ctx, in)
	out = append(out, PlaceholderTokenFailures(ctx, in)...)
	paths := changedFloorPaths(ctx, in)
	out = append(out, personaBudgetFailures(ctx, in.Worktree, paths)...)
	// The docs floor only WARNs: an undocumented architecture change is an auditor finding, never a handoff REJECT.
	docsFloorWarn(in, paths)
	return append(out, changedPackageFloorChecks(ctx, in, paths)...)
}

// ProtectedSurfaceFloorChecks fails the handoff for every changed path on the protected control plane.
func ProtectedSurfaceFloorChecks(member func(string) bool) BuildFloorCheckFn {
	return func(ctx context.Context, in ReviewInput) []string {
		if in.Worktree == "" {
			return nil
		}
		base := in.WorktreeBaseSHA
		if base == "" {
			base = "HEAD"
		}
		// Rename detection is off, so a file moved out of the surface is judged by its old path too.
		return protectedSurfaceFailures(changedWorktreePathsSince(ctx, in.Worktree, base, "--no-renames"), member, base)
	}
}

// protectedSurfaceFailures fails open on a nil predicate; ship's tripwire stays armed.
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

// NewBuildExplanationReviewer returns the mandatory Build explanation floor, composed outside the optional reviewer seam.
func NewBuildExplanationReviewer() DeliverableReviewer {
	return NewBuildFloorReviewer(func(ctx context.Context, in ReviewInput) []string {
		return explanationDocumentationFailures(ctx, in)
	})
}

func explanationDocumentationFailures(ctx context.Context, in ReviewInput) []string {
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

// docsFloorWarn prints the documentation floor's WARN to stderr, the phase log the auditor reads.
// See ADR-0077.
func docsFloorWarn(in ReviewInput, paths []string) {
	var cfg docsfloor.Config
	if in.ProjectRoot != "" {
		if p, err := policy.Load(filepath.Join(in.ProjectRoot, ".evolve", "policy.json")); err == nil {
			cfg.Stage = p.DocsFloorConfig().Stage
		}
	}
	// The blocking-grade classifier drops test-only diffs; the verdict still stays WARN.
	v := docsfloor.Evaluate(cfg, docsfloor.Input{
		ArchitectureLabeled: docsfloor.IsArchitectureClass(paths),
		ChangedFiles:        paths,
	})
	if v.Status == docsfloor.StatusWarn {
		fmt.Fprintf(os.Stderr, "[docs-floor] WARN: %s\n", v.Reason)
	}
}

// changedFloorPaths diffs against the cycle base, not HEAD: the builder commits its work, so a HEAD diff is empty
// at review time and the floor would approve vacuously.
func changedFloorPaths(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	if in.WorktreeBaseSHA != "" {
		return changedWorktreePathsSince(ctx, in.Worktree, in.WorktreeBaseSHA)
	}
	return changedWorktreePaths(ctx, in.Worktree)
}

// changedPackageFloorChecks runs the post-build selfcheck machinery as a rejecting floor, one line per failing
// package. An inability to run is green, because downstream gates stay armed.
func changedPackageFloorChecks(ctx context.Context, in ReviewInput, paths []string) []string {
	if in.Worktree == "" {
		return nil
	}
	// reviewAndGuard normalizes gofmt and derived projections first, so this tests the tree Audit receives.
	pkgs := changedGoTestPackages(paths)
	moduleDir := codequality.ModuleDir(in.Worktree)
	pkgs = buildTagVisiblePackages(ctx, moduleDir, pkgs)
	// Added tag-gated packages are invisible to the default-context run, so they run under their own tags.
	taggedFails := addedTaggedTestFailures(ctx, in, moduleDir)
	if len(pkgs) == 0 {
		return floorLinesFor(nil, taggedFails, nil)
	}
	// Enforced packages run once, in apicoverNamingFailures' coverage pass, which doubles as their selfcheck.
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
	// The ACS toolchain gate reads this artifact; the advisory binding skips its duplicate run when the floor is enforced.
	removeBuildSelfCheckArtifact(in.Worktree)
	if len(fails) > 0 {
		writeBuildSelfCheckArtifact(in.Worktree, fails)
	}
	return floorLinesFor(fails, taggedFails, namingFails)
}

// floorLinesFor names each tagged failure's tags so the builder re-runs exactly what the floor ran.
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

// addedTaggedTestFailures runs every added test package under its declared build tags, with the ship backstop's
// seed and grouping. It fails open when the seed cannot be derived.
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

// floorFailureDiagnosticMax bounds one package's output, which becomes the next builder's whole failure story.
const floorFailureDiagnosticMax = 400

// floorFailureDiagnostic keeps the tail, because `go test` writes its FAIL lines, panics and stack traces last.
func floorFailureDiagnostic(output string) string {
	if len(output) <= floorFailureDiagnosticMax {
		return output
	}
	return "…" + output[len(output)-floorFailureDiagnosticMax:]
}

// buildTagVisiblePackages drops changed packages with no Go files under the default build tags, whose `go test` is a
// setup failure rather than a test failure. It fails open: a `go list` error returns the input unchanged.
func buildTagVisiblePackages(ctx context.Context, moduleDir string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return pkgs
	}
	args := append([]string{"list", "-e", "-f", "{{.Dir}}\t{{len .GoFiles}}\t{{len .TestGoFiles}}\t{{len .XTestGoFiles}}"}, pkgs...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: go list failed (%v) — build-tag visibility filter skipped this handoff\n", err)
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
			fmt.Fprintf(os.Stderr, "[build-floor] NOTE: %s has no Go files under the default build tags (build-tagged package) — excluded from the selfcheck run\n", p)
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// apicoverNamingFailures applies CI's apicover naming floor at handoff. Its coverage run doubles as the enforced
// packages' selfcheck, and every fail-open plumbing branch WARNs.
func apicoverNamingFailures(ctx context.Context, moduleDir string, enforced []string, changedPaths []string) []string {
	if len(enforced) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(enforced))
	for _, p := range enforced {
		dirs = append(dirs, filepath.Join(moduleDir, strings.TrimPrefix(p, "./")))
	}
	// Only violations in files this change touched hard-fail; a touched package's older debt only WARNs.
	changedByDir := changedFileBasenamesByDir(moduleDir, dirs, changedPaths)
	// apicover's enforce contract needs a coverage profile, or every named export reads as false-green.
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

// scopedCoverFunc returns the cover -func path, the test output, and a status that separates test failures from
// plumbing errors. The per-run -timeout keeps one hung package from wedging the check.
func scopedCoverFunc(ctx context.Context, moduleDir string, pkgs []string) (path, output string, status int) {
	// The coverage run is a full go-test execution, so it takes the host-wide verification single-flight.
	// A lock failure degrades to unserialized, never to skipped verification.
	if release, lerr := verifylock.Acquire(ctx, filepath.Dir(moduleDir), os.Stderr); lerr == nil {
		defer release()
	} else {
		fmt.Fprintf(os.Stderr, "[build-floor] WARN: verification single-flight unavailable (%v) — running unserialized\n", lerr)
	}
	tmpDir, err := os.MkdirTemp("", "buildfloor-cover-*")
	if err != nil {
		return "", err.Error(), coverStatusPlumbingError
	}
	profile := filepath.Join(tmpDir, "cover.out")
	args := append([]string{"test", "-count=1", "-timeout", "300s", "-coverprofile", profile}, pkgs...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.Scrub(os.Environ())
	if out, err := cmd.CombinedOutput(); err != nil {
		return tmpDir + "/", string(out), coverStatusTestsFailed
	}
	funcOut := filepath.Join(tmpDir, "cover.func.txt")
	cmd2 := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profile)
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

// changedFileBasenamesByDir maps each enforced package dir to the basenames of its changed .go files.
func changedFileBasenamesByDir(moduleDir string, dirs []string, changedPaths []string) map[string]map[string]bool {
	out := make(map[string]map[string]bool, len(dirs))
	worktree := filepath.Dir(moduleDir) // moduleDir = <worktree>/go
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
