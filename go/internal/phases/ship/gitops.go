package ship

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/commitprefixgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
	"github.com/mickeyyaya/evolve-loop/go/internal/versionbump"
)

func (o *Options) acquireShipLock() (release func(), err error) {
	p := flock.ShipLockPath(o.ProjectRoot)
	if o.shipLock != nil {
		return o.shipLock(p)
	}
	return flock.Lock(p)
}

func atomicShip(ctx context.Context, opts *Options, res *RunResult) error {
	tree, fromWorktree, err := landingTree(opts)
	if err != nil {
		return err
	}

	branch, err := currentBranch(ctx, opts)
	if err != nil {
		return err
	}
	if branch == "" {
		return shipErr(core.CodeGitDetachedHead, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: detached HEAD — refuse to ship; checkout a branch first")
	}

	if fromWorktree {
		return shipFromWorktree(ctx, opts, res, branch, tree)
	}
	return shipDirect(ctx, opts, res, branch)
}

func landingTree(opts *Options) (tree string, fromWorktree bool, err error) {
	worktree, typed, err := activeWorktreeForShip(opts)
	if err != nil {
		return "", false, shipErr(core.CodeWorktreeResolve, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: resolve host-bound active worktree: "+err.Error())
	}
	if opts.Class != ClassCycle || worktree == "" || worktree == opts.ProjectRoot {
		return opts.ProjectRoot, false, nil
	}
	info, statErr := os.Stat(worktree)
	if statErr == nil && info.IsDir() {
		return worktree, true, nil
	}
	if typed {
		if statErr == nil {
			statErr = fmt.Errorf("not a directory")
		}
		return "", false, shipErr(core.CodeWorktreeResolve, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: host-bound active worktree %s is unavailable: %v", worktree, statErr),
			"worktree", worktree)
	}
	return opts.ProjectRoot, false, nil
}

func activeWorktreeForShip(opts *Options) (worktree string, typed bool, err error) {
	if opts.Class != ClassCycle || opts.ActiveWorktree == "" {
		return readActiveWorktree(opts), false, nil
	}
	if opts.WorkspacePath != "" {
		runState := filepath.Join(opts.WorkspacePath, core.RunStateFile)
		if _, statErr := os.Stat(runState); statErr == nil {
			state, readErr := readStateMap(runState)
			if readErr != nil {
				return "", true, fmt.Errorf("read run.json mirror: %w", readErr)
			}
			mirrored := stateString(state, "active_worktree")
			if mirrored != opts.ActiveWorktree {
				return "", true, fmt.Errorf("run.json active_worktree %q does not match host identity %q", mirrored, opts.ActiveWorktree)
			}
		} else if !os.IsNotExist(statErr) {
			return "", true, fmt.Errorf("inspect run.json mirror: %w", statErr)
		}
	}
	return opts.ActiveWorktree, true, nil
}

func detectColliders(ctx context.Context, opts *Options, worktree, branch, cycleBranch string) ([]string, error) {
	incomingFiles := make(map[string]bool)

	diffOut, err := captureGitOutputAtDir(ctx, opts, worktree, shipmanifest.RawPathRead("diff", "--name-only", branch, cycleBranch)...)
	if err == nil {
		for _, line := range strings.Split(diffOut, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				incomingFiles[shipmanifest.UnquoteGitPath(line)] = true
			}
		}
	}

	statusOut, err := captureGitOutputAtDir(ctx, opts, worktree, shipmanifest.RawPathRead("status", "--porcelain")...)
	if err == nil {
		for _, line := range strings.Split(statusOut, "\n") {
			line = strings.TrimSpace(line)
			if len(line) > 3 {
				status := line[:2]
				if !strings.Contains(status, "D") {
					path := shipmanifest.UnquoteGitPath(line[3:])
					incomingFiles[path] = true
				}
			}
		}
	}

	expandedIncomingFiles := make(map[string]bool)
	for p := range incomingFiles {
		wtFilePath := filepath.Join(worktree, p)
		info, err := os.Stat(wtFilePath)
		if err != nil {
			continue
		}
		if info.IsDir() {
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
			continue
		}
		mainFilePath := filepath.Join(opts.ProjectRoot, p)
		if _, err := os.Stat(mainFilePath); err != nil {
			continue
		}
		mainTracked, err := captureGitOutput(ctx, opts, "ls-files", p)
		if err != nil {
			continue
		}
		if strings.TrimSpace(mainTracked) == "" {
			colliders = append(colliders, p)
		}
	}
	sort.Strings(colliders)
	return colliders, nil
}

func readActiveWorktree(opts *Options) string {
	csMap, err := readStateMap(opts.cycleStateFile())
	if err != nil {
		return ""
	}
	return stateString(csMap, "active_worktree")
}

func (o *Options) cycleStateFile() string {
	if o.WorkspacePath != "" {
		runJSON := filepath.Join(o.WorkspacePath, core.RunStateFile)
		if _, err := os.Stat(runJSON); err == nil {
			return runJSON
		}
	}
	return filepath.Join(o.ProjectRoot, ".evolve", "cycle-state.json")
}

func shipDirect(ctx context.Context, opts *Options, res *RunResult, branch string) error {
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
			if err := stageReleaseSet(ctx, opts); err != nil {
				return err
			}
		} else {
			_ = discardBinaryChurn(ctx, opts, opts.ProjectRoot)
			consumeCommittedItems(ctx, opts, res, "")
			if err := stageExplicitPaths(ctx, opts, res, ""); err != nil {
				return err
			}
		}
	}

	if !opts.DryRun {
		if err := stageBinaryGuard(ctx, opts); err != nil {
			return err
		}
		if err := refuseCommittedDocDeletion(ctx, opts, "--cached"); err != nil {
			return err
		}
	}

	exit, err := opts.run(ctx, "git", []string{"diff", "--cached", "--quiet"}, io.Discard, io.Discard)
	if err != nil {
		return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
			"ship: git diff --cached --quiet failed: "+err.Error(), "git_err", err.Error())
	}
	nothingStaged := exit == 0
	if nothingStaged {
		res.Logs = append(res.Logs, "[ship] no staged changes to ship; exiting cleanly (audit was for an empty diff)")
		return nil
	}

	msg := opts.CommitMessage
	if opts.Class == ClassCycle || opts.Class == ClassManual {
		footer, err := buildDiffFooter(ctx, opts)
		if err != nil {
			return err
		}
		msg = msg + footer
	}
	msg += reviewTrailer(opts)

	if err := runCommitPrefixGate(ctx, opts, msg, opts.ProjectRoot); err != nil {
		return shipErr(core.CodeCommitPrefixGate, core.ShipClassPrecondition, core.StageAtomicShip,
			"ship: commit-prefix-gate rejected main-path commit (Layer 1 of ADR-0012). To bypass for manual class only: --bypass-prefix-gate: "+err.Error(),
			"gate_err", err.Error())
	}

	if !opts.DryRun && opts.internalAuditBoundTreeSHA != "" {
		stagedTree, terr := captureGitOutput(ctx, opts, "write-tree")
		if terr != nil {
			return terr
		}
		stagedTree = strings.TrimSpace(stagedTree)
		if stagedTree == "" {
			return shipErr(core.CodeGitIO, core.ShipClassTransient, core.StageAtomicShip,
				"ship: git write-tree produced no tree SHA - cannot verify audit-bound tree binding pre-commit")
		}
		ok, offending := auditBindingSatisfied(ctx, opts, "", stagedTree)
		if !ok {
			return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StageAtomicShip,
				fmt.Sprintf("INTEGRITY BREACH (pre-commit): audit-bound tree SHA %s != staged tree SHA %s - refused; staged changes preserved for operator triage%s",
					opts.internalAuditBoundTreeSHA, stagedTree, offending),
				"audit_bound_tree", opts.internalAuditBoundTreeSHA, "staged_tree", stagedTree, "phase", "pre-commit")
		}
		res.Logs = append(res.Logs, fmt.Sprintf("[ship]   OK: pre-commit tree-SHA binding verified (audit=%s staged=%s)", opts.internalAuditBoundTreeSHA, stagedTree))
	}

	if opts.DryRun {
		res.Logs = append(res.Logs, fmt.Sprintf("[ship] [DRY-RUN] would commit + push to %s", branch))
		return nil
	}

	exit, err = opts.run(ctx, "git", []string{"commit", "-m", msg}, opts.Stdout, opts.Stderr)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitCommitFailed, core.ShipClassPrecondition, core.StageAtomicShip,
			fmt.Sprintf("ship: git commit failed (rc=%d): %v", exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err), "branch", branch)
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: committed to %s", branch))

	if err := pushWithRepair(ctx, opts, res, landing.PushRequest{Branch: branch, Site: landing.SiteDirect}); err != nil {
		return err
	}
	res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: pushed to origin/%s", branch))
	releaseLandedBindings(ctx, opts, res, "HEAD")

	if opts.internalAuditBoundTreeSHA != "" {
		committedTree, _ := captureGitOutput(ctx, opts, "rev-parse", "HEAD^{tree}")
		committedTree = strings.TrimSpace(committedTree)
		if committedTree != "" {
			ok, offending := auditBindingSatisfied(ctx, opts, "", committedTree)
			if !ok {
				return shipErr(core.CodeIntegrityTreeDrift, core.ShipClassIntegrity, core.StagePostShip,
					fmt.Sprintf("INTEGRITY BREACH: audit-bound tree SHA %s != committed tree SHA %s - main-path tree drift detected%s",
						opts.internalAuditBoundTreeSHA, committedTree, offending),
					"audit_bound_tree", opts.internalAuditBoundTreeSHA, "committed_tree", committedTree, "phase", "post-push")
			}
			res.Logs = append(res.Logs, fmt.Sprintf("[ship] OK: tree-SHA binding verified (audit=%s committed=%s)", opts.internalAuditBoundTreeSHA, committedTree))
		}
	}

	return maybeCreateRelease(ctx, opts, res)
}

