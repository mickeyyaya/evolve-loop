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
	dirs     []string
	index    indexState
}

type indexState map[string][]string

func (s Snapshot) absentEntry() string {
	return "0 " + strings.Repeat("0", len(s.Tree)) + " 0"
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
	Created  []string
	Kept     []string
	after    string
}

func Take(ctx context.Context, worktree string) (Snapshot, error) {
	if strings.TrimSpace(worktree) == "" {
		return Snapshot{}, fmt.Errorf("treefence: worktree path required")
	}
	tree, err := writeTree(ctx, worktree)
	if err != nil {
		return Snapshot{}, err
	}
	index, err := readIndex(ctx, worktree)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Worktree: worktree, Tree: tree, index: index}, nil
}

func (s Snapshot) Restore(ctx context.Context) (Result, error) {
	if s.mode != allFiles {
		return Result{}, errNotRestorable
	}
	res, workErr := s.restoreWorkingCopy(ctx)
	indexed, indexErr := s.restoreIndex(ctx)
	res.Restored = mergeSorted(res.Restored, indexed)
	return res, errors.Join(workErr, indexErr)
}

func mergeSorted(a, b []string) []string {
	merged := append(slices.Clone(a), b...)
	sort.Strings(merged)
	return slices.Compact(merged)
}

