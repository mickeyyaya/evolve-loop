package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var gitRunner sysexec.RunFunc = sysexec.DefaultRunner

func defaultGitHEAD() (string, error) {
	head, err := gitexec.Git{Exec: gitRunner}.HEAD(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN git HEAD probe failed (cycle outcome labels degraded): %v\n", err)
		return "", nil
	}
	return head, nil
}

func porcelainDirtySet(ctx context.Context, dir string) map[string]bool {
	return porcelainDirtySetTo(ctx, dir, os.Stderr)
}

func porcelainDirtySetTo(ctx context.Context, dir string, warn io.Writer) map[string]bool {
	set, err := porcelainDirty(ctx, dir)
	if err != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN dirty-set: %v — read as a clean tree\n", err)
		return map[string]bool{}
	}
	return set
}

func porcelainDirty(ctx context.Context, dir string) (map[string]bool, error) {
	set := map[string]bool{}
	out, code, err := gitCapture(ctx, dir, "status", "--porcelain", "-uall")
	if err != nil {
		return nil, fmt.Errorf("git status in %s: %w", dir, err)
	}
	if code != 0 {
		return nil, fmt.Errorf("git status in %s: exit %d", dir, code)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		set[porcelainPath(line)] = true
		if old := porcelainOldPath(line); old != "" {
			set[old] = true
		}
	}
	return set, nil
}

func porcelainPath(line string) string    { return gitexec.PorcelainPath(line) }
func porcelainOldPath(line string) string { return gitexec.PorcelainOldPath(line) }

func gitCapture(ctx context.Context, dir string, args ...string) (string, int, error) {
	stdout, stderr, code, err := gitexec.Git{Dir: dir, Exec: gitRunner}.Capture(ctx, args...)
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}
	if err != nil {
		return "", -1, err
	}
	return stdout, code, nil
}

func defaultGitDirtyPaths(ctx context.Context, repoRoot string) ([]string, error) {
	set, err := porcelainDirty(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}
