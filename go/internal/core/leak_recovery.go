package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

var evolveDeliverablePrefixes = []string{
	evalsDir,
	".evolve/phases/",
	".evolve/commit-prefix-scope.json",
}

// isEvolveDeliverablePath matches a directory prefix but requires exact
// equality for a file entry, so a look-alike name cannot false-positive.
func isEvolveDeliverablePath(p string) bool {
	for _, pfx := range evolveDeliverablePrefixes {
		if strings.HasSuffix(pfx, "/") {
			if p == strings.TrimSuffix(pfx, "/") || strings.HasPrefix(p, pfx) {
				return true
			}
		} else if p == pfx {
			return true
		}
	}
	return false
}

type writeAuthority int

const (
	readOnlyAuthority writeAuthority = iota
	evalAuthorAuthority
	sourceWriterAuthority
)

type phaseLeakScope struct {
	projectRoot string
	cycleState  CycleState
	phase       Phase
	baseline    map[string]bool
	leased      map[string]bool
}

func (o *Orchestrator) recoverPhaseLeak(ctx context.Context, scope phaseLeakScope) error {
	if !o.leakRecoverablePhase(scope.phase) || scope.cycleState.ActiveWorktree == "" {
		return nil
	}
	if scope.baseline == nil {
		return nil
	}
	ownership := laneOwnership(scope.cycleState, activeVerifiedMints(scope.projectRoot))
	recovery := leakRecovery{
		projectRoot: scope.projectRoot,
		worktree:    scope.cycleState.ActiveWorktree,
		cycle:       scope.cycleState.CycleID,
		baseline:    scope.baseline,
		authority:   o.writeAuthority(scope.phase),
		ownership:   ownership,
		exempt:      ownership.exemptions(scope.projectRoot, scope.cycleState.WorkspacePath, scope.leased),
	}
	if recoverBuildLeak(ctx, recovery) {
		return nil
	}
	return fmt.Errorf("phase %s: worktree-leak recovery failed (main tree left unsafe for review and audit)", scope.phase)
}

func (cr *cycleRun) recoveryBaselineFor(guard *treediff.Guard, before []string) map[string]bool {
	if guard == nil || !cr.mainTreeIsCheckout() {
		return nil
	}
	set := make(map[string]bool, len(before))
	for _, p := range before {
		set[p] = true
	}
	return set
}

type checkoutDecision struct {
	decided  bool
	checkout bool
}

func (cr *cycleRun) mainTreeIsCheckout() bool {
	if cr.checkout == nil {
		cr.checkout = &checkoutDecision{}
	}
	if !cr.checkout.decided {
		cr.checkout.decided, cr.checkout.checkout = true, !notAGitCheckout(cr.req.ProjectRoot)
	}
	return cr.checkout.checkout
}

func notAGitCheckout(projectRoot string) bool {
	_, err := plane.Classify(projectRoot)
	return errors.Is(err, fs.ErrNotExist)
}

const (
	snapshotAttempts     = 3
	snapshotRetryBackoff = 100 * time.Millisecond
)

var errPrePhaseSnapshot = fmt.Errorf("pre-phase main-tree snapshot failed after %d attempts, so the phase was not run", snapshotAttempts)

func withSnapshotRetries(ctx context.Context, read func() error) error {
	err := read()
	for attempt := 1; err != nil && attempt < snapshotAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		backoffSleep(time.Duration(attempt) * snapshotRetryBackoff)
		err = read()
	}
	return err
}

func retryingDirtyPaths(read treediff.GitDirtyFn) treediff.GitDirtyFn {
	if read == nil {
		return nil
	}
	return func(ctx context.Context, repoRoot string) ([]string, error) {
		var paths []string
		err := withSnapshotRetries(ctx, func() error {
			var readErr error
			paths, readErr = read(ctx, repoRoot)
			return readErr
		})
		return paths, err
	}
}

func readMainTreeStatus(ctx context.Context, projectRoot string) (string, error) {
	var out string
	err := withSnapshotRetries(ctx, func() error {
		stdout, code, err := gitCapture(ctx, projectRoot, "status", "--porcelain", "-uall")
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("git status exit %d", code)
		}
		out = stdout
		return nil
	})
	return out, err
}

func (cr *cycleRun) snapshotThenDispatch(next Phase, req PhaseRequest, hooks retryOpts) (map[string]bool, PhaseResponse, int, error) {
	guard, before, err := cr.snapshotMainTree(next)
	if err != nil {
		return nil, PhaseResponse{}, 0, err
	}
	resp, attempts, err := cr.retryPhaseRunner(next, req, hooks)
	return cr.recoveryBaselineFor(guard, before), resp, attempts, err
}

