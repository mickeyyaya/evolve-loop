package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

func resolveSandboxDenials(paths []string, root, worktree string, write bool) ([]string, error) {
	var out []string
	for _, path := range paths {
		if path == "" || strings.ContainsAny(path, "*?[]{}\x00\n\r") {
			return nil, fmt.Errorf("denial must be a literal path: %q", path)
		}
		clean := filepath.Clean(path)
		if !filepath.IsAbs(clean) && root == "" && worktree == "" {
			return nil, fmt.Errorf("relative sandbox denial requires an absolute project root or worktree")
		}
		if escapesBase(clean) {
			return nil, fmt.Errorf("denial escapes repository: %q", path)
		}
		bases := []string{root, worktree}
		if filepath.IsAbs(clean) {
			bases = []string{""}
		}
		for _, base := range bases {
			if base == "" && !filepath.IsAbs(clean) {
				continue
			}
			if write && clean == ".git" && worktree != "" && root != worktree && base == root {
				continue
			}
			full := clean
			if base != "" {
				if !filepath.IsAbs(base) {
					return nil, fmt.Errorf("sandbox root must be absolute: %q", base)
				}
				full = filepath.Join(base, clean)
			}
			real, err := canonicalSandboxPath(full)
			if err != nil {
				return nil, err
			}
			for _, p := range []string{full, real} {
				if !slices.Contains(out, p) {
					out = append(out, p)
				}
			}
		}
	}
	return out, nil
}

func sandboxGitWritePaths(worktree, goos string) (writes, denies []string, err error) {
	if worktree == "" {
		return nil, nil, nil
	}
	info, err := plane.Classify(worktree)
	if err != nil || !info.IsLinkedWorktree {
		return nil, nil, nil
	}
	if goos != "darwin" {
		return nil, nil, fmt.Errorf("linked-worktree Git metadata isolation unsupported on %s: atomic lockfiles require precise file grants", goos)
	}
	common, err := plane.CommonGitDir(info)
	if err != nil {
		return nil, nil, err
	}
	writes = []string{filepath.Join(common, "objects")}
	for _, name := range []string{"index", "index.lock", "HEAD", "HEAD.lock", "logs/HEAD", "logs/HEAD.lock", "COMMIT_EDITMSG"} {
		writes = append(writes, filepath.Join(info.GitDir, name))
	}
	for _, dir := range []string{info.GitDir, common} {
		for _, name := range []string{"commondir", "gitdir", "config", "config.worktree", "hooks", "info", "shallow", "packed-refs", "objects/info"} {
			denies = append(denies, filepath.Join(dir, name))
		}
	}
	if info.Branch == "" {
		return writes, denies, nil
	}
	if filepath.Clean(info.Branch) != info.Branch || strings.HasPrefix(info.Branch, "..") || strings.ContainsAny(info.Branch, "*?[]{}\x00\n\r") {
		return nil, nil, fmt.Errorf("invalid linked-worktree branch for sandbox grants")
	}
	for _, dir := range []string{"refs/heads", "logs/refs/heads"} {
		ref := filepath.Join(common, dir, info.Branch)
		writes = append(writes, ref, ref+".lock")
	}
	return writes, denies, nil
}

func canonicalSandboxPath(path string) (string, error) {
	parent := path
	for {
		real, err := filepath.EvalSymlinks(parent)
		if err == nil {
			rel, err := filepath.Rel(parent, path)
			if err != nil {
				return "", err
			}
			return filepath.Join(real, rel), nil
		}
		if !os.IsNotExist(err) || filepath.Dir(parent) == parent {
			return "", fmt.Errorf("resolve sandbox path %q: %w", path, err)
		}
		parent = filepath.Dir(parent)
	}
}

const worktreePathTemplate = "{worktree_path}"

const runDirGlob = ".evolve/runs/cycle-*"

func resolveSandboxWriteGrants(subpaths []string, root, worktree, workspace string) ([]string, error) {
	var out []string
	for _, declared := range subpaths {
		if declared == "" || strings.ContainsAny(declared, "\x00\n\r") {
			return nil, fmt.Errorf("write grant must be a path: %q", declared)
		}
		var base, rel string
		switch {
		case filepath.IsAbs(declared):
			base, rel = filepath.Clean(declared), "."
		case strings.HasPrefix(declared, worktreePathTemplate):
			if worktree == "" {
				continue
			}
			base, rel = worktree, strings.TrimPrefix(declared, worktreePathTemplate)
		case declared == runDirGlob || strings.HasPrefix(declared, runDirGlob+"/"):
			if workspace == "" {
				continue
			}
			base, rel = workspace, strings.TrimPrefix(declared, runDirGlob)
		default:
			base, rel = root, declared
		}
		rel = filepath.Clean(strings.TrimPrefix(rel, string(filepath.Separator)))
		if escapesBase(rel) {
			return nil, fmt.Errorf("write grant escapes its base: %q", declared)
		}
		rel = globFreeAncestor(rel)
		if base == "" || !filepath.IsAbs(base) {
			return nil, fmt.Errorf("write grant %q needs an absolute project root, worktree or workspace", declared)
		}
		canonicalBase, err := canonicalSandboxPath(base)
		if err != nil {
			return nil, fmt.Errorf("write grant %q: %w", declared, err)
		}
		expected := filepath.Join(canonicalBase, rel)
		actual, err := canonicalSandboxPath(expected)
		if err != nil {
			return nil, fmt.Errorf("write grant %q: %w", declared, err)
		}
		if actual != expected {
			return nil, fmt.Errorf("write grant %q resolves outside its declared scope (%s -> %s)", declared, expected, actual)
		}
		if !slices.Contains(out, expected) {
			out = append(out, expected)
		}
	}
	return out, nil
}

func globFreeAncestor(rel string) string {
	segments := strings.Split(rel, string(filepath.Separator))
	keep := 0
	for _, s := range segments {
		if strings.ContainsAny(s, "*?[") {
			break
		}
		keep++
	}
	if keep == 0 {
		return "."
	}
	return filepath.Join(segments[:keep]...)
}

func escapesBase(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
