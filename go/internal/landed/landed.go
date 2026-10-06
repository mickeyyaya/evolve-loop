// Package landed decides whether a worktree's changes since a merge-base are already in origin/main.
package landed

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	gitModeAbsent  = "000000"
	gitModeSymlink = "120000"
	gitModeGitlink = "160000"
)

type Verdict struct {
	Landed bool
	Reason string
}

type Blob struct {
	Data    []byte
	Present bool
}

type UntrackedFile struct {
	Name, Hash string
}

type change struct {
	oldMode, newMode, status, path string
}

type pathVersions struct {
	base, main, tree Blob
	mainMode         string
}

var landedVerdict = Verdict{Landed: true}

func notLanded(format string, args ...any) Verdict {
	return Verdict{Reason: fmt.Sprintf(format, args...)}
}

func Changes(ctx context.Context, worktree gitexec.Git, base string) (Verdict, error) {
	changes, err := listChanges(ctx, worktree, base)
	if err != nil || len(changes) == 0 {
		return Verdict{Landed: err == nil}, err
	}
	blobs, err := readBlobs(ctx, worktree, base, changes)
	if err != nil {
		return Verdict{}, err
	}
	modes, err := mainModes(ctx, worktree, changes)
	if err != nil {
		return Verdict{}, err
	}
	for _, ch := range changes {
		tree, err := worktreeContent(worktree.Dir, ch)
		if err != nil {
			return Verdict{}, err
		}
		v := pathVersions{base: blobs[base+":"+ch.path], main: blobs[plane.OriginMainRef+":"+ch.path], tree: tree, mainMode: modes[ch.path]}
		verdict, err := judgePath(ctx, worktree, ch, v)
		if err != nil || !verdict.Landed {
			return verdict, err
		}
	}
	return landedVerdict, nil
}

func listChanges(ctx context.Context, worktree gitexec.Git, base string) ([]change, error) {
	out, err := gitIn(ctx, worktree, "", "diff", "--raw", "-z", "--no-renames", "--no-ext-diff", "--no-relative", base)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	var changes []change
	for i := 0; i+1 < len(fields); i += 2 {
		meta := strings.Fields(strings.TrimPrefix(fields[i], ":"))
		if len(meta) != 5 {
			return nil, fmt.Errorf("git diff --raw: unexpected record %q", fields[i])
		}
		if strings.Contains(fields[i+1], "\n") {
			return nil, fmt.Errorf("changed path %q has a newline in its name, so it cannot be compared with origin/main", fields[i+1])
		}
		changes = append(changes, change{oldMode: meta[0], newMode: meta[1], status: meta[4][:1], path: fields[i+1]})
	}
	return changes, nil
}

func readBlobs(ctx context.Context, worktree gitexec.Git, base string, changes []change) (map[string]Blob, error) {
	var specs []string
	for _, ch := range changes {
		specs = append(specs, base+":"+ch.path, plane.OriginMainRef+":"+ch.path)
	}
	out, err := gitIn(ctx, worktree, strings.Join(specs, "\n")+"\n", "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	return ParseCatFileBatch(specs, out)
}

func ParseCatFileBatch(specs []string, out string) (map[string]Blob, error) {
	r := bufio.NewReader(strings.NewReader(out))
	blobs := make(map[string]Blob, len(specs))
	for _, spec := range specs {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: no answer for %s: %w", spec, err)
		}
		header = strings.TrimSuffix(header, "\n")
		if strings.HasSuffix(header, " missing") {
			blobs[spec] = Blob{}
			continue
		}
		parts := strings.Fields(header)
		if len(parts) != 3 {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q for %s", header, spec)
		}
		size, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: unexpected size in %q for %s", header, spec)
		}
		data := make([]byte, size+1)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("git cat-file --batch: short read for %s: %w", spec, err)
		}
		blobs[spec] = Blob{Data: data[:size], Present: parts[1] == "blob"}
	}
	return blobs, nil
}

