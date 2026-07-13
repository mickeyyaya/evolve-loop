package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) recoverFromShipError(ctx context.Context, cycle int, cs CycleState, se *ShipError, depth, fleetWidth int) (Phase, bool) {
	o.recordShipError(ctx, cycle, cs, se)
	budget := shipRecoveryBudget(se.Code, fleetWidth)
	if depth >= budget {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery exhausted after %d attempt(s) (%s/%s, budget %d, fleet width %d); aborting\n", depth, se.Code, se.Class, budget, fleetWidth)
		return "", false
	}
	// Contention-class errors (a sibling lane moved main) back off with jitter
	// before re-auditing so lockstep siblings don't re-collide on every attempt.
	// Non-contention errors recover immediately — a pause fixes nothing there.
	if isContentionShipCode(se.Code) {
		pause := contentionBackoff(depth)
		fmt.Fprintf(os.Stderr, "[orchestrator] contention backoff %s before ship recovery attempt %d/%d (%s)\n", pause, depth+1, budget, se.Code)
		backoffSleep(pause)
	}
	// ADR-0049 S5b: a fleet ff-merge divergence (a peer cycle moved main) is
	// recovered by rebasing the cycle branch onto the new main BEFORE the
	// re-audit (the router routes this code to audit). A clean rebase replays
	// this cycle's patches onto the peer's changes → re-audit re-binds the merged
	// tree → re-ship fast-forwards. A conflict confined to GENERATED projections
	// (e.g. control-flags.md) is auto-resolved by regenerating them from the
	// merged source (rebaseWithDerivedRegen) — every flag cycle rewrites that
	// projection, so the partition cannot separate them and a debugger round-trip
	// would be pure waste. A conflict touching any NON-derived path is genuine
	// overlapping work the partition should have kept apart — abort loud.
	// The code/class used for the routing decision; a fleet rebase may reclassify
	// a clean RebaseNeeded into a CONFLICT (G13a) that routes to the debugger.
	recoverCode, recoverClass := se.Code, se.Class
	if se.Code == CodeGitFleetRebaseNeeded {
		ok, conflict := rebaseCycleBranchOntoMain(ctx, cs.ActiveWorktree)
		switch {
		case ok:
			// Clean (or derived-only, regenerated) replay → router routes
			// RebaseNeeded to audit (re-bind the merged tree).
		case conflict:
			// Genuine overlapping work — a re-audit cannot resolve it. Reclassify to
			// the integrity-class conflict code so recovery routes to the debugger.
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase CONFLICT (overlapping work the partition should have separated) → debugger\n", cycle)
			recoverCode, recoverClass = CodeGitFleetRebaseConflict, ShipClassIntegrity
		default:
			fmt.Fprintf(os.Stderr, "[orchestrator] cycle %d fleet rebase onto main failed (infra); aborting\n", cycle)
			return "", false
		}
	}
	// Recovery is deterministic Chain-of-Responsibility (no LLM); both routing
	// strategies just delegate to the pure router.Recover, so call it directly.
	// This keeps recovery available even when no routing Strategy was wired
	// (e.g. Stage:Off) — error handling must not depend on routing being on.
	dec := router.Recover(router.RouteInput{
		Blocker: &router.Blocker{
			Code:  string(recoverCode),
			Class: string(recoverClass),
			Stage: string(se.Stage),
		},
	})
	cand := o.candidatePhase(dec.NextPhase)
	if cand == "" || cand == PhaseEnd {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) is unrecoverable (%s); aborting\n", se.Code, se.Class, dec.Reason)
		return "", false
	}
	if !o.sm.CanTransition(PhaseShip, cand) {
		fmt.Fprintf(os.Stderr, "[orchestrator] ship recovery proposed illegal edge ship→%s (%s); aborting\n", cand, dec.Reason)
		return "", false
	}
	fmt.Fprintf(os.Stderr, "[orchestrator] ship error %s (%s) → recovery routes to %s (%s)\n", se.Code, se.Class, cand, dec.Reason)
	return cand, true
}

// gitFn runs a git subcommand in dir and returns (stdout, exitCode, err); it
// matches gitCapture so production wiring passes gitCapture directly while tests
// inject a fake — the Humble Object seam for rebaseWithDerivedRegen.
type gitFn = func(ctx context.Context, dir string, args ...string) (string, int, error)

// regenFn regenerates the derived projection identified by relPath from the
// worktree's (post-rebase, merged) source-of-truth. relPath identifies WHICH
// projection (the registry key); the callee decides where to write it (the
// production impl writes via EVOLVE_WORKTREE_ROOT, not worktree/relPath directly).
type regenFn = func(ctx context.Context, worktree, relPath string) error

// derivedArtifactSpec describes a GENERATED projection: how to regenerate it and
// the source-of-truth path prefix whose change makes the projection stale.
type derivedArtifactSpec struct {
	regenArgs  []string // `evolve <args>` that regenerates the projection from source
	ssotPrefix string   // repo-relative source-of-truth prefix; a diff touching it means the projection drifted
}

