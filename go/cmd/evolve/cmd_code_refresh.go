package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	codeRefreshVerbName     = "refresh"
	codeRefreshSchema       = "evolve.code.refresh/1"
	codeIndexRefreshTimeout = 5 * time.Minute
	codeIndexMainBranch     = "main"
	codeIndexMainRef        = "origin/" + codeIndexMainBranch

	codeExitRefused    = 3
	codeExitIndexDirty = 22
	codeExitIndexFetch = 23
)

var (
	errCodeIndexAbsent = errors.New("the code index is absent")
	errCodeIndexDirty  = errors.New("the code index worktree has changes that are not the index tool's own")
	errCodeIndexFetch  = errors.New("git fetch origin main failed in the code index worktree")

	codeIndexToolEdits = []string{".gitignore", ".ignore"}
)

type codeIndexHost struct {
	run  sysexec.RunFunc
	env  []string
	stat func(string) (os.FileInfo, error)
}

func defaultCodeIndexHost() codeIndexHost {
	return codeIndexHost{run: sysexec.DefaultRunner, env: os.Environ(), stat: os.Stat}
}

type codeIndexMove struct {
	IndexDir string   `json:"index_dir"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Behind   int      `json:"behind"`
	Rebuilt  []string `json:"rebuilt"`
}

func (h codeIndexHost) refresh(ctx context.Context, projectRoot string) (codeIndexMove, error) {
	loc, err := loadCodeIndexLocation(projectRoot)
	if err != nil {
		return codeIndexMove{}, err
	}
	if loc.indexDir, err = guardedIndexDir(projectRoot, loc.indexDir); err != nil {
		return codeIndexMove{}, err
	}
	backends, err := newCodeBackends(loc, h.run, h.env)
	if err != nil {
		return codeIndexMove{}, err
	}
	if err := h.checkIndexWorktree(ctx, loc.indexDir); err != nil {
		return codeIndexMove{}, err
	}
	if _, err := h.git(ctx, loc.indexDir, "fetch", "--quiet", "origin", codeIndexMainBranch); err != nil {
		return codeIndexMove{}, fmt.Errorf("%w: %w", errCodeIndexFetch, err)
	}
	fresh, err := h.compare(ctx, loc.indexDir)
	if err != nil {
		return codeIndexMove{}, err
	}
	if _, err := h.git(ctx, loc.indexDir, "checkout", "--quiet", "--detach", codeIndexMainRef); err != nil {
		return codeIndexMove{}, fmt.Errorf("%w; clean the index worktree, then run evolve code refresh", err)
	}
	rebuilt, err := backends.Refresh(ctx)
	if err != nil {
		return codeIndexMove{}, err
	}
	return codeIndexMove{IndexDir: loc.indexDir, From: fresh.Head, To: fresh.Main, Behind: fresh.Behind, Rebuilt: append([]string{}, rebuilt...)}, nil
}

func (h codeIndexHost) checkIndexWorktree(ctx context.Context, indexDir string) error {
	if err := h.present(indexDir); err != nil {
		return err
	}
	top, err := h.git(ctx, indexDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	if filepath.Clean(strings.TrimSpace(top)) != indexDir {
		return fmt.Errorf("code_index.index_dir %s is not its own git worktree (git reports %q)", indexDir, strings.TrimSpace(top))
	}
	status, err := h.git(ctx, indexDir, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return err
	}
	if foreign := foreignIndexChanges(status); len(foreign) > 0 {
		return fmt.Errorf("%w: %s; nothing was reset: clean %s by hand, then run evolve code refresh", errCodeIndexDirty, strings.Join(foreign, ", "), indexDir)
	}
	return nil
}

func (h codeIndexHost) present(indexDir string) error {
	_, err := h.stat(indexDir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%w: %s does not exist (see docs/architecture/code-index-cli.md, section 4)", errCodeIndexAbsent, indexDir)
	case err != nil:
		return fmt.Errorf("code index %s: %w", indexDir, err)
	}
	return nil
}

func foreignIndexChanges(porcelain string) []string {
	var foreign []string
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 {
			continue
		}
		if path := line[3:]; !slices.Contains(codeIndexToolEdits, path) {
			foreign = append(foreign, path)
		}
	}
	return foreign
}

func (h codeIndexHost) git(ctx context.Context, indexDir string, args ...string) (string, error) {
	var stdout, stderr strings.Builder
	rc, err := h.run(ctx, "git", indexDir, args, h.env, nil, &stdout, &stderr)
	switch {
	case err != nil:
		return "", fmt.Errorf("git %s in %s: %w", args[0], indexDir, err)
	case rc != 0:
		return "", fmt.Errorf("git %s in %s: exit %d: %s", args[0], indexDir, rc, firstNonBlankLine(stderr.String()))
	}
	return stdout.String(), nil
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

type indexDirRule struct {
	reason string
	hit    func(dir string) bool
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

func firstNonBlankLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func codeRefreshExit(err error) (int, string) {
	switch {
	case err == nil:
		return 0, ""
	case errors.Is(err, errCodeIndexAbsent):
		return codeExitAbsent, "absent"
	case errors.Is(err, errCodeIndexDirty):
		return codeExitIndexDirty, "dirty"
	case errors.Is(err, errCodeIndexFetch):
		return codeExitIndexFetch, "fetch"
	}
	return codeExitFailed, "failed"
}

func codeRefreshVerb(args []string, stdout, stderr io.Writer, host codeIndexHost, getenv func(string) string, timeout time.Duration) int {
	fs := flag.NewFlagSet("evolve code refresh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print one evolve.code.refresh/1 envelope on stdout")
	root := fs.String("project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	out := codeEnvelopeWriter{schema: codeRefreshSchema, json: slices.Contains(args, "--json"), stdout: stdout, stderr: stderr}
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return out.fail(codeExitUsage, "usage", fmt.Sprintf("evolve code refresh: want no argument\n%s", codeUsage))
	}
	out.json = *asJSON
	if subagent.ReadDispatchDepth(getenv) > 0 {
		return out.fail(codeExitRefused, "refused", "evolve code refresh: refused inside a phase: the code index is shared, so a phase agent asks the pipeline for a refresh and never runs it")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	moved, err := host.refresh(ctx, codeProjectRoot(*root))
	if rc, kind := codeRefreshExit(err); rc != 0 {
		return out.fail(rc, kind, fmt.Sprintf("evolve code refresh: %v", err))
	}
	if !out.json {
		fmt.Fprintf(stdout, "code refresh: %s is at %s (%s), %d commit(s) after %s\n", moved.IndexDir, codeIndexMainRef, shortSHA(moved.To), moved.Behind, shortSHA(moved.From))
	}
	return out.ok(moved)
}
