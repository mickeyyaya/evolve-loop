// Package treefence is a content fence around a read-only phase's worktree.
//
// A phase that is not a declared source writer (registry writes_source=false:
// audit, adversarial-review, retrospective, …) must hand downstream the exact
// tree it was given. The agents behind those phases have a shell, and cycles
// 1603, 1604 and 1605 each failed the deterministic build-explanation gate
// because the auditor's mutation probes rewrote the builder's material files
// in place (`cp /tmp/digest.go.bak …`, "wrote reverted tdd.go", probe tests
// dropped into the material packages) — the sealed diff digest no longer
// matched the tree, and no LLM round could repair a defect the auditor itself
// had introduced. See docs/incidents/2026-09-03-auditor-mutates-the-worktree.md.
//
// Take records the worktree as a git tree object — tracked changes and
// untracked files, .gitignore respected — through a throwaway index, so the
// real index is never touched. Restore diffs a fresh tree against it and
// undoes every difference: added paths are removed, modified/deleted/retyped
// paths are written back from the snapshot with their mode, and the
// directories the phase created around its files are pruned. Ignored paths
// (build outputs, the worktree's own .evolve/ scratch) are outside the fence
// by construction — the same set the explanation digest ignores.
package treefence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// Snapshot is the content state of a worktree at Take time.
type Snapshot struct {
	Worktree string
	// Tree is the git tree object id the working tree hashed to.
	Tree string
}

// Result reports what Restore had to undo.
type Result struct {
	// Restored lists the worktree-relative paths written back or removed,
	// sorted. Empty means the phase left the tree byte-identical.
	Restored []string
}

// Take snapshots the worktree. An empty path or a non-repository is an error,
// never a silent no-op — the caller decides how loudly to fail open.
func Take(ctx context.Context, worktree string) (Snapshot, error) {
	if strings.TrimSpace(worktree) == "" {
		return Snapshot{}, fmt.Errorf("treefence: worktree path required")
	}
	tree, err := writeTree(ctx, worktree)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Worktree: worktree, Tree: tree}, nil
}

// Restore puts the worktree back to the snapshot's content and reports the
// paths it touched. A tree that still matches is a no-op. Every entry is
// attempted even when one fails; the failures are joined and returned so the
// caller reports each by name.
func (s Snapshot) Restore(ctx context.Context) (Result, error) {
	after, err := writeTree(ctx, s.Worktree)
	if err != nil {
		return Result{}, err
	}
	if after == s.Tree {
		return Result{}, nil
	}
	added, changed, err := s.diff(ctx, after)
	if err != nil {
		return Result{}, err
	}
	var errs []error
	// Removals first: a path the phase turned from a file into a directory
	// (or the reverse) has to be cleared before its other form is written
	// back, whatever order git listed the two entries in.
	for _, rel := range added {
		if err := removeAdded(s.Worktree, rel); err != nil {
			errs = append(errs, err)
		}
	}
	for _, rel := range changed {
		abs := filepath.Join(s.Worktree, filepath.FromSlash(rel))
		if err := writeFromTree(ctx, s.Worktree, s.Tree, rel, abs); err != nil {
			errs = append(errs, err)
		}
	}
	restored := append(append(make([]string, 0, len(added)+len(changed)), added...), changed...)
	sort.Strings(restored)
	return Result{Restored: restored}, errors.Join(errs...)
}

// diff lists the paths the phase added (to remove) and the paths it modified,
// deleted or retyped (to write back), from `diff-tree -z`; a malformed listing
// is an error, never a silently shortened one.
func (s Snapshot) diff(ctx context.Context, after string) (added, changed []string, err error) {
	out, err := git(ctx, s.Worktree, nil, "diff-tree", "-r", "-z", "--no-renames", "--name-status", s.Tree, after)
	if err != nil {
		return nil, nil, err
	}
	out = strings.TrimSuffix(out, "\x00")
	if out == "" {
		return nil, nil, nil
	}
	fields := strings.Split(out, "\x00")
	if len(fields)%2 != 0 {
		return nil, nil, fmt.Errorf("treefence: malformed diff-tree listing (%d fields)", len(fields))
	}
	for i := 0; i < len(fields); i += 2 {
		status, rel := fields[i], fields[i+1]
		if status == "" || rel == "" {
			return nil, nil, fmt.Errorf("treefence: malformed diff-tree entry %q %q", status, rel)
		}
		if status[0] == 'A' {
			added = append(added, rel)
		} else {
			changed = append(changed, rel)
		}
	}
	return added, changed, nil
}

// removeAdded deletes a path the phase created and prunes the directories it
// created around it. A path that is already gone — or whose parent was
// meanwhile restored to a file — is nothing to do.
func removeAdded(worktree, rel string) error {
	abs := filepath.Join(worktree, filepath.FromSlash(rel))
	if _, err := os.Lstat(abs); err != nil {
		if os.IsNotExist(err) || errors.Is(err, syscall.ENOTDIR) {
			return nil
		}
		return fmt.Errorf("treefence: stat %s: %w", rel, err)
	}
	if err := os.RemoveAll(abs); err != nil {
		return fmt.Errorf("treefence: remove %s: %w", rel, err)
	}
	pruneEmptyParents(worktree, filepath.Dir(abs))
	return nil
}