func mainModes(ctx context.Context, worktree gitexec.Git, changes []change) (map[string]string, error) {
	var paths []string
	for _, ch := range changes {
		if ch.oldMode != ch.newMode {
			paths = append(paths, ch.path)
		}
	}
	modes := map[string]string{}
	if len(paths) == 0 {
		return modes, nil
	}
	out, err := gitIn(ctx, worktree, "", append([]string{"--literal-pathspecs", "ls-tree", "-r", "-z", plane.OriginMainRef, "--"}, paths...)...)
	if err != nil {
		return nil, err
	}
	for _, entry := range strings.Split(strings.TrimSuffix(out, "\x00"), "\x00") {
		meta, path, ok := strings.Cut(entry, "\t")
		if fields := strings.Fields(meta); ok && len(fields) == 3 {
			modes[path] = fields[0]
		}
	}
	return modes, nil
}

func worktreeContent(dir string, ch change) (Blob, error) {
	p := filepath.Join(dir, filepath.FromSlash(ch.path))
	info, err := os.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return Blob{}, nil
	case err != nil:
		return Blob{}, err
	case info.Mode()&os.ModeSymlink != 0:
		link, err := os.Readlink(p)
		return Blob{Data: []byte(link), Present: true}, err
	case !info.Mode().IsRegular():
		return Blob{}, nil
	}
	data, err := os.ReadFile(p)
	return Blob{Data: data, Present: true}, err
}

func judgePath(ctx context.Context, worktree gitexec.Git, ch change, v pathVersions) (Verdict, error) {
	switch {
	case ch.status == "U" || ch.newMode == gitModeGitlink || ch.oldMode == gitModeGitlink:
		return notLanded("%s is unmerged or a submodule, which the proof does not judge", ch.path), nil
	case !v.tree.Present && v.main.Present:
		return notLanded("%s is deleted in the tree but still in origin/main", ch.path), nil
	case !v.tree.Present:
		return landedVerdict, nil
	case !v.main.Present:
		return notLanded("%s is not in origin/main", ch.path), nil
	case ch.oldMode != ch.newMode && v.mainMode != ch.newMode:
		return notLanded("%s has mode %s in the tree but %s in origin/main", ch.path, ch.newMode, modeOrAbsent(v.mainMode)), nil
	case bytes.Equal(v.tree.Data, v.main.Data):
		return landedVerdict, nil
	case !v.base.Present || ch.newMode == gitModeSymlink || isBinary(v.base.Data, v.main.Data, v.tree.Data):
		return notLanded("%s differs from origin/main", ch.path), nil
	}
	kept, err := mergeKeepsMain(ctx, worktree.Exec, v)
	if err != nil || !kept {
		return notLanded("%s: the tree's change is not in origin/main", ch.path), err
	}
	return landedVerdict, nil
}

func modeOrAbsent(mode string) string {
	if mode == "" {
		return gitModeAbsent
	}
	return mode
}

func isBinary(contents ...[]byte) bool {
	return slices.ContainsFunc(contents, func(c []byte) bool { return bytes.IndexByte(c, 0) >= 0 })
}

func mergeKeepsMain(ctx context.Context, run sysexec.RunFunc, v pathVersions) (bool, error) {
	tmp, err := os.MkdirTemp("", "evolve-landed-")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	for name, data := range map[string][]byte{"main": v.main.Data, "base": v.base.Data, "tree": v.tree.Data} {
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o600); err != nil {
			return false, err
		}
	}
	var merged strings.Builder
	code, err := run(ctx, "git", tmp, []string{"merge-file", "-p", "--quiet", "main", "base", "tree"}, nil, nil, &merged, io.Discard)
	if err != nil {
		return false, fmt.Errorf("git merge-file: %w", err)
	}
	return code == 0 && merged.String() == string(v.main.Data), nil
}
