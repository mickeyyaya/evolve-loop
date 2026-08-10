// Package repostate answers one question for repo-wide scanners: which files
// under a directory does git actually TRACK? (ADR-0084 invariant 1.)
//
// Problem. Several guard suites (profiles, phasespec, phasecoherence) and the
// ship-time persona-lint scan walk on-disk config dirs (.evolve/profiles,
// .evolve/phases) that the RUNTIME also mints untracked stubs into. A scanner
// that binds everything on disk reds on state that can never reach a CI
// checkout — the 2026-08-09 zero-ship batch (fingerprint cd49274beab2) and
// the v22.13.0 release red were both this class
// (docs/incidents/2026-08-09-zero-ship-batch.md). The per-name .gitignore
// ratchet that previously protected the scanners re-arms on every new mint.
//
// Contract. TrackedSet returns the basenames (extension stripped by the
// caller's filter) of files git tracks DIRECTLY under relDir — nested entries
// are excluded so a nested tracked file can never basename-alias a same-named
// top-level stub. Includes index-staged (uncommitted) files: the stricter
// direction, no plane-vs-CI skew. On any git failure the error carries git's
// stderr; callers MUST fall back to binding every on-disk file (strict) and
// MUST treat an unexpectedly empty set as a failure — going dark unbinds the
// gate (see phasecoherence/unpaired_test.go for the reference call site).
package repostate

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// TrackedFiles returns the git-tracked paths directly under relDir (relative
// paths as git reports them, nested entries excluded).
func TrackedFiles(root, relDir string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "--", relDir).Output()
	if err != nil {
		// exec.ExitError.Error() is just "exit status N"; the reason an
		// operator needs ("not a git repository", ...) is on stderr.
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-files %s in %s: %w: %s", relDir, root, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git ls-files %s in %s: %w", relDir, root, err)
	}
	var files []string
	clean := filepath.Clean(relDir)
	for _, line := range strings.Split(string(out), "\n") {
		rel := strings.TrimSpace(line)
		if rel == "" || filepath.Dir(rel) != clean {
			continue
		}
		files = append(files, rel)
	}
	return files, nil
}

// TrackedSet returns the basenames of files git tracks directly under relDir
// that carry ext (e.g. ".json"), with ext stripped — the binding set for a
// flat directory scanner.
func TrackedSet(root, relDir, ext string) (map[string]bool, error) {
	files, err := TrackedFiles(root, relDir)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(files))
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, ext) {
			set[strings.TrimSuffix(base, ext)] = true
		}
	}
	return set, nil
}
