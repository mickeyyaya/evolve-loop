package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	waveCodeRefreshVerb    = "code-refresh"
	waveCodeRefreshDesc    = "refresh the code index"
	waveCodeRefreshTimeout = 5 * time.Minute
	waveCodeIndexMainRef   = "origin/main"
)

func runWaveCodeRefresh(run sysexec.RunFunc, timeout time.Duration, env, args []string, stdout, stderr io.Writer) int {
	var projectRoot string
	if _, err := (cliFlags{values: map[string]*string{"--project-root": &projectRoot}}).parse(args); err != nil || projectRoot == "" {
		fmt.Fprintf(stderr, "%s%s needs --project-root\n", wavePrefix, waveCodeRefreshVerb)
		return exitUsage
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	indexDir, err := refreshCodeIndex(ctx, run, env, projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%sWARN: code index not refreshed: %v\n", wavePrefix, err)
		return 0
	}
	fmt.Fprintf(stdout, "%scode index refreshed at %s\n", wavePrefix, indexDir)
	return 0
}

func refreshCodeIndex(ctx context.Context, run sysexec.RunFunc, env []string, projectRoot string) (string, error) {
	loc, err := loadCodeIndexLocation(projectRoot)
	if err != nil {
		return "", err
	}
	indexDir, err := guardedIndexDir(projectRoot, loc.indexDir)
	if err != nil {
		return "", err
	}
	loc.indexDir = indexDir
	backends, err := newCodeBackends(loc, run, env)
	if err != nil {
		return "", err
	}
	top, err := runInIndexWorktree(ctx, run, env, indexDir, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", err
	}
	if filepath.Clean(strings.TrimSpace(top)) != indexDir {
		return "", fmt.Errorf("code_index.index_dir %s is not its own git worktree (git reports %q)", indexDir, strings.TrimSpace(top))
	}
	for _, git := range [][]string{{"fetch", "--quiet", "origin"}, {"checkout", "--quiet", "--detach", waveCodeIndexMainRef}} {
		if _, err := runInIndexWorktree(ctx, run, env, indexDir, git); err != nil {
			return "", err
		}
	}
	if _, err := backends.Refresh(ctx); err != nil {
		return "", err
	}
	return indexDir, nil
}

type indexDirRule struct {
	reason string
	hit    func(dir string) bool
}

func guardedIndexDir(projectRoot, rawIndexDir string) (string, error) {
	root := resolveExisting(projectRoot)
	dir := resolveExisting(rawIndexDir)
	rules := []indexDirRule{
		{"lies under the project worktrees directory", func(d string) bool { return pathWithin(filepath.Join(root, ".evolve", "worktrees"), d) }},
		{"is the project root", func(d string) bool { return d == root }},
		{"lies inside the project root", func(d string) bool { return pathWithin(root, d) }},
		{"is or lies inside the runtime plane", func(d string) bool {
			hub, err := plane.ResolveHub(projectRoot)
			return err == nil && pathWithin(resolveExisting(filepath.Join(hub.Root, "runtime")), d)
		}},
	}
	for _, r := range rules {
		if r.hit(dir) {
			return "", fmt.Errorf("code_index.index_dir %s %s; it must be a separate worktree", dir, r.reason)
		}
	}
	return dir, nil
}

func resolveExisting(p string) string {
	clean := filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		return resolved
	}
	return filepath.Join(resolveExisting(filepath.Dir(clean)), filepath.Base(clean))
}

func pathWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func runInIndexWorktree(ctx context.Context, run sysexec.RunFunc, env []string, indexDir string, args []string) (string, error) {
	var stdout, stderr strings.Builder
	rc, err := run(ctx, "git", indexDir, args, env, nil, &stdout, &stderr)
	switch {
	case err != nil:
		return "", fmt.Errorf("git %s in %s: %w", args[0], indexDir, err)
	case rc != 0:
		return "", fmt.Errorf("git %s in %s: exit %d: %s", args[0], indexDir, rc, firstNonBlankLine(stderr.String()))
	}
	return stdout.String(), nil
}

func firstNonBlankLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
