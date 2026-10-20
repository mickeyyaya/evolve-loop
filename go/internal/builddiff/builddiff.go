// Package builddiff derives the diff of a cycle from its recorded base and writes it into the build report.
// See docs/architecture/packages/internal-builddiff.md.
package builddiff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const untrackedReadLimit = 1 << 20

var ErrNoBase = errors.New("builddiff: the cycle has no recorded base")

type Change struct {
	Action, Path string
}

type Summary struct {
	Files, Added, Deleted int
	Changes               []Change
}

func (s Summary) String() string {
	return fmt.Sprintf("%d files +%d -%d %v", s.Files, s.Added, s.Deleted, s.Changes)
}

var actionOf = map[byte]string{'A': "ADD", 'D': "DELETE", 'M': "MODIFY", 'T': "MODIFY"}

func Derive(ctx context.Context, g gitexec.Git, base string) (Summary, error) {
	if base == "" {
		return Summary{}, ErrNoBase
	}
	numstat, err := g.Output(ctx, "-c", "core.quotepath=off", "diff", "--numstat", "--no-renames", base)
	if err != nil {
		return Summary{}, fmt.Errorf("git diff --numstat %s: %w", base, err)
	}
	status, err := g.Output(ctx, "-c", "core.quotepath=off", "diff", "--name-status", "--no-renames", base)
	if err != nil {
		return Summary{}, fmt.Errorf("git diff --name-status %s: %w", base, err)
	}
	others, err := g.Output(ctx, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return Summary{}, fmt.Errorf("git ls-files --others: %w", err)
	}
	var s Summary
	s.addNumstat(numstat)
	s.addStatus(status)
	s.addUntracked(g.Dir, strings.Split(strings.TrimRight(others, "\x00"), "\x00"))
	slices.SortFunc(s.Changes, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	s.Files = len(s.Changes)
	return s, nil
}

func (s *Summary) addNumstat(out string) {
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			continue
		}
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		s.Added += added
		s.Deleted += deleted
	}
}

func (s *Summary) addStatus(out string) {
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		code, path, ok := strings.Cut(line, "\t")
		if !ok || code == "" {
			continue
		}
		action, known := actionOf[code[0]]
		if !known {
			action = code
		}
		s.Changes = append(s.Changes, Change{Action: action, Path: path})
	}
}

func (s *Summary) addUntracked(root string, paths []string) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		s.Added += untrackedLines(filepath.Join(root, filepath.FromSlash(path)))
		s.Changes = append(s.Changes, Change{Action: "ADD", Path: path})
	}
}

func untrackedLines(path string) int {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > untrackedReadLimit {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	body, _ := io.ReadAll(io.LimitReader(f, untrackedReadLimit))
	return textLineCount(body)
}

func textLineCount(body []byte) int {
	if bytes.IndexByte(body, 0) >= 0 {
		return 0
	}
	n := bytes.Count(body, []byte("\n"))
	if len(body) > 0 && body[len(body)-1] != '\n' {
		n++
	}
	return n
}