func shipFromWorktree(ctx context.Context, opts *Options, res *RunResult, branch, worktree string) error {
	return newWorktreeShip(ctx, opts, res, branch, worktree).run()
}

func currentBranch(ctx context.Context, opts *Options) (string, error) {
	var buf strings.Builder
	exit, err := opts.run(ctx, "git", []string{"symbolic-ref", "--short", "HEAD"}, &buf, io.Discard)
	if err != nil {
		return "", fmt.Errorf("ship: symbolic-ref --short HEAD: %w", err)
	}
	if exit != 0 {
		return "", nil
	}
	return strings.TrimSpace(buf.String()), nil
}

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

func captureGitOutputAtDir(ctx context.Context, opts *Options, dir string, args ...string) (string, error) {
	all := append([]string{"-C", dir}, args...)
	return captureGitOutput(ctx, opts, all...)
}

func stageExplicitPaths(ctx context.Context, opts *Options, res *RunResult, dir string) error {
	root, prefix := dir, []string{"-C", dir}
	if dir == "" {
		root, prefix = opts.ProjectRoot, nil
	}
	read := func(args ...string) (string, error) { return captureGitOutputAtDir(ctx, opts, root, args...) }
	sel, err := shipmanifest.Select(read, root, opts.WorkspacePath)
	if err != nil {
		return err
	}
	logIgnored(res, sel)
	add := &explicitAdd{ctx: ctx, opts: opts, prefix: prefix}
	paths, refused, err := shipmanifest.StageRetrying(sel.Paths, add.attempt)
	if len(refused) > 0 {
		res.Logs = append(res.Logs, fmt.Sprintf(
			"[ship] git refused %d gitignored pathspec(s) the check-ignore probe cannot see (directory-form rules); dropped and retried: %s",
			len(sel.Paths)-len(paths), strings.Join(refused, " ")))
	}
	if err != nil {
		return add.failure(dir, paths)
	}
	clearStageRefusal(opts.WorkspacePath)
	res.Logs = append(res.Logs, fmt.Sprintf(
		"[ship] staged %d explicit path(s) (declared manifest=%d, changed=%d) — no `git add -A`",
		len(paths), len(sel.Manifest), sel.Changed))
	return nil
}

