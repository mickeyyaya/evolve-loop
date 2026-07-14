// gitops.go — atomic commit + (ff-merge if worktree) + push + optional gh release.
//
// Mirrors ship.sh section 7 (lines 578-907) and section 8 (909-939):
//
//   - Branch detection (refuse detached HEAD)
//   - Worktree-aware ship (when cycle-state.json:active_worktree set + class=cycle):
//     commit in worktree, pre-merge tree-SHA check, ff-merge into main, push,
//     post-push tree-SHA verification, ship-binding.json sidecar
//   - Non-worktree path: git add -A → diff-footer-append → commit-prefix-gate
//     → commit → push
//   - EVOLVE_SHIP_RELEASE_NOTES → gh release create
package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitprefixgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/versionbump"
)

// acquireShipLock acquires the ADR-0049 S5 integrator lock (gap G1): the
// BLOCKING flock serializing the shared-main integration critical section so
// two concurrent ships can't corrupt main's index/ref/origin. nil seam →
// flock.Lock on <ProjectRoot>/.evolve/ship.lock. No-op under the whole-cycle
// project lock (uncontended); load-bearing once that lock is scoped per-run.
func (o *Options) acquireShipLock() (release func(), err error) {
	p := flock.ShipLockPath(o.ProjectRoot)
	if o.shipLock != nil {
		return o.shipLock(p)
	}
	return flock.Lock(p)
}

// atomicShip is the single entry point for "do the actual git work."
// Returns nil on success (commit + push completed, or DryRun skipped them).
// Returns *IntegrityError on tree-SHA binding mismatch.
func atomicShip(ctx context.Context, opts *Options, res *RunResult) error {
	// Branch detection — refuse detached HEAD.
	branch, err := currentBranch(ctx, opts)
	if err != nil {
		return err
	}
	if branch == "" {
		return shipErr(core.CodeGitDetachedHead, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: detached HEAD — refuse to ship; checkout a branch first")
	}

	// Decide worktree path: only for --class cycle with active_worktree set.
	if opts.Class == ClassCycle {
		if wt := readActiveWorktree(opts); wt != "" && wt != opts.ProjectRoot {
			if _, err := os.Stat(wt); err == nil {
				return shipFromWorktree(ctx, opts, res, branch, wt)
			}
		}
	}

	return shipDirect(ctx, opts, res, branch)
}

// detectColliders returns the sorted list of paths that are incoming from the
// worktree (commits branch..cycleBranch + worktree status) AND exist UNTRACKED
// in the main working tree — the files a ff-merge would refuse to overwrite.
// Shared by the shipFromWorktree pre-flight and the collider repair
// (repair.go) so both always see the same list.
func detectColliders(ctx context.Context, opts *Options, worktree, branch, cycleBranch string) ([]string, error) {
	incomingFiles := make(map[string]bool)

	// 1. Files modified in commits branch..cycleBranch
	diffOut, err := captureGitOutputAtDir(ctx, opts, worktree, "diff", "--name-only", branch, cycleBranch)
	if err == nil {
		for _, line := range strings.Split(diffOut, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				incomingFiles[line] = true
			}
		}
	}

	// 2. Files in worktree status (modified, added, untracked, staged)
	statusOut, err := captureGitOutputAtDir(ctx, opts, worktree, "status", "--porcelain")
	if err == nil {
		for _, line := range strings.Split(statusOut, "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 3 {
				status := line[:2]
				if !strings.Contains(status, "D") { // Skip deleted files
					path := line[3:]
					if strings.HasPrefix(path, "\"") && strings.HasSuffix(path, "\"") {
						path = path[1 : len(path)-1]
					}
					incomingFiles[path] = true
				}
			}
		}
	}

	// Expand directories in incomingFiles
	expandedIncomingFiles := make(map[string]bool)
	for p := range incomingFiles {
		wtFilePath := filepath.Join(worktree, p)
		info, err := os.Stat(wtFilePath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			// Best-effort expansion: per-entry Walk errors are tolerated (callback returns nil).
			_ = filepath.Walk(wtFilePath, func(path string, walkInfo os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if !walkInfo.IsDir() {
					rel, relErr := filepath.Rel(worktree, path)
					if relErr == nil {
						expandedIncomingFiles[rel] = true
					}
				}
				return nil
			})
		} else {
			expandedIncomingFiles[p] = true
		}
	}

	var colliders []string
	for p := range expandedIncomingFiles {
		wtFilePath := filepath.Join(worktree, p)
		if _, err := os.Stat(wtFilePath); err != nil {
			continue // Does not exist in worktree
		}
		mainFilePath := filepath.Join(opts.ProjectRoot, p)
		if _, err := os.Stat(mainFilePath); err != nil {
			continue // Does not exist on main side
		}
		// Check if it is untracked in main repo
		mainTracked, err := captureGitOutput(ctx, opts, "ls-files", p)
		if err != nil {
			continue
		}
		if strings.TrimSpace(mainTracked) == "" {
			colliders = append(colliders, p)
		}
	}
	sort.Strings(colliders) // deterministic error messages + manifest order
	return colliders, nil
}

