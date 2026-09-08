package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// Keep the lexical path as well as its existing symlink target. Resolving the
// nearest existing ancestor also covers policies for not-yet-created files.
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
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
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
			// The worktree's .git is its own pointer file. Do not convert that
			// deny into a blanket denial of the shared repository object store.
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

// Only staging/checkpoint metadata is writable. Control-file denials also
// override broad scratch grants when the common repository lives under /tmp.
func sandboxGitWritePaths(worktree, goos string) (writes, denies []string, err error) {
	if worktree == "" {
		return nil, nil, nil
	}
	info, err := plane.Classify(worktree)
	if err != nil || !info.IsLinkedWorktree {
		return nil, nil, nil
	}
	// bubblewrap needs the whole gitdir writable to create/rename lockfiles;
	// that also exposes commondir and future config files. Do not claim the
	// narrower metadata boundary on a backend unable to express it.
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
	} // detached HEAD uses its exact GitDir path
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