func (o *Orchestrator) writeAuthority(p Phase) writeAuthority {
	switch {
	case o.worktreePhase(p):
		return sourceWriterAuthority
	case p == PhaseScout:
		return evalAuthorAuthority
	}
	return readOnlyAuthority
}

type leakRecovery struct {
	projectRoot string
	worktree    string
	cycle       int
	baseline    map[string]bool
	authority   writeAuthority
	ownership   mainTreeOwnership
	exempt      leakExemptions
}

func (r leakRecovery) authors(p string) bool {
	switch r.authority {
	case sourceWriterAuthority:
		return true
	case evalAuthorAuthority:
		return r.ownership.ownsEval(p)
	}
	return false
}

func recoverBuildLeak(ctx context.Context, r leakRecovery) bool {
	if r.worktree == "" || inPlaceWorktree(r.worktree, r.projectRoot) {
		return true
	}
	out, err := readMainTreeStatus(ctx, r.projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: cannot read the main tree after %d attempts: %v — recovery fails rather than report the tree clean\n", snapshotAttempts, err)
		return false
	}
	var staged []string
	for _, line := range strings.Split(out, "\n") {
		p, isNew := r.newLeak(line)
		if !isNew {
			continue
		}
		stage, recovered := r.recoverPath(ctx, line[:2], p)
		if !recovered {
			return false
		}
		if stage {
			staged = append(staged, p)
		}
	}
	return stageRelocated(ctx, r.worktree, staged)
}

func (r leakRecovery) newLeak(line string) (string, bool) {
	if len(line) < 4 {
		return "", false
	}
	p := porcelainPath(line)
	if p == "" || r.baseline[p] {
		return "", false
	}
	return p, !isLegitimateMainTreePath(p) || buildArtifacts[p]
}

func (r leakRecovery) recoverPath(ctx context.Context, xy, p string) (stage, recovered bool) {
	if r.exempt.leased[p] {
		fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: left %s in the main tree — the console lease holds it\n", p)
		return false, true
	}
	if owner, foreign := r.ownership.foreignOwner(p); foreign {
		return false, r.leaveForOwner(p, owner)
	}
	switch {
	case strings.Contains(xy, "?"):
		return r.recoverUntracked(p)
	case buildArtifacts[p]:
		return false, r.discard(ctx, p, "rebuilt artifact")
	case !r.authors(p):
		return false, true
	case strings.Contains(xy, "M"):
		return r.recoverTrackedEdit(ctx, p)
	case strings.ContainsAny(xy, "AD"):
		return false, r.discard(ctx, p, "main-tree change")
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: unrecoverable leak status %q for %s (falling through to abort)\n", xy, p)
	return false, false
}

func (r leakRecovery) recoverUntracked(p string) (stage, recovered bool) {
	authored := r.authors(p)
	if !authored && isEvolveDeliverablePath(p) {
		return false, true
	}
	src, dst := filepath.Join(r.projectRoot, p), filepath.Join(r.worktree, p)
	if info, err := os.Lstat(dst); err == nil && info.Mode().IsRegular() {
		return false, r.setAsideShadowedLeak(src, dst, p)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: mkdir for %s: %v\n", p, err)
		return false, false
	}
	if err := moveFile(src, dst); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: relocate %s: %v\n", p, err)
		return false, false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: relocated leaked %s out of main tree\n", p)
	return authored, true
}

func (r leakRecovery) leaveForOwner(p, owner string) bool {
	if !r.exempt.heldElsewhere(p) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %s belongs to %s, not this lane, and no live lane holds it — recovery refuses to claim it or leave it unjudged\n", p, owner)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: left %s in the main tree — it belongs to %s, not this lane\n", p, owner)
	return true
}

func (r leakRecovery) setAsideShadowedLeak(src, dst, p string) bool {
	same, err := sameBytes(src, dst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: compare leaked %s with the worktree's copy: %v\n", p, err)
		return false
	}
	if same {
		return removeDuplicateLeak(src, p)
	}
	quarantined := uniqueQuarantinePath(filepath.Join(quarantineDir(r.projectRoot, r.cycle), p))
	if err := moveFile(src, quarantined); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: quarantine leaked %s: %v\n", p, err)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: leaked %s differs from the worktree's own copy, which wins; the main-tree copy is kept at %s\n", p, quarantined)
	return true
}

func removeDuplicateLeak(src, p string) bool {
	if err := os.Remove(src); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: drop leaked %s: %v\n", p, err)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: dropped leaked %s — the worktree already holds the same bytes\n", p)
	return true
}

func sameBytes(a, b string) (bool, error) {
	left, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	right, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(left, right), nil
}