func logIgnored(res *RunResult, sel shipmanifest.Selection) {
	switch {
	case sel.ProbeErr != nil:
		res.Logs = append(res.Logs, fmt.Sprintf(
			"[ship] WARN: check-ignore probe failed (%v) — staging the full declared set", sel.ProbeErr))
	case len(sel.Ignored) > 0:
		res.Logs = append(res.Logs, fmt.Sprintf(
			"[ship] dropped %d gitignored declared path(s) from staging: %s",
			len(sel.Ignored), strings.Join(sel.Ignored, " ")))
	}
}

type explicitAdd struct {
	ctx     context.Context
	opts    *Options
	prefix  []string
	errTail bytes.Buffer
	exit    int
	runErr  error
}

func (a *explicitAdd) attempt(paths []string) (string, error) {
	a.errTail.Reset()
	stderr := io.Writer(&a.errTail)
	if a.opts.Stderr != nil {
		stderr = io.MultiWriter(a.opts.Stderr, &a.errTail)
	}
	args := append(append(append([]string{}, a.prefix...), "add", "-A", "--"), paths...)
	a.exit, a.runErr = a.opts.run(a.ctx, "git", args, io.Discard, stderr)
	switch {
	case a.runErr != nil:
		return "", a.runErr
	case a.exit != 0:
		return a.errTail.String(), fmt.Errorf("git add exited %d", a.exit)
	}
	return "", nil
}