// readActiveWorktree extracts cycle-state.json:active_worktree. Empty
// when absent — caller should fall through to the direct ship path.
func readActiveWorktree(opts *Options) string {
	csMap, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return ""
	}
	return stateString(csMap, "active_worktree")
}

// cycleStateFile returns the file ship reads run-defining inputs (active_worktree,
// cycle_id) from. It prefers the per-run run.json mirror under the run workspace
// (ADR-0049 S3 / gap G3) — a full cycle-state.json mirror (CB.4, only
// CycleState-modeled keys) — so a concurrent cycle's host-global cycle-state.json
// cannot make ship integrate the WRONG run's worktree/number. Falls back to the
// global file when WorkspacePath is unset (standalone `evolve ship`) or the
// mirror is absent. A no-op for the live loop: with one cycle running, run.json
// and the global file hold identical content. (cycle_size_estimate is NOT a
// CycleState field, so verifyTrivial — a standalone-only path — keeps reading the
// global file directly.)
func (o *Options) cycleStateFile() string {
	if o.WorkspacePath != "" {
		runJSON := filepath.Join(o.WorkspacePath, core.RunStateFile)
		if _, err := os.Stat(runJSON); err == nil {
			return runJSON
		}
	}
	return filepath.Join(o.ProjectRoot, ".evolve", "cycle-state.json")
}