func (r leakRecovery) recoverTrackedEdit(ctx context.Context, p string) (stage, recovered bool) {
	if !worktreeCleanForPath(ctx, r.worktree, p) {
		return false, r.discard(ctx, p, "main-tree change (worktree diverged)")
	}
	if err := relocateTrackedEdit(ctx, r.projectRoot, r.worktree, p); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: relocate tracked edit %s: %v\n", p, err)
		return false, false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: relocated leaked tracked edit %s into worktree\n", p)
	return true, true
}

func (r leakRecovery) discard(ctx context.Context, p, kind string) bool {
	if err := discardMainLeak(ctx, r.projectRoot, p); err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: %v\n", err)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: discarded leaked %s %s\n", kind, p)
	return true
}

func stageRelocated(ctx context.Context, worktree string, paths []string) bool {
	if len(paths) == 0 {
		return true
	}
	args := append([]string{"add", "-f", "--"}, paths...)
	if _, c, e := gitCapture(ctx, worktree, args...); e != nil || c != 0 {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-leak-recover: git add of relocated paths failed (rc=%d): %v — aborting recovery\n", c, e)
		return false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] build-leak-recover: %d leaked path(s) relocated into worktree; main tree restored\n", len(paths))
	return true
}

// moveFile relocates src→dst, falling back to copy+remove when os.Rename
// fails. os.Rename returns EXDEV when src and dst are on different
// filesystems, e.g. when the worktree base resolves to another mount.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if fi, serr := os.Stat(src); serr == nil {
		mode = fi.Mode()
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

// relocateTrackedEdit assumes the worktree's copy of p is at HEAD (the
// caller checks), so overlaying the leaked content never clobbers
// independent in-worktree work.
func relocateTrackedEdit(ctx context.Context, projectRoot, worktree, p string) error {
	if err := copyFile(filepath.Join(projectRoot, p), filepath.Join(worktree, p)); err != nil {
		return fmt.Errorf("relocate content of %s: %w", p, err)
	}
	return discardMainLeak(ctx, projectRoot, p) // restore main to HEAD; the worktree now holds the edit
}

// `git checkout HEAD -- p` resets both index and working tree, so it also
// discards a staged-only leak (plain `git checkout -- p` would no-op).
func discardMainLeak(ctx context.Context, projectRoot, p string) error {
	// gitCapture reports a non-zero exit as (c!=0, e==nil); branch instead of
	// wrapping a nil error with %w, which would render "<nil>" and break
	// errors.Is/As.
	_, c, e := gitCapture(ctx, projectRoot, "checkout", "HEAD", "--", p)
	if e != nil {
		return fmt.Errorf("git checkout HEAD -- %s: %w", p, e)
	}
	if c != 0 {
		return fmt.Errorf("git checkout HEAD -- %s: exit %d", p, c)
	}
	return nil
}

func isGitignored(ctx context.Context, dir, p string) bool {
	// git check-ignore -q exits 0 if the path is ignored, 1 if not ignored.
	_, code, err := gitCapture(ctx, dir, "check-ignore", "-q", "--", p)
	return err == nil && code == 0
}

// `git diff --quiet HEAD -- p` exits 0 (clean) / 1 (differs); a launch error
// is treated as "not clean" so the caller falls back to the conservative
// discard path.
func worktreeCleanForPath(ctx context.Context, worktree, p string) bool {
	_, c, e := gitCapture(ctx, worktree, "diff", "--quiet", "HEAD", "--", p)
	return e == nil && c == 0
}

// buildArtifacts are the tracked build outputs a leaked rebuild targets.
// go/bin/evolve is gitignored and normally never appears in `git status`,
// but is listed defensively.
var buildArtifacts = map[string]bool{
	"go/evolve":     true,
	"go/bin/evolve": true,
}

// isLegitimateMainTreePath reports whether a main-tree path is a write
// target that must not trigger the tree-diff guard: tracked build artifacts,
// non-deliverable `.evolve/` state at any nesting depth, and bare directory
// entries from `-uall`. It is the one classification both recoverBuildLeak
// and the boundary guard consult.
func isLegitimateMainTreePath(p string) bool {
	// Build artifacts: a builder may rebuild these into the main tree.
	if buildArtifacts[p] {
		return true
	}
	// Trailing "/" = a bare directory entry (nested worktree/submodule that
	// `-uall` reports without recursing): never a file we can act on.
	if strings.HasSuffix(p, "/") {
		return true
	}
	// `.evolve/` paths at any nesting depth are runtime state — EXCEPT the
	// evolve deliverable prefixes (evals/, phases/, commit-prefix-scope.json)
	// which only flow through worktree-based phases and must never escape.
	if p == ".evolve" || strings.HasPrefix(p, ".evolve/") || strings.Contains(p, "/.evolve/") {
		return !isEvolveDeliverablePath(p)
	}
	return false
}