func readIndex(ctx context.Context, worktree string) (indexState, error) {
	out, err := git(ctx, worktree, nil, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	state := indexState{}
	for _, entry := range strings.Split(out, "\x00") {
		if entry == "" {
			continue
		}
		meta, rel, ok := strings.Cut(entry, "\t")
		if !ok || rel == "" {
			return nil, fmt.Errorf("treefence: malformed index entry %q", entry)
		}
		state[rel] = append(state[rel], meta)
	}
	return state, nil
}

func (s Snapshot) indexDrift(ctx context.Context) ([]string, error) {
	current, err := readIndex(ctx, s.Worktree)
	if err != nil {
		return nil, err
	}
	var drift []string
	for _, state := range []indexState{s.index, current} {
		for rel := range state {
			if !s.writable[rel] && !s.grantsNew(rel, s.index[rel]) && !slices.Equal(s.index[rel], current[rel]) {
				drift = append(drift, rel)
			}
		}
	}
	sort.Strings(drift)
	return slices.Compact(drift), nil
}

func (s Snapshot) restoreIndex(ctx context.Context) ([]string, error) {
	drift, err := s.indexDrift(ctx)
	if err != nil || len(drift) == 0 {
		return nil, err
	}
	var feed strings.Builder
	for _, rel := range drift {
		feed.WriteString(s.absentEntry() + "\t" + rel + "\x00")
	}
	for _, rel := range drift {
		for _, meta := range s.index[rel] {
			feed.WriteString(meta + "\t" + rel + "\x00")
		}
	}
	_, err = gitIn(ctx, s.Worktree, feed.String(), "update-index", "-z", "--index-info")
	return drift, err
}

func (s Snapshot) restoreWorkingCopy(ctx context.Context) (Result, error) {
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
	added, keptAdded := s.partitionAdded(added)
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
	sort.Strings(added)
	return Result{Restored: restored, Created: added, Kept: kept, after: after}, errors.Join(errs...)
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

func (s Snapshot) partitionAdded(paths []string) (fenced, kept []string) {
	for _, rel := range paths {
		if s.writable[rel] || s.inGrantedDir(rel) {
			kept = append(kept, rel)
		} else {
			fenced = append(fenced, rel)
		}
	}
	return fenced, kept
}

func (s Snapshot) inGrantedDir(rel string) bool {
	return slices.ContainsFunc(s.dirs, func(dir string) bool { return strings.HasPrefix(rel, dir+"/") })
}

func (s Snapshot) grantsNew(rel string, indexed []string) bool {
	return len(indexed) == 0 && s.inGrantedDir(rel)
}

func (s Snapshot) verify(ctx context.Context) error {
	if err := s.verifyTree(ctx); err != nil {
		return err
	}
	drift, err := s.indexDrift(ctx)
	if err != nil {
		return err
	}
	if len(drift) > 0 {
		return fmt.Errorf("treefence: restored index differs from dispatch snapshot: %s", strings.Join(drift, ", "))
	}
	return nil
}

func (s Snapshot) verifyTree(ctx context.Context) error {
	tree, err := writeTree(ctx, s.Worktree)
	if err != nil || tree == s.Tree {
		return err
	}
	added, changed, err := s.diff(ctx, tree)
	if err != nil {
		return err
	}
	fencedAdded, _ := s.partitionAdded(added)
	fencedChanged, _ := s.partition(changed)
	if len(fencedAdded)+len(fencedChanged) > 0 {
		return fmt.Errorf("treefence: restored content differs from dispatch snapshot")
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
	return run(ctx, dir, env, "", args...)
}

func gitIn(ctx context.Context, dir, stdin string, args ...string) (string, error) {
	return run(ctx, dir, nil, stdin, args...)
}

func run(ctx context.Context, dir string, env []string, stdin string, args ...string) (string, error) {
	cmd := sysexec.Command(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(env)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("treefence: git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

type Fence struct {
	snap         *Snapshot
	takeErr      error
	retakenAfter error
	workspace    string
}

func (f *Fence) WithWorkspace(workspace string) *Fence {
	if f == nil {
		return nil
	}
	f.workspace = workspace
	return f
}

func (f *Fence) WithWritableDirs(dirs ...string) *Fence {
	if f == nil || f.snap == nil {
		return f
	}
	f.snap.dirs = cleanDirs(dirs)
	return f
}

func cleanDirs(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		if dir = strings.Trim(filepath.ToSlash(filepath.Clean(dir)), "/"); dir != "" && dir != "." {
			out = append(out, dir)
		}
	}
	return out
}

func Begin(ctx context.Context, worktree string, readOnly bool, writable ...string) *Fence {
	if !readOnly || worktree == "" {
		return &Fence{}
	}
	snap, first := Take(ctx, worktree)
	if first != nil {
		var err error
		if snap, err = retake(ctx, worktree, first); err != nil {
			return &Fence{takeErr: err}
		}
	}
	snap.writable = writableSet(writable)
	return &Fence{snap: &snap, retakenAfter: first}
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

const patchFileName = "fence-reverted.patch"

type Outcome struct {
	Verified     bool
	Restored     []string
	Created      []string
	Kept         []string
	Writer       bool
	TakeErr      error
	RestoreErr   error
	RetakenAfter error
	patchPath    string
	patchErr     error
	workspace    string
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
	writer := len(f.snap.writable) > 0 || len(f.snap.dirs) > 0
	out := Outcome{Verified: err == nil, Restored: res.Restored, Created: res.Created, Kept: res.Kept, Writer: writer, RestoreErr: err, RetakenAfter: f.retakenAfter, workspace: f.workspace}
	if len(res.Restored) > 0 && f.workspace != "" && res.after != "" {
		out.savePatch(ctx, f.snap.Worktree, f.snap.Tree, res.after, res.Restored)
	}
	return out
}

func (o *Outcome) savePatch(ctx context.Context, worktree, tree, after string, paths []string) {
	args := append([]string{"diff-tree", "-p", "--binary", tree, after, "--"}, paths...)
	patch, err := git(ctx, worktree, nil, args...)
	if err != nil {
		o.patchErr = err
		return
	}
	if err := os.MkdirAll(o.workspace, 0o755); err != nil {
		o.patchErr = err
		return
	}
	target := filepath.Join(o.workspace, patchFileName)
	if err := os.WriteFile(target, []byte(patch), 0o644); err != nil {
		o.patchErr = err
		return
	}
	o.patchPath = target
}

const listMax = 12

func listed(paths []string) []string {
	if len(paths) <= listMax {
		return paths
	}
	return append(slices.Clone(paths[:listMax]), fmt.Sprintf("… +%d more", len(paths)-listMax))
}

func (o Outcome) Diagnostics(phase string) []cyclestate.Diagnostic {
	if o.TakeErr != nil {
		return []cyclestate.Diagnostic{{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: snapshot unavailable for read-only phase %s (%v) — worktree writes by this phase were not checked", phase, o.TakeErr)}}
	}
	var out []cyclestate.Diagnostic
	if o.RetakenAfter != nil {
		out = append(out, cyclestate.Diagnostic{Severity: "warning", Message: fmt.Sprintf(
			"worktree fence: snapshot retaken after: %v — read-only phase %s was fenced by the second snapshot", o.RetakenAfter, phase)})
	}
	if len(o.Restored) > 0 {
		shown, more := o.Restored, ""
		if len(shown) > listMax {
			more = fmt.Sprintf(" … +%d more", len(shown)-listMax)
			shown = shown[:listMax]
		}
		msg := fmt.Sprintf(
			"worktree fence: read-only phase %s wrote %d path(s) into its worktree; restored to the dispatched tree: %s%s",
			phase, len(o.Restored), strings.Join(shown, ", "), more)
		if o.patchPath != "" {
			msg += fmt.Sprintf(" (reverted patch: %s)", o.patchPath)
		} else if o.patchErr != nil {
			msg += fmt.Sprintf(" (could not save %s: %v)", filepath.Join(o.workspace, patchFileName), o.patchErr)
		}
		out = append(out, cyclestate.Diagnostic{Severity: "warning", Message: msg})
	}
	if o.Writer && len(o.Created) > 0 {
		out = append(out, cyclestate.Diagnostic{Severity: "error", Message: fmt.Sprintf(
			"worktree fence: writer phase %s created %d file(s) outside its grant and the fence removed them: %s; next step: grant each path with --writable PATH, or grant a directory for new files with --writable-dir DIR, then run the seat again",
			phase, len(o.Created), strings.Join(listed(o.Created), ", "))})
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

func (f *Fence) RetakenAfter() error {
	if f == nil {
		return nil
	}
	return f.retakenAfter
}

func (f *Fence) TakeErr() error {
	if f == nil {
		return nil
	}
	return f.takeErr
}
