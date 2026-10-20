// Package treefence is a content fence around a read-only phase's worktree.
// See docs/architecture/packages/internal-treefence.md.
package treefence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type Snapshot struct {
	Worktree string
	Tree     string
	mode     snapshotMode
	writable map[string]bool
}

type snapshotMode uint8

const (
	allFiles snapshotMode = iota
	declaredFiles
)

func TakeStaged(ctx context.Context, worktree string, pathspec []string) (Snapshot, error) {
	if strings.TrimSpace(worktree) == "" {
		return Snapshot{}, fmt.Errorf("treefence: worktree path required")
	}
	tree, err := indexCopyTree(ctx, worktree, declaredFiles, pathspec)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Worktree: worktree, Tree: tree, mode: declaredFiles}, nil
}

type Result struct {
	Restored []string
	Kept     []string
}

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

func (s Snapshot) Restore(ctx context.Context) (Result, error) {
	if s.mode != allFiles {
		return Result{}, errNotRestorable
	}
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
	added, keptAdded := s.partition(added)
	changed, keptChanged := s.partition(changed)
	var errs []error
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
	kept := append(keptAdded, keptChanged...)
	sort.Strings(kept)
	return Result{Restored: restored, Kept: kept}, errors.Join(errs...)
}

func (s Snapshot) Differing(ctx context.Context, base Snapshot) ([]string, error) {
	if s.Tree == base.Tree {
		return nil, nil
	}
	added, changed, err := base.diff(ctx, s.Tree)
	if err != nil {
		return nil, err
	}
	paths := append(added, changed...)
	sort.Strings(paths)
	return paths, nil
}

func (s Snapshot) partition(paths []string) (fenced, kept []string) {
	for _, rel := range paths {
		if s.writable[rel] {
			kept = append(kept, rel)
		} else {
			fenced = append(fenced, rel)
		}
	}
	return fenced, kept
}

func (s Snapshot) verify(ctx context.Context) error {
	tree, err := writeTree(ctx, s.Worktree)
	if err != nil || tree == s.Tree {
		return err
	}
	added, changed, err := s.diff(ctx, tree)
	if err != nil {
		return err
	}
	for _, rel := range append(added, changed...) {
		if !s.writable[rel] {
			return fmt.Errorf("treefence: restored content differs from dispatch snapshot")
		}
	}
	return nil
}

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

func pruneEmptyParents(root, dir string) {
	root, dir = filepath.Clean(root), filepath.Clean(dir)
	for dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func writeTree(ctx context.Context, worktree string) (string, error) {
	return indexCopyTree(ctx, worktree, allFiles, nil)
}

var errNotRestorable = errors.New("treefence: only a full snapshot restores a worktree")

func indexCopyTree(ctx context.Context, worktree string, mode snapshotMode, declared []string) (string, error) {
	tree, err := gitexec.Isolated(worktree).TreeOfIndexCopy(ctx, addArgs(mode, declared)...)
	if err != nil {
		return "", fmt.Errorf("treefence: snapshot tree: %w", err)
	}
	return tree, nil
}

func addArgs(mode snapshotMode, declared []string) [][]string {
	switch {
	case mode == declaredFiles && len(declared) == 0:
		return [][]string{{"add", "-u", "--", "."}}
	case mode == declaredFiles:
		return [][]string{append([]string{"add", "-A", "--"}, declared...), {"add", "-u", "--", "."}}
	default:
		return [][]string{{"add", "-A", "--", "."}}
	}
}

func writeFromTree(ctx context.Context, worktree, tree, rel, abs string) error {
	entry, err := git(ctx, worktree, nil, "ls-tree", "-z", tree, "--", rel)
	if err != nil {
		return err
	}
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

func gitEnv(extra []string) []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		if k, _, _ := strings.Cut(kv, "="); slices.Contains(gitexec.AmbientGitVars, k) {
			continue
		}
		env = append(env, kv)
	}
	return append(env, extra...)
}

func git(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := sysexec.Command(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("treefence: git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

type Fence struct {
	snap    *Snapshot
	takeErr error
}

func Begin(ctx context.Context, worktree string, readOnly bool, writable ...string) *Fence {
	if !readOnly || worktree == "" {
		return &Fence{}
	}
	snap, err := Take(ctx, worktree)
	if err != nil {
		snap, err = retake(ctx, worktree, err)
	}
	if err != nil {
		return &Fence{takeErr: err}
	}
	snap.writable = writableSet(writable)
	return &Fence{snap: &snap}
}

const retakeDelay = 100 * time.Millisecond

func retake(ctx context.Context, worktree string, first error) (Snapshot, error) {
	select {
	case <-ctx.Done():
		return Snapshot{}, fmt.Errorf("%w; retry: %w", first, ctx.Err())
	case <-time.After(retakeDelay):
	}
	snap, err := Take(ctx, worktree)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w; retry: %w", first, err)
	}
	return snap, nil
}

func writableSet(paths []string) map[string]bool {
	if len(paths) == 0 {
		return nil
	}
	set := make(map[string]bool, len(paths))
	for _, rel := range paths {
		set[filepath.ToSlash(rel)] = true
	}
	return set
}

type Outcome struct {
	Verified   bool
	Restored   []string
	Kept       []string
	TakeErr    error
	RestoreErr error
}

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
	if err == nil {
		err = f.snap.verify(ctx)
	}
	return Outcome{Verified: err == nil, Restored: res.Restored, Kept: res.Kept, RestoreErr: err}
}

const listMax = 12

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
	if len(o.Kept) > 0 {
		out = append(out, cyclestate.Diagnostic{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: phase %s kept its writes to the %d path(s) this dispatch may write: %s",
			phase, len(o.Kept), strings.Join(o.Kept, ", "))})
	}
	if o.RestoreErr != nil {
		out = append(out, cyclestate.Diagnostic{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: restore failed for read-only phase %s (%v) — the worktree may carry this phase's writes", phase, o.RestoreErr)})
	}
	return out
}

func (f *Fence) TakeErr() error {
	if f == nil {
		return nil
	}
	return f.takeErr
}
