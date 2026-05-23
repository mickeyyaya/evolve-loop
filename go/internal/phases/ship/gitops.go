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
	"strings"
)

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
		return fmt.Errorf("ship: detached HEAD — refuse to ship; checkout a branch first")
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

// readActiveWorktree extracts cycle-state.json:active_worktree. Empty
// when absent — caller should fall through to the direct ship path.
func readActiveWorktree(opts *Options) string {
	csPath := filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json")
	csMap, err := readStateMap(csPath)
	if err != nil {
		return ""
	}
	return stateString(csMap, "active_worktree")
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
	if !opts.DryRun {
		exit, err := opts.Runner(ctx, "git", []string{"add", "-A"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, opts.Stderr)
		if err != nil || exit != 0 {
			return fmt.Errorf("ship: git add -A failed (rc=%d): %v", exit, err)
		}
	}

	// Check for staged changes. git diff --cached --quiet exits 0 if no
	// diff, 1 if diff. (We use io.Discard for stdout — there's no output.)
	exit, err := opts.Runner(ctx, "git", []string{"diff", "--cached", "--quiet"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("ship: git diff --cached --quiet failed: %w", err)
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

	// Optional: commit-prefix-gate (Layer 1 of ADR-0012). Best-effort
	// shellout to the bash gate when present; missing or non-executable
	// is silently skipped to match bash behavior (`if [ -x ... ]`).
	if err := runCommitPrefixGate(ctx, opts, msg, opts.ProjectRoot); err != nil {
		return fmt.Errorf("ship: commit-prefix-gate rejected main-path commit (Layer 1 of ADR-0012). To bypass for manual class only: EVOLVE_BYPASS_PREFIX_GATE=1 SHIP_CLASS=manual: %w", err)
	}

	if opts.DryRun {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] [DRY-RUN] would commit + push to %s", branch))
		return nil
	}

	// git commit -m <msg>
	exit, err = opts.Runner(ctx, "git", []string{"commit", "-m", msg}, os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		return fmt.Errorf("ship: git commit failed (rc=%d): %v", exit, err)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: committed to %s", branch))

	// git push origin <branch>
	exit, err = opts.Runner(ctx, "git", []string{"push", "origin", branch}, os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		return fmt.Errorf("ship: git push failed (rc=%d): %v", exit, err)
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
		return fmt.Errorf("ship: could not resolve cycle branch from worktree %s: %w", worktree, err)
	}
	cycleBranch = strings.TrimSpace(cycleBranch)
	if cycleBranch == "" {
		return fmt.Errorf("ship: empty cycle branch from worktree %s", worktree)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship]   cycle branch: %s", cycleBranch))

	if !opts.DryRun {
		exit, err := opts.Runner(ctx, "git", []string{"-C", worktree, "add", "-A"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, opts.Stderr)
		if err != nil || exit != 0 {
			return fmt.Errorf("ship: worktree git add -A failed (rc=%d): %v", exit, err)
		}
	}

	// Check staged changes in worktree.
	exit, err := opts.Runner(ctx, "git", []string{"-C", worktree, "diff", "--cached", "--quiet"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("ship: worktree diff --cached --quiet failed: %w", err)
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
		msg := opts.CommitMessage + footer

		if err := runCommitPrefixGate(ctx, opts, msg, worktree); err != nil {
			return fmt.Errorf("ship: commit-prefix-gate rejected worktree commit (Layer 1 of ADR-0012). To bypass for manual class only: EVOLVE_BYPASS_PREFIX_GATE=1 SHIP_CLASS=manual: %w", err)
		}

		if opts.DryRun {
			res.Logs = append(res.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would commit in worktree on %s", cycleBranch))
		} else {
			exit, err := opts.Runner(ctx, "git", []string{"-C", worktree, "-c", "commit.gpgsign=false", "commit", "-m", msg},
				os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
			if err != nil || exit != 0 {
				return fmt.Errorf("ship: git commit in worktree failed (rc=%d): %v", exit, err)
			}
			res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: committed in worktree on %s", cycleBranch))

			// v10.15.0: pre-merge tree-SHA binding check.
			wtTreeSHA, err := captureGitOutputAtDir(ctx, opts, worktree, "rev-parse", "HEAD^{tree}")
			if err != nil {
				return err
			}
			wtTreeSHA = strings.TrimSpace(wtTreeSHA)
			if opts.internalAuditBoundTreeSHA != "" && wtTreeSHA != "" {
				if opts.internalAuditBoundTreeSHA != wtTreeSHA {
					// Rollback worktree.
					_, _ = opts.Runner(ctx, "git", []string{"-C", worktree, "reset", "--soft", "HEAD~1"}, os.Environ(), opts.ProjectRoot, nil, io.Discard, io.Discard)
					return &IntegrityError{
						Msg: fmt.Sprintf("INTEGRITY BREACH (pre-merge): audit-bound tree SHA %s != worktree-commit tree SHA — refused to ff-merge; worktree rolled back to staged state for operator triage",
							opts.internalAuditBoundTreeSHA),
					}
				}
				res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: pre-merge tree-SHA binding verified (audit=%s worktree=%s)", opts.internalAuditBoundTreeSHA, wtTreeSHA))
			}
		}
	}

	if opts.DryRun {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship]   [DRY-RUN] would ff-merge + push %s into %s", cycleBranch, branch))
		return nil
	}

	// ff-merge cycle branch into main.
	exit, err = opts.Runner(ctx, "git", []string{"merge", "--ff-only", cycleBranch}, os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		return fmt.Errorf("ship: ff-merge %s into %s failed (rc=%d; divergent history): %v", cycleBranch, branch, exit, err)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: ff-merged %s into %s", cycleBranch, branch))

	// Push.
	exit, err = opts.Runner(ctx, "git", []string{"push", "origin", branch}, os.Environ(), opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		headOut, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
		return fmt.Errorf("ship: git push failed (rc=%d); main is at %s: %v", exit, strings.TrimSpace(headOut), err)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: pushed to origin/%s", branch))

	headSHA, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD")
	res.CommitSHA = strings.TrimSpace(headSHA)

	// Post-push tree-SHA verification.
	committedTree, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD^{tree}")
	committedTree = strings.TrimSpace(committedTree)
	if opts.internalAuditBoundTreeSHA != "" && committedTree != "" {
		if opts.internalAuditBoundTreeSHA != committedTree {
			return &IntegrityError{
				Msg: fmt.Sprintf("INTEGRITY BREACH: audit-bound tree SHA %s != committed tree SHA %s — worktree-to-main tree drift detected", opts.internalAuditBoundTreeSHA, committedTree),
			}
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
	csPath := filepath.Join(opts.ProjectRoot, ".evolve", "cycle-state.json")
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
	exit, err := opts.Runner(ctx, "git", []string{"symbolic-ref", "--short", "HEAD"}, os.Environ(), opts.ProjectRoot, nil, &buf, io.Discard)
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
	if _, err := opts.Runner(ctx, "git", args1, os.Environ(), opts.ProjectRoot, nil, &files, io.Discard); err != nil {
		return "", fmt.Errorf("ship: diff --name-status: %w", err)
	}
	args2 := []string{"diff", "--cached", "--shortstat"}
	if cwd != opts.ProjectRoot {
		args2 = append([]string{"-C", cwd}, args2...)
	}
	if _, err := opts.Runner(ctx, "git", args2, os.Environ(), opts.ProjectRoot, nil, &shortStat, io.Discard); err != nil {
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

// runCommitPrefixGate is a best-effort shellout to commit-prefix-gate.sh
// when present. Missing or non-executable is silently skipped (matches
// `if [ -x ... ]`).
func runCommitPrefixGate(ctx context.Context, opts *Options, msg, repoDir string) error {
	gatePath := filepath.Join(opts.ProjectRoot, "legacy", "scripts", "guards", "commit-prefix-gate.sh")
	fi, err := os.Stat(gatePath)
	if err != nil || fi.Mode()&0o111 == 0 {
		return nil // missing or non-executable — skip
	}
	args := []string{gatePath, "--msg", msg}
	if repoDir != opts.ProjectRoot {
		args = append(args, "--repo-dir", repoDir)
	}
	env := append(os.Environ(), fmt.Sprintf("SHIP_CLASS=%s", opts.Class))
	exit, err := opts.Runner(ctx, "bash", args, env, opts.ProjectRoot, nil, opts.Stdout, opts.Stderr)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("commit-prefix-gate exit=%d", exit)
	}
	return nil
}

// maybeCreateRelease runs `gh release create v<VERSION>` when
// EVOLVE_SHIP_RELEASE_NOTES is set. Best-effort: a missing gh CLI or a
// non-zero exit logs WARN and continues (release may already exist).
func maybeCreateRelease(ctx context.Context, opts *Options, res *RunResult) error {
	notes := opts.envStr("EVOLVE_SHIP_RELEASE_NOTES")
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
	exit, err := opts.Runner(ctx, "gh", []string{"release", "create", tag, "--title", tag, "--notes-file", "-"},
		os.Environ(), opts.ProjectRoot, strings.NewReader(notes), opts.Stdout, opts.Stderr)
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