// pruneEmptyParents removes now-empty directories from dir up to (not
// including) root; a non-empty directory stops the walk.
func pruneEmptyParents(root, dir string) {
	root, dir = filepath.Clean(root), filepath.Clean(dir)
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// writeTree hashes the working tree — tracked changes plus untracked files,
// ignore rules respected — into a tree object via a throwaway index seeded
// from the real one (so only changed paths are re-hashed).
func writeTree(ctx context.Context, worktree string) (string, error) {
	tmp, err := os.MkdirTemp("", "treefence-index-*")
	if err != nil {
		return "", fmt.Errorf("treefence: temp index: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	index := filepath.Join(tmp, "index")
	if real, err := git(ctx, worktree, nil, "rev-parse", "--git-path", "index"); err == nil {
		realPath := strings.TrimSpace(real)
		if !filepath.IsAbs(realPath) {
			realPath = filepath.Join(worktree, realPath)
		}
		if data, rerr := os.ReadFile(realPath); rerr == nil {
			_ = os.WriteFile(index, data, 0o644) // best-effort seed; an empty index only costs time
		}
	}
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := git(ctx, worktree, env, "add", "-A", "--", "."); err != nil {
		return "", err
	}
	out, err := git(ctx, worktree, env, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// writeFromTree restores one path's content and mode from the snapshot tree.
func writeFromTree(ctx context.Context, worktree, tree, rel, abs string) error {
	entry, err := git(ctx, worktree, nil, "ls-tree", "-z", tree, "--", rel)
	if err != nil {
		return err
	}
	// "<mode> <type> <object>\t<path>\0"
	head, _, ok := strings.Cut(strings.TrimSuffix(entry, "\x00"), "\t")
	meta := strings.Fields(head)
	if !ok || len(meta) != 3 {
		return fmt.Errorf("treefence: %s is not in the snapshot tree", rel)
	}
	mode, object := meta[0], meta[2]
	if mode == "160000" {
		return fmt.Errorf("treefence: %s is a submodule gitlink — cannot restore", rel)
	}
	content, err := git(ctx, worktree, nil, "cat-file", "blob", object)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(abs); err != nil {
		return fmt.Errorf("treefence: clear %s: %w", rel, err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("treefence: parent of %s: %w", rel, err)
	}
	if mode == "120000" {
		return os.Symlink(content, abs)
	}
	perm := os.FileMode(0o644)
	if mode == "100755" {
		perm = 0o755
	}
	tmp := abs + ".treefence-tmp"
	if err := os.WriteFile(tmp, []byte(content), perm); err != nil {
		return fmt.Errorf("treefence: write %s: %w", rel, err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("treefence: chmod %s: %w", rel, err)
	}
	return os.Rename(tmp, abs)
}

// ambientGitVars would redirect git away from the worktree the fence was
// given; the fence never trusts the parent process's environment for them.
var ambientGitVars = map[string]bool{"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true}

func gitEnv(extra []string) []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		if k, _, _ := strings.Cut(kv, "="); ambientGitVars[k] {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

func git(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("treefence: git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Fence is the two-step form every dispatcher uses: Begin before the agent
// runs, End after it returns. Both dispatch surfaces that call the bridge —
// the shared phase runner and the retro phase's own runner — hold one so the
// mechanism cannot be wired on one and forgotten on the other.
type Fence struct {
	snap    *Snapshot
	takeErr error
}

// Begin takes the fence for a read-only dispatch. A source writer (readOnly
// false) or a dispatch without a worktree gets an inert fence.
func Begin(ctx context.Context, worktree string, readOnly bool) *Fence {
	if !readOnly || worktree == "" {
		return &Fence{}
	}
	snap, err := Take(ctx, worktree)
	if err != nil {
		return &Fence{takeErr: err}
	}
	return &Fence{snap: &snap}
}

// Outcome is what End did, rendered by Diagnostics for the phase response.
type Outcome struct {
	// Restored lists the paths written back or removed (also on a partial
	// failure — the caller reports both halves).
	Restored []string
	// TakeErr is set when the fence was wanted but could not be taken; the
	// phase ran unfenced.
	TakeErr error
	// RestoreErr is set when one or more entries could not be put back.
	RestoreErr error
}

// End restores the worktree and reports. An inert fence yields an empty
// Outcome.
func (f *Fence) End(ctx context.Context) Outcome {
	if f == nil {
		return Outcome{}
	}
	if f.takeErr != nil {
		return Outcome{TakeErr: f.takeErr}
	}
	if f.snap == nil {
		return Outcome{}
	}
	res, err := f.snap.Restore(ctx)
	return Outcome{Restored: res.Restored, RestoreErr: err}
}

// listMax bounds the restored-path list carried on a diagnostic.
const listMax = 12

// Diagnostics renders the outcome for the phase response: nothing for a clean
// phase; a warning naming the restored paths for a writing one; a warning
// naming the failure when the fence could not do its job. Never an error
// severity — the fence reports, the gates decide.
func (o Outcome) Diagnostics(phase string) []cyclestate.Diagnostic {
	if o.TakeErr != nil {
		return []cyclestate.Diagnostic{{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: snapshot unavailable for read-only phase %s (%v) — worktree writes by this phase were not checked", phase, o.TakeErr)}}
	}
	var out []cyclestate.Diagnostic
	if len(o.Restored) > 0 {
		shown, more := o.Restored, ""
		if len(shown) > listMax {
			more = fmt.Sprintf(" … +%d more", len(shown)-listMax)
			shown = shown[:listMax]
		}
		out = append(out, cyclestate.Diagnostic{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: read-only phase %s wrote %d path(s) into its worktree; restored to the dispatched tree: %s%s",
			phase, len(o.Restored), strings.Join(shown, ", "), more)})
	}
	if o.RestoreErr != nil {
		out = append(out, cyclestate.Diagnostic{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: restore failed for read-only phase %s (%v) — the worktree may carry this phase's writes", phase, o.RestoreErr)})
	}
	return out
}

// TakeErr reports why the fence could not be taken (nil for an inert or a
// live fence), so a dispatcher can log it the moment the phase starts.
func (f *Fence) TakeErr() error {
	if f == nil {
		return nil
	}
	return f.takeErr
}