// shipDirect: the non-worktree path.
//
//	git add -A
//	check for staged changes (else exit 0)
//	build actual-diff footer (cycle/manual only)
//	run commit-prefix-gate (best-effort; missing is OK)
//	git commit -m <msg-with-footer>
//	git push origin <branch>
func shipDirect(ctx context.Context, opts *Options, res *RunResult, branch string) error {
	// ADR-0049 S5 / gap G1: the non-worktree ship path (manual ships, release
	// ships, and any cycle ship without a live worktree) mutates
	// opts.ProjectRoot's index directly via add -A → commit → push. Hold the
	// integrator lock across that whole critical section so two concurrent
	// ships that both land here serialize instead of racing main's
	// index/ref/origin — the same guard shipFromWorktree already holds.
	// BLOCKING flock; skipped on dry-run (mutates nothing). No-op under the
	// whole-cycle project lock (uncontended).
	if !opts.DryRun {
		release, lockErr := opts.acquireShipLock()
		if lockErr != nil {
			return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
				"ship: acquire integrator lock (.evolve/ship.lock): "+lockErr.Error())
		}
		defer release()
	}

	if !opts.DryRun {
		if opts.Class == ClassRelease {
			// Release staging is class-special (v18.3.0→v18.5.0 forensics):
			// the pipeline's rebuild-binary step is the AUDITED producer of
			// go/evolve, so the churn discard below would throw away the
			// release's own product (→ SELF_SHA_TAMPERED on the next ship);
			// and `add -A` swept untracked operator files (evolve.log,
			// release-*.log) into release commits. Stage exactly the known
			// release set instead.
			if err := stageReleaseSet(ctx, opts); err != nil {
				return err
			}
		} else {
			_ = discardBinaryChurn(ctx, opts, opts.ProjectRoot)
			exit, err := opts.run(ctx, "git", []string{"add", "-A"}, io.Discard, opts.Stderr)
			if err != nil || exit != 0 {
				return shipErr(core.CodeGitStageFailed, core.ShipClassTransient, core.StageAtomicShip,
					fmt.Sprintf("ship: git add -A failed (rc=%d): %v", exit, err),
					"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err))
			}
		}
	}

	// Backstop against accidental compiled-binary commits (tracked-binary-in-
	// acs-dir): after staging, before the commit, refuse any staged oversized
	// executable outside the go/bin//go/evolve allowlist.
	if !opts.DryRun {
		if err := stageBinaryGuard(ctx, opts); err != nil {
			return err
		}
	}

	// Check for staged changes. git diff --cached --quiet exits 0 if no
	// diff, 1 if diff. (We use io.Discard for stdout — there's no output.)
	exit, err := opts.run(ctx, "git", []string{"diff", "--cached", "--quiet"}, io.Discard, io.Discard)
	if err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: git diff --cached --quiet failed: "+err.Error(), "git_err", err.Error())
	}
	if exit == 0 {
		res.Logs = append(res.Logs, "[ship] no staged changes to ship; exiting cleanly (audit was for an empty diff)")
		return nil
	}

	// Build commit message with actual-diff footer (cycle/manual only).
	msg := opts.CommitMessage
	if opts.Class == ClassCycle || opts.Class == ClassManual {
		footer, err := buildDiffFooter(ctx, opts)
		if err != nil {
			return err
		}
		msg = msg + footer
	}
	// Reviewed-by trailer (manual class only) — durable per-commit record of
	// who reviewed before commit, derived from the verified attestation.
	msg += reviewedByTrailer(opts)

	// Optional: commit-prefix-gate (Layer 1 of ADR-0012). Best-effort
	// shellout to the bash gate when present; missing or non-executable
	// is silently skipped to match bash behavior (`if [ -x ... ]`).
	if err := runCommitPrefixGate(ctx, opts, msg, opts.ProjectRoot); err != nil {
		return shipErr(core.CodeCommitPrefixGate, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: commit-prefix-gate rejected main-path commit (Layer 1 of ADR-0012). To bypass for manual class only: --bypass-prefix-gate: "+err.Error(),
			"gate_err", err.Error())
	}

	if opts.DryRun {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] [DRY-RUN] would commit + push to %s", branch))
		return nil
	}

	// git commit -m <msg>
	exit, err = opts.run(ctx, "git", []string{"commit", "-m", msg}, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitCommitFailed, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: git commit failed (rc=%d): %v", exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "branch", branch)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: committed to %s", branch))

	// git push origin <branch> — a rejection gets ONE inline fetch+ff-retry
	// (repairPushRace); a diverged origin reclassifies to needs-reaudit.
	exit, err = opts.run(ctx, "git", []string{"push", "origin", branch}, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		origErr := shipErr(core.CodeGitPushRejected, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git push failed (rc=%d): %v", exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "branch", branch)
		if rerr := repairPushRace(ctx, opts, res, branch, origErr); rerr != nil {
			return rerr
		}
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: pushed to origin/%s", branch))

	// Record HEAD SHA for the result struct.
	headSHA, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	res.CommitSHA = strings.TrimSpace(headSHA)

	// Optional GitHub release.
	return maybeCreateRelease(ctx, opts, res)
}

// shipFromWorktree: the v8.43.0 worktree-aware path. Commit in the
// cycle's worktree (where Builder's edits live), pre-merge tree-SHA
// check, ff-merge cycle branch into main, push main, post-push
// integrity verification, ship-binding.json sidecar.
func shipFromWorktree(ctx context.Context, opts *Options, res *RunResult, branch, worktree string) error {
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] v8.43.0: worktree-aware ship — committing in active_worktree=%s", worktree))

	cycleBranch, err := captureGitOutputAtDir(ctx, opts, worktree, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return shipErr(core.CodeWorktreeResolve, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: could not resolve cycle branch from worktree %s: %v", worktree, err),
			"worktree", worktree, "git_err", err.Error())
	}
	cycleBranch = strings.TrimSpace(cycleBranch)
	if cycleBranch == "" {
		return shipErr(core.CodeWorktreeResolve, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: empty cycle branch from worktree "+worktree, "worktree", worktree)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship]   cycle branch: %s", cycleBranch))

	// ADR-0049 S5 / gap G1: hold the integrator lock across the entire
	// shared-main critical section that follows — the collider scan (which
	// reads main's working tree, a TOCTOU with the merge it feeds), the
	// go/evolve discard, the ff-merge, the push, and the post-push tree
	// verification. flock is BLOCKING; two concurrent ships serialize here
	// instead of racing main's index/ref/origin. Skipped on dry-run (mutates
	// nothing). No-op under the whole-cycle project lock (uncontended); the
	// keystone of floor-removal once that lock is scoped per-run.
	if !opts.DryRun {
		release, lockErr := opts.acquireShipLock()
		if lockErr != nil {
			return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
				"ship: acquire integrator lock (.evolve/ship.lock): "+lockErr.Error())
		}
		defer release()
	}

	// Collider pre-flight check (v12.2 / Task 2). detectColliders is the
	// single source of truth — the repair ladder (repair.go) re-derives the
	// same list when healing this refusal.
	colliders, err := detectColliders(ctx, opts, worktree, branch, cycleBranch)
	if err != nil {
		return err
	}
	if len(colliders) > 0 {
		return shipErr(core.CodeGitFFMergeDiverged, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: untracked files in main working tree would be overwritten by merge: %s", strings.Join(colliders, ", ")),
			"colliders", strings.Join(colliders, ","))
	}

	// Ship-bind manifest reconciliation (cycle-653 second seam, shadow):
	// report any path about to be bound that no phase report declared.
	reconcileManifestShadow(ctx, opts, res, worktree, branch, cycleBranch)

	if !opts.DryRun {
		_ = discardBinaryChurn(ctx, opts, worktree)
		exit, err := opts.run(ctx, "git", []string{"-C", worktree, "add", "-A"}, io.Discard, opts.Stderr)
		if err != nil || exit != 0 {
			return shipErr(core.CodeGitStageFailed, core.ShipClassTransient, core.StageAtomicShip,
				fmt.Sprintf("ship: worktree git add -A failed (rc=%d): %v", exit, err),
				"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "worktree", worktree)
		}
	}

	// Check staged changes in worktree.
	exit, err := opts.run(ctx, "git", []string{"-C", worktree, "diff", "--cached", "--quiet"}, io.Discard, io.Discard)
	if err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: worktree diff --cached --quiet failed: "+err.Error(), "git_err", err.Error(), "worktree", worktree)
	}
	worktreeCleanNoCommit := exit == 0
	if worktreeCleanNoCommit {
		// Check if branch is ahead of main.
		out, err := captureGitOutput(ctx, opts, "rev-list", "--count", branch+".."+cycleBranch)
		if err != nil {
			return err
		}
		ahead := strings.TrimSpace(out)
		if ahead == "0" || ahead == "" {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] no changes in worktree AND branch not ahead of %s; exiting cleanly", branch))
			return nil
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship]   no uncommitted worktree changes but branch is %s commit(s) ahead; will merge", ahead))
	}

	if !worktreeCleanNoCommit {
		// Build worktree-aware commit message with footer.
		footer, err := buildDiffFooterAtDir(ctx, opts, worktree)
		if err != nil {
			return err
		}
		// No Reviewed-by trailer here: this worktree path is --class cycle only
		// (gated above), and the trailer is manual-class only.
		msg := opts.CommitMessage + footer

		if err := runCommitPrefixGate(ctx, opts, msg, worktree); err != nil {
			return shipErr(core.CodeCommitPrefixGate, core.ShipClassPrecondition, core.StageAtomicShip,
				"ship: commit-prefix-gate rejected worktree commit (Layer 1 of ADR-0012). To bypass for manual class only: --bypass-prefix-gate: "+err.Error(),
				"gate_err", err.Error(), "worktree", worktree)
		}

		// ADR-0048 Slice C1 (verify-before-mutate): check the audit-bound tree-SHA
		// binding against the STAGED INDEX via `git write-tree` BEFORE the commit.
		// write-tree's SHA is exactly the tree a commit from this index would carry,
		// so this is equivalent to the prior post-commit `HEAD^{tree}` check — minus
		// the orphan commit + `reset --soft` rollback. A mismatch now refuses with no
		// commit object ever created, closing the commit-then-fail-verify window.
		// Runs only when an audit binding is set; skipped on dry-run, which never
		// stages the index (the `git add -A` above is itself dry-run-guarded), so
		// write-tree would report a stale tree. The post-push tree assertion below
		// remains as the transactional post-condition. Fails CLOSED: a set binding
		// that cannot be verified aborts rather than shipping unverified work.
		if !opts.DryRun && opts.internalAuditBoundTreeSHA != "" {
			stagedTreeSHA, err := captureGitOutputAtDir(ctx, opts, worktree, "write-tree")
			if err != nil {
				return err
			}
			stagedTreeSHA = strings.TrimSpace(stagedTreeSHA)
			if stagedTreeSHA == "" {
				return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
					"ship: git write-tree produced no tree SHA — cannot verify audit-bound tree binding before commit",
					"worktree", worktree)
			}
			if opts.internalAuditBoundTreeSHA != stagedTreeSHA {
				return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip,
					fmt.Sprintf("INTEGRITY BREACH (pre-commit): audit-bound tree SHA %s != staged tree SHA %s — refused to commit; worktree changes preserved (staged) for operator triage",
						opts.internalAuditBoundTreeSHA, stagedTreeSHA),
					"audit_bound_tree", opts.internalAuditBoundTreeSHA, "worktree_tree", stagedTreeSHA, "phase", "pre-commit")
			}
			res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: pre-commit tree-SHA binding verified (audit=%s staged=%s)", opts.internalAuditBoundTreeSHA, stagedTreeSHA))
		}

		if opts.DryRun {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would commit in worktree on %s", cycleBranch))
		} else {
			exit, err := opts.run(ctx, "git", []string{"-C", worktree, "-c", "commit.gpgsign=false", "commit", "-m", msg},
				opts.Stdout, opts.Stderr)
			if err != nil || exit != 0 {
				return shipErr(core.CodeGitCommitFailed, core.ShipClassPrecondition, core.StageAtomicShip,
					fmt.Sprintf("ship: git commit in worktree failed (rc=%d): %v", exit, err),
					"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "worktree", worktree)
			}
			res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: committed in worktree on %s", cycleBranch))
		}
	}

	if opts.DryRun {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would ff-merge + push %s into %s", cycleBranch, branch))
		return nil
	}

	// The release-managed binary go/evolve is regenerated by `go build` (the
	// release rebuild step and ordinary dev/test builds) and is frequently left
	// dirty in the MAIN working tree. A ff-merge does a checkout-style working-
	// tree update and REFUSES to overwrite a dirty tracked file, so build-artifact
	// drift in go/evolve blocks an otherwise-valid cycle merge (cycle-153). Its
	// working-tree state during a cycle is meaningless — the cycle's real work
	// lives in the worktree, and go/evolve is re-committed only by
	// `evolve ship --class release`. Reset it to HEAD so the merge isn't blocked.
	// Best-effort + path-scoped: a no-op when go/evolve is clean or absent (e.g.
	// a vendored deployment), so it cannot discard real cycle work. A non-zero
	// exit is surfaced as a WARN (not fatal) so an operator sees why the ff-merge
	// may still trip on go/evolve in a non-standard layout (e.g. a symlink).
	if ce, cerr := opts.run(ctx, "git", []string{"checkout", "HEAD", "--", "go/evolve"}, io.Discard, io.Discard); ce != 0 || cerr != nil {
		fmt.Fprintf(opts.Stderr, "[ship] WARN: could not reset go/evolve to HEAD (exit=%d, err=%v); ff-merge may still fail if it is dirty\n", ce, cerr)
	}

	// ff-merge cycle branch into main.
	exit, err = opts.run(ctx, "git", []string{"merge", "--ff-only", cycleBranch}, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		// ADR-0049 S5b: under fleet mode a ff-merge divergence is EXPECTED — a
		// peer cycle moved main while this cycle was mid-pipeline (the colliders
		// were already cleared above, so this is the moved-main case). Signal the
		// orchestrator to rebase onto the new main and re-verify the merged tree
		// (test-the-merged-tree) rather than aborting: a transient
		// GIT_FLEET_REBASE_NEEDED, not the terminal GIT_FF_MERGE_DIVERGED. The
		// sequential loop is unaffected (it never has a moving main mid-cycle).
		if opts.envBool(ipcenv.FleetKey) {
			return shipErr(core.CodeGitFleetRebaseNeeded, core.ShipClassTransient, core.StageAtomicShip,
				fmt.Sprintf("ship: fleet ff-merge %s into %s diverged (a peer cycle moved %s mid-pipeline); rebase + re-verify the merged tree, then re-ship", cycleBranch, branch, branch),
				"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "cycle_branch", cycleBranch, "branch", branch)
		}
		return shipErr(core.CodeGitFFMergeDiverged, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: ff-merge %s into %s failed (rc=%d; divergent history): %v", cycleBranch, branch, exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "cycle_branch", cycleBranch, "branch", branch)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: ff-merged %s into %s", cycleBranch, branch))

	// Push — same inline fetch+ff-retry policy as shipDirect; the healed
	// path falls through to the post-push tree verification + sidecar.
	exit, err = opts.run(ctx, "git", []string{"push", "origin", branch}, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		headOut, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
		origErr := shipErr(core.CodeGitPushRejected, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git push failed (rc=%d); main is at %s: %v", exit, strings.TrimSpace(headOut), err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "branch", branch, "head", strings.TrimSpace(headOut))
		if rerr := repairPushRace(ctx, opts, res, branch, origErr); rerr != nil {
			return rerr
		}
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: pushed to origin/%s", branch))

	headSHA, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	res.CommitSHA = strings.TrimSpace(headSHA)

	// Post-push tree-SHA verification.
	committedTree, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD^{tree}")
	committedTree = strings.TrimSpace(committedTree)
	if opts.internalAuditBoundTreeSHA != "" && committedTree != "" {
		if opts.internalAuditBoundTreeSHA != committedTree {
			return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StagePostShip,
				fmt.Sprintf("INTEGRITY BREACH: audit-bound tree SHA %s != committed tree SHA %s — worktree-to-main tree drift detected", opts.internalAuditBoundTreeSHA, committedTree),
				"audit_bound_tree", opts.internalAuditBoundTreeSHA, "committed_tree", committedTree, "phase", "post-push")
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: tree-SHA binding verified (audit=%s committed=%s)", opts.internalAuditBoundTreeSHA, committedTree))
	}

	// ship-binding.json sidecar.
	if err := writeShipBinding(opts, committedTree, headSHA); err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: could not write ship-binding.json: "+err.Error())
	}

	return maybeCreateRelease(ctx, opts, res)
}

// writeShipBinding emits .evolve/runs/cycle-<N>/ship-binding.json for
// post-ship audit. Best-effort; failure is a WARN, not a ship failure.
func writeShipBinding(opts *Options, committedTree, commitSHA string) error {
	csPath := opts.cycleStateFile() // ADR-0049 S3 / G3: run-scoped (cycle_id)
	csMap, err := readStateMap(csPath)
	if err != nil {
		return err
	}
	cid, ok := stateInt(csMap, "cycle_id")
	if !ok {
		return errors.New("no cycle_id in cycle-state.json")
	}
	dir := filepath.Join(opts.ProjectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "ship-binding.json")
	body := map[string]any{
		"audit_bound_tree_sha": opts.internalAuditBoundTreeSHA,
		"tree_sha_committed":   committedTree,
		"commit_sha":           strings.TrimSpace(commitSHA),
		"cycle":                cid,
	}
	buf, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "ship-binding.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(buf, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// currentBranch returns the short ref name (e.g. "main") or "" when detached.
func currentBranch(ctx context.Context, opts *Options) (string, error) {
	var buf strings.Builder
	exit, err := opts.run(ctx, "git", []string{"symbolic-ref", "--short", "HEAD"}, &buf, io.Discard)
	if err != nil {
		return "", fmt.Errorf("ship: symbolic-ref --short HEAD: %w", err)
	}
	if exit != 0 {
		return "", nil // detached HEAD — caller checks
	}
	return strings.TrimSpace(buf.String()), nil
}

// buildDiffFooter computes the actual-diff footer appended to commit
// messages for cycle/manual classes. Mirrors ship.sh's footer text
// byte-for-byte so reviewers and Test U pass.
func buildDiffFooter(ctx context.Context, opts *Options) (string, error) {
	return buildDiffFooterAtDir(ctx, opts, opts.ProjectRoot)
}

func buildDiffFooterAtDir(ctx context.Context, opts *Options, cwd string) (string, error) {
	var files, shortStat strings.Builder
	args1 := []string{"diff", "--cached", "--name-status"}
	if cwd != opts.ProjectRoot {
		args1 = append([]string{"-C", cwd}, args1...)
	}
	if _, err := opts.run(ctx, "git", args1, &files, io.Discard); err != nil {
		return "", fmt.Errorf("ship: diff --name-status: %w", err)
	}
	args2 := []string{"diff", "--cached", "--shortstat"}
	if cwd != opts.ProjectRoot {
		args2 = append([]string{"-C", cwd}, args2...)
	}
	if _, err := opts.run(ctx, "git", args2, &shortStat, io.Discard); err != nil {
		return "", fmt.Errorf("ship: diff --shortstat: %w", err)
	}

	filesStr := strings.TrimRight(files.String(), "\n")
	if filesStr == "" {
		return "", nil
	}
	lines := strings.Split(filesStr, "\n")
	prefixed := make([]string, 0, len(lines))
	for _, l := range lines {
		prefixed = append(prefixed, "- "+l)
	}
	footer := fmt.Sprintf("\n\n---\n## Actual diff (v8.34.0+)\n\nFiles modified (%d):\n%s\n\n%s",
		len(lines), strings.Join(prefixed, "\n"), strings.TrimRight(shortStat.String(), "\n"))
	return footer, nil
}

// runCommitPrefixGate calls the commitprefixgate Go library directly
// (v11.8.2+; prior versions shelled out to legacy/scripts/guards/
// commit-prefix-gate.sh). Missing manifest is silently passed through by
// the library (matches the bash "pass-through when not provisioned" rule).
func runCommitPrefixGate(ctx context.Context, opts *Options, msg, repoDir string) error {
	_, err := commitprefixgate.Run(commitprefixgate.Options{
		CommitMsg: msg,
		RepoDir:   repoDir,
		Mode:      commitprefixgate.ModeStaged,
		Stderr:    opts.Stderr,
		Bypass:    opts.BypassPrefixGate,
		ShipClass: string(opts.Class),
	})
	return err
}

// maybeCreateRelease runs `gh release create v<VERSION>` when
// EVOLVE_SHIP_RELEASE_NOTES is set. Best-effort: a missing gh CLI or a
// non-zero exit logs WARN and continues (release may already exist).
func maybeCreateRelease(ctx context.Context, opts *Options, res *RunResult) error {
	// SSOT IPC-protocol-allowed: releasepipeline -> ship subprocess (reader side;
	// writer is releasepipeline.go). Not an operator dial.
	notes := opts.envStr("EVOLVE_" + "SHIP_RELEASE_NOTES")
	if notes == "" {
		return nil
	}
	pj := filepath.Join(opts.PluginRoot, ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(pj)
	if err != nil {
		res.Logs = append(res.Logs, "[ship] WARN: no .claude-plugin/plugin.json — skipping release")
		return nil
	}
	var p struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || p.Version == "" {
		res.Logs = append(res.Logs, "[ship] WARN: cannot parse plugin.json:version — skipping release")
		return nil
	}
	tag := "v" + p.Version

	if opts.DryRun {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] [DRY-RUN] would create GitHub release %s", tag))
		return nil
	}

	res.Logs = append(res.Logs, fmt.Sprintf("[ship] creating GitHub release %s...", tag))
	exit, err := opts.runStdin(ctx, "gh", []string{"release", "create", tag, "--title", tag, "--notes-file", "-"},
		strings.NewReader(notes), opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] WARN: gh release create failed (release may already exist; rc=%d)", exit))
		return nil
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: GitHub release %s created", tag))
	return nil
}

// captureGitOutputAtDir is captureGitOutput with -C <dir> prefix.
func captureGitOutputAtDir(ctx context.Context, opts *Options, dir string, args ...string) (string, error) {
	all := append([]string{"-C", dir}, args...)
	return captureGitOutput(ctx, opts, all...)
}

// stageReleaseSet stages the explicit release pathspec: the versionbump
// marker files (SSOT: versionbump.DefaultPaths — the writer the release
// pipeline's version-bump step runs), CHANGELOG.md (changelog-gen's output),
// and the tracked binary go/evolve (+ ShipBinaryPath when it differs) that
// rebuild-binary produces. Paths absent on disk are skipped so a partial
// layout (tests, exotic repos) never fails staging on a nonexistent pathspec.
func stageReleaseSet(ctx context.Context, opts *Options) error {
	// versionbump.Paths.Files() is the SSOT for the marker files the version-bump
	// step writes; consuming it (not a hand-listed subset) means a newly added
	// marker — e.g. .codex-plugin/plugin.json — is staged automatically and can
	// never be committed one version behind the rest of the release.
	vb := versionbump.DefaultPaths(opts.ProjectRoot)
	abs := append(vb.Files(),
		filepath.Join(opts.ProjectRoot, "CHANGELOG.md"),
		filepath.Join(opts.ProjectRoot, "go", "evolve"),
	)
	if p := opts.ShipBinaryPath; p != "" {
		if rel, err := filepath.Rel(opts.ProjectRoot, p); err == nil && !strings.HasPrefix(rel, "..") {
			abs = append(abs, p)
		}
	}
	args := []string{"add", "--"}
	seen := map[string]struct{}{}
	for _, a := range abs {
		rel, err := filepath.Rel(opts.ProjectRoot, a)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if _, dup := seen[rel]; dup {
			continue
		}
		if _, err := os.Stat(a); err != nil {
			continue // absent on disk — nothing to stage
		}
		seen[rel] = struct{}{}
		args = append(args, rel)
	}
	if len(args) == 2 {
		return nil // nothing exists to stage; the staged-diff check decides
	}
	exit, err := opts.run(ctx, "git", args, io.Discard, opts.Stderr)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitStageFailed, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git add (release set) failed (rc=%d): %v", exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err))
	}
	return nil
}

// discardBinaryChurn discards unaudited tracked-binary rebuild churn from the
// WORKTREE before `git add -A` stages the ship commit. It deliberately uses
// `git checkout -- <path>` (restore from INDEX), not `git checkout HEAD -- <path>`:
// after normalizeWorktreeToBase's `git reset --soft`, the index holds the full
// audited diff, so the index — not HEAD — is the audited reference. Restoring
// from the index discards exactly the post-audit worktree churn (e.g. an ACS
// rerun rebuilding go/evolve) while preserving an audited, intentionally staged
// binary update. Contrast with core.discardMainLeak, which runs mid-cycle on the
// MAIN tree where the cycle is not yet committed and HEAD is the audited
// reference — there `HEAD --` is correct. The two forms are not interchangeable.
// osExecutable is a test seam for the running-binary lookup (production =
// os.Executable). The churn discard must never delete the binary it is
// executing from — inbox ship-manual-deletes-running-binary, 2026-07-12
// incidents: `ship --class manual` resolved the untracked go/bin/evolve via
// this fallback and os.Remove'd it, degrading every kernel hook to the stale
// tracked fallback until rebuild.
var osExecutable = os.Executable

// isRunningExecutable reports whether path is the currently-executing binary,
// resolving symlinks best-effort on both sides.
func isRunningExecutable(path string) bool {
	exe, err := osExecutable()
	if err != nil {
		return false
	}
	resolvedPath, errP := filepath.EvalSymlinks(path)
	resolvedExe, errE := filepath.EvalSymlinks(exe)
	if errP != nil || errE != nil {
		return path == exe
	}
	return resolvedPath == resolvedExe
}

func discardBinaryChurn(ctx context.Context, opts *Options, dir string) error {
	binPath := opts.ShipBinaryPath
	if binPath == "" {
		if execPath, err := osExecutable(); err == nil {
			binPath = execPath
		}
	}
	var relBin string
	if binPath != "" {
		if rel, err := filepath.Rel(opts.ProjectRoot, binPath); err == nil && !strings.HasPrefix(rel, "..") {
			relBin = filepath.ToSlash(rel)
		}
	}

	// Always discard the standard production path "go/evolve" as well as relBin
	pathsToDiscard := []string{"go/evolve"}
	if relBin != "" && relBin != "go/evolve" {
		pathsToDiscard = append(pathsToDiscard, relBin)
	}

	for _, p := range pathsToDiscard {
		absPath := filepath.Join(dir, p)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			continue // doesn't exist, skip
		}
		// Check if it is tracked in the repository context of dir
		var buf strings.Builder
		exitCode, err := opts.run(ctx, "git", []string{"-C", dir, "ls-files", p}, &buf, io.Discard)
		if err == nil && exitCode == 0 && strings.TrimSpace(buf.String()) != "" {
			// Revert the changes
			_, _ = opts.run(ctx, "git", []string{"-C", dir, "checkout", "--", p}, io.Discard, io.Discard)
		} else {
			// Untracked: remove the file — unless it is the currently-executing
			// binary. An untracked go/bin/evolve is gitignored (go/.gitignore
			// `/bin/`), so `git add -A` can never stage it; removing it has zero
			// staging-hygiene value and kills the binary the kernel hooks and the
			// rollback shellout are running (2026-07-12 incidents, cycle-243).
			if isRunningExecutable(absPath) {
				if opts.Stderr != nil {
					fmt.Fprintf(opts.Stderr,
						"[ship] WARN: churn discard skipped %s — it is the currently-executing binary\n", p)
				}
				continue
			}
			_ = os.Remove(absPath)
		}
	}
	return nil
}