// derivedArtifacts is the single classifier for GENERATED projections that a flag
// cycle edits indirectly (via the registry) and that must be regenerated from the
// merged source — NEVER hand-maintained by the LLM builder (cycle-11 H1). It feeds
// BOTH the post-build normalizer (normalizeDerivedProjections — deterministic regen
// before audit, like build-gofmt) and the fleet rebase recovery
// (rebaseWithDerivedRegen — auto-resolve a rebase conflict confined to these paths).
//
// control-flags.md is the flag registry's projection (cmd_flags.go: `evolve flags
// generate` splices flagregistry.RenderIndex() into a marker region). Every
// flag-reduction cycle rewrites its whole marker region, so the projection drifts
// unless regenerated; regeneration IS the projection re-run (single-source-with-
// projection: no second renderer).
//
// Keep in sync with TestDerivedArtifacts_MapIntegrity (each key is a real on-disk
// file carrying a GENERATED marker; each ssotPrefix is a real source path).
var derivedArtifacts = map[string]derivedArtifactSpec{
	"docs/architecture/control-flags.md": {
		regenArgs:  []string{"flags", "generate"},
		ssotPrefix: "go/internal/flagregistry/",
	},
}

// isDerivedArtifact reports whether a repo-relative path is a registered,
// auto-resolvable GENERATED projection.
func isDerivedArtifact(relPath string) bool {
	_, ok := derivedArtifacts[relPath]
	return ok
}

// regenStaleProjections regenerates + stages each derived projection whose SSOT
// was changed (derivedProjectionsForChanges(changed)). Best-effort per projection:
// a regen or stage failure WARNs and is skipped (the docs/flags gate stays the
// backstop), never aborting the cycle. Returns the projections successfully
// regenerated + staged. regen and stage are injected for testability (see regenFn).
func regenStaleProjections(ctx context.Context, worktree string, changed []string, regen, stage regenFn) []string {
	var done []string
	for _, rel := range derivedProjectionsForChanges(changed) {
		if err := regen(ctx, worktree, rel); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-derived-regen: %s: %v; docs gate remains the backstop\n", rel, err)
			continue
		}
		if err := stage(ctx, worktree, rel); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN build-derived-regen: stage %s: %v\n", rel, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] build-derived-regen: regenerated %s from its source-of-truth before audit\n", rel)
		done = append(done, rel)
	}
	return done
}

// stageWorktreePath force-stages relPath in the worktree so audit's `git diff
// HEAD` and the shipped tree include a regenerated projection. It is the
// production `stage` argument to regenStaleProjections.
func stageWorktreePath(ctx context.Context, worktree, relPath string) error {
	_, code, err := gitCapture(ctx, worktree, "add", "--", relPath)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("git add %s: exit %d", relPath, code)
	}
	return nil
}

// derivedProjectionsForChanges returns the registered projection paths whose
// source-of-truth was touched by changedPaths (so the projection is now stale and
// must be regenerated). Pure + deterministic (sorted) — the gate consulted by
// normalizeDerivedProjections so a cycle that did not edit a projection's SSOT
// pays no `go run` cost.
func derivedProjectionsForChanges(changedPaths []string) []string {
	var stale []string
	for path, spec := range derivedArtifacts {
		for _, c := range changedPaths {
			if strings.HasPrefix(c, spec.ssotPrefix) {
				stale = append(stale, path)
				break
			}
		}
	}
	sort.Strings(stale)
	return stale
}

// maxRebaseContinueSteps bounds the replay loop so a pathological rebase can never
// spin forever (each cycle branch has only a handful of commits).
const maxRebaseContinueSteps = 100

// rebaseCycleBranchOntoMain rebases the cycle's worktree branch onto the current
// main so a fleet cycle whose ff-merge diverged (a peer moved main) can re-audit
// + re-ship the merged tree (ADR-0049 S5b). Returns ok=true on a clean replay OR
// when every conflict was confined to derived projections that were regenerated
// from the merged source: the re-audit re-binds the regenerated tree, so the
// ship-time tree-SHA binding (ship/gitops.go) still holds — integrity-safe. A
// conflict touching any NON-derived path (genuine overlapping work, incl. the
// SSOT itself) returns conflict=true → the debugger. Infra failures and failed
// regenerations return (false,false). The in-progress rebase is always aborted on
// a non-ok return so the worktree is left clean. An empty worktree returns
// (false,false) — a degraded run never rebases.
func rebaseCycleBranchOntoMain(ctx context.Context, worktree string) (ok bool, conflict bool) {
	if worktree == "" {
		return false, false
	}
	return rebaseWithDerivedRegen(ctx, worktree, gitCapture, regenerateDerivedArtifact, isDerivedArtifact)
}