func (a *explicitAdd) failure(dir string, paths []string) error {
	gitStderr := a.errTail.String()
	tail := strings.TrimSpace(gitStderr)
	if len(tail) > 300 {
		tail = "…" + tail[len(tail)-300:]
	}
	class := core.ShipClassTransient
	deterministic := a.exit == 128 && (strings.Contains(gitStderr, "fatal: Invalid path ") ||
		strings.Contains(gitStderr, " is outside repository at ") ||
		strings.Contains(gitStderr, "fatal: pathspec ") && strings.Contains(gitStderr, " did not match any files")) ||
		a.exit == 1 && strings.Contains(gitStderr, "The following paths are ignored by one of your .gitignore files:")
	if deterministic || recordStageRefusal(a.opts.WorkspacePath, paths) {
		class = core.ShipClassPrecondition
	}
	return shipErr(core.CodeGitStageFailed, class, core.StageAtomicShip,
		fmt.Sprintf("ship: git add failed (rc=%d): %v: %s", a.exit, a.runErr, tail),
		"git_rc", fmt.Sprintf("%d", a.exit), "git_err", errStr(a.runErr), "git_stderr", tail, "worktree", dir)
}

const stageRefusalMemoFile = "ship-stage-refusal.txt"

func stageRefusalKey(paths []string) string {
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\n")
}

func recordStageRefusal(workspace string, paths []string) bool {
	if workspace == "" {
		return false
	}
	memo := filepath.Join(workspace, stageRefusalMemoFile)
	key := stageRefusalKey(paths)
	prev, err := os.ReadFile(memo)
	_ = os.WriteFile(memo, []byte(key), 0o644)
	return err == nil && string(prev) == key
}

func clearStageRefusal(workspace string) {
	if workspace == "" {
		return
	}
	_ = os.Remove(filepath.Join(workspace, stageRefusalMemoFile))
}

func stageReleaseSet(ctx context.Context, opts *Options) error {
	vb := versionbump.DefaultPaths(opts.ProjectRoot)
	abs := append(vb.Files(),
		filepath.Join(opts.ProjectRoot, "CHANGELOG.md"),
		filepath.Join(opts.ProjectRoot, "go", "evolve"),
	)
	if p := opts.ShipBinaryPath; p != "" {
		if rel, err := opts.rel(opts.ProjectRoot, p); err == nil && !strings.HasPrefix(rel, "..") {
			abs = append(abs, p)
		}
	}
	args := []string{"add", "--"}
	seen := map[string]struct{}{}
	for _, a := range abs {
		rel, err := opts.rel(opts.ProjectRoot, a)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if _, dup := seen[rel]; dup {
			continue
		}
		if _, err := os.Stat(a); err != nil {
			continue
		}
		seen[rel] = struct{}{}
		args = append(args, rel)
	}
	if len(args) == 2 {
		return nil
	}
	exit, err := opts.run(ctx, "git", args, io.Discard, opts.Stderr)
	if err != nil || exit != 0 {
		return shipErr(core.CodeGitStageFailed, core.ShipClassTransient, core.StageAtomicShip,
			fmt.Sprintf("ship: git add (release set) failed (rc=%d): %v", exit, err),
			"git_rc", fmt.Sprintf("%d", exit), "git_err", errStr(err))
	}
	return nil
}

var osExecutable = os.Executable

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

	pathsToDiscard := []string{"go/evolve"}
	if relBin != "" && relBin != "go/evolve" {
		pathsToDiscard = append(pathsToDiscard, relBin)
	}

	for _, p := range pathsToDiscard {
		absPath := filepath.Join(dir, p)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			continue
		}
		var buf strings.Builder
		exitCode, err := opts.run(ctx, "git", []string{"-C", dir, "ls-files", p}, &buf, io.Discard)
		if err == nil && exitCode == 0 && strings.TrimSpace(buf.String()) != "" {
			_, _ = opts.run(ctx, "git", []string{"-C", dir, "checkout", "--", p}, io.Discard, io.Discard)
		} else {
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
