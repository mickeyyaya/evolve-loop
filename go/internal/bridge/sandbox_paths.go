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

// worktreePathTemplate is the profile-side placeholder for the cycle's
// worktree in sandbox.write_subpaths (e.g. "{worktree_path}/tests"). It is
// the same token the persona files and the bash-era dispatcher used.
const worktreePathTemplate = "{worktree_path}"

// resolveSandboxWriteGrants turns a profile's declared sandbox.write_subpaths
// into the absolute paths the SBPL generator grants. It is the write-side
// sibling of resolveSandboxDenials, with the opposite failure posture:
//
//   - A denial resolves to BOTH the declared path and its symlink target, so
//     a retarget cannot dodge it (fail-closed for denies).
//   - A grant must resolve to EXACTLY the declared path below the canonical
//     base. If any component under the base is a symlink, the grant is
//     refused and the launch fails, because honoring it would hand the phase
//     write access to wherever the link points (fail-closed for allows). This
//     is the defense the retrospective grant's former hard-coded helper
//     carried for its one path, now applied to every declared one.
//
// Relative entries resolve against the project root — the bash contract the
// profiles were written to — and "{worktree_path}" entries against the
// worktree; a worktree-templated entry with no worktree is skipped (there is
// nothing to grant and the phase cannot write source anyway). A glob entry is
// widened here to its longest glob-free ancestor (see globFreeAncestor), and
// the canonical-path walk stops at the first existing ancestor, so a
// not-yet-created claim dir resolves cleanly.
func resolveSandboxWriteGrants(subpaths []string, root, worktree string) ([]string, error) {
	var out []string
	for _, declared := range subpaths {
		if declared == "" || strings.ContainsAny(declared, "\x00\n\r") {
			return nil, fmt.Errorf("write grant must be a path: %q", declared)
		}
		var base, rel string
		switch {
		case filepath.IsAbs(declared):
			// An absolute entry is its own base: granted as declared (cleaned).
			// The retarget check below is vacuous for it by design (base == the
			// path, rel == "."), so a symlinked absolute declaration is followed
			// to its target — the operator declared that exact path.
			base, rel = filepath.Clean(declared), "."
		case strings.HasPrefix(declared, worktreePathTemplate):
			if worktree == "" {
				continue
			}
			base, rel = worktree, strings.TrimPrefix(declared, worktreePathTemplate)
		default:
			base, rel = root, declared
		}
		rel = filepath.Clean(strings.TrimPrefix(rel, string(filepath.Separator)))
		if escapesBase(rel) {
			return nil, fmt.Errorf("write grant escapes its base: %q", declared)
		}
		// A glob names a family of paths ("cycle-*"); the grant is that family's
		// home — the longest glob-free ancestor. This is the ONE home of that
		// projection: the adapters receive only literal absolute paths (their
		// stated contract), so SBPL and bwrap cannot disagree about what a glob
		// means, and a non-terminal glob ("cycle-*/learn") widens to the right
		// ancestor instead of to a literal "*" that matches nothing.
		rel = globFreeAncestor(rel)
		if base == "" || !filepath.IsAbs(base) {
			return nil, fmt.Errorf("write grant %q needs an absolute project root or worktree", declared)
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

// globFreeAncestor returns the longest leading portion of a cleaned relative
// path whose segments contain no glob metacharacter, or "." when the first
// segment already does.
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

// escapesBase reports whether a Cleaned relative path climbs above its base
// via a literal ".." component. Shared by the denial and grant resolvers so
// the two sides of the sandbox contract cannot drift on this check.
func escapesBase(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