// rebaseWithDerivedRegen is the testable core of the fleet rebase recovery (Humble
// Object): pure orchestration over an injected git runner + regenerator. See
// rebaseCycleBranchOntoMain for the contract.
func rebaseWithDerivedRegen(ctx context.Context, worktree string, git gitFn, regen regenFn, isDerived func(string) bool) (ok bool, conflict bool) {
	// abort cleans up an in-progress rebase. It runs under a DETACHED context so a
	// cancelled ctx (per-cycle deadline / SIGINT mid-rebase) can never leave the
	// worktree half-rebased — the work calls below use ctx (cancellation interrupts
	// the actual rebase), but the cleanup must still complete.
	abort := func() {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _, _ = git(cctx, worktree, "rebase", "--abort")
	}
	if _, exit, err := git(ctx, worktree, "rebase", "main"); err == nil && exit == 0 {
		return true, false
	}
	// The rebase paused or failed. A genuine conflict leaves unmerged paths; none
	// means an infra failure (not a conflict) — abort and let the cycle fail.
	if len(unmergedRebasePaths(ctx, worktree, git)) == 0 {
		abort()
		fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s onto main failed without conflicts (infra); aborted\n", worktree)
		return false, false
	}
	for step := 0; step < maxRebaseContinueSteps; step++ {
		unmerged := unmergedRebasePaths(ctx, worktree, git)
		if len(unmerged) == 0 {
			// A replayed commit became empty after resolution (a peer already made the
			// same change) → skip it; the rebase proceeds to the next commit.
			if _, c, e := git(ctx, worktree, "rebase", "--skip"); e == nil && c == 0 {
				return true, false
			}
			continue
		}
		// Classify the WHOLE set before touching anything: if any conflict is on a
		// non-derived path we abort without regenerating its derived siblings, since
		// the rebase aborts anyway and a partial regen would be wasted.
		for _, p := range unmerged {
			if !isDerived(p) {
				abort()
				fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: non-derived conflict on %s (overlapping work) → debugger\n", worktree, p)
				return false, true
			}
		}
		for _, p := range unmerged {
			if rerr := regen(ctx, worktree, p); rerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: regenerate %s failed: %v; aborting\n", worktree, p, rerr)
				abort()
				return false, false
			}
			if _, c, e := git(ctx, worktree, "add", "--", p); e != nil || c != 0 {
				fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: git add %s failed (rc=%d, err=%v); aborting\n", worktree, p, c, e)
				abort()
				return false, false
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s: regenerated derived projection %s from merged source\n", worktree, p)
		}
		// core.editor=true makes `--continue`'s commit-message edit a no-op (reuse the
		// replayed message) instead of blocking on an interactive editor. A non-zero
		// `--continue` means a SUBSEQUENT replayed commit conflicts (or emptied) — the
		// loop re-classifies it; exit 0 means the entire replay finished.
		if _, c, e := git(ctx, worktree, "-c", "core.editor=true", "rebase", "--continue"); e == nil && c == 0 {
			return true, false
		}
	}
	abort()
	fmt.Fprintf(os.Stderr, "[orchestrator] fleet rebase of %s exceeded %d replay steps; aborted\n", worktree, maxRebaseContinueSteps)
	return false, false
}

// unmergedRebasePaths returns the repo-relative paths with conflict markers
// (--diff-filter=U) during an in-progress rebase, one per line. The outer
// TrimSpace drops the trailing newline; git path entries carry no interior
// whitespace, so no per-line trimming is needed.
func unmergedRebasePaths(ctx context.Context, worktree string, git gitFn) []string {
	out, _, _ := git(ctx, worktree, "diff", "--name-only", "--diff-filter=U")
	var paths []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l != "" {
			paths = append(paths, l)
		}
	}
	return paths
}

// regenerateDerivedArtifact re-projects the derived artifact identified by relPath
// from the worktree's MERGED source by compiling + running the registered `evolve`
// projection subcommand. The projection writes via EVOLVE_WORKTREE_ROOT (NOT via
// worktree/relPath directly) — relPath is the classifier/registry key. It builds
// from source (`go run`) deliberately: the running campaign binary's
// flagregistry.All is the pre-campaign flag set and would re-inflate the doc.
// EVOLVE_WORKTREE_ROOT points sourceRoot() at the worktree so the projection
// writes the worktree's copy (precedence WORKTREE>PROJECT>cwd, cmd_subagent.go).
func regenerateDerivedArtifact(ctx context.Context, worktree, relPath string) error {
	spec, ok := derivedArtifacts[relPath]
	if !ok {
		return fmt.Errorf("no regenerator registered for %q", relPath)
	}
	goArgs := append([]string{"run", "./cmd/evolve"}, spec.regenArgs...)
	cmd := exec.CommandContext(ctx, "go", goArgs...)
	cmd.Dir = filepath.Join(worktree, "go")
	cmd.Env = append(os.Environ(), ipcenv.WorktreeRootKey+"="+worktree)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("regenerate %s via `evolve %s`: %w: %s", relPath, strings.Join(spec.regenArgs, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// decideAfterDebugger maps the debugger phase's recovery decision (surfaced on
// PhaseResponse.Signals by the debugger runner) to the next phase, mirroring
// decideAfterRetro. RESHIP→ship; RERUN_PHASE→the named phase (defaulting to
// audit); BLOCK/empty/unknown→end. A malformed decision already safe-defaulted
// to BLOCK in the debugger's Classify, so this conservatively ends on anything
// not explicitly RESHIP/RERUN_PHASE.
