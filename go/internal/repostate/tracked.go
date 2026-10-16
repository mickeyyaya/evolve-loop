// Package repostate lists the files that git tracks under a directory.
// Repo-wide scanners bind to this set, not to every on-disk file.
package repostate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TrackedFiles(root, relDir string) ([]string, error) {
	all, err := TrackedTree(root, relDir)
	if err != nil {
		return nil, err
	}
	var files []string
	clean := filepath.Clean(relDir)
	for _, rel := range all {
		if filepath.Dir(rel) == clean {
			files = append(files, rel)
		}
	}
	return files, nil
}

func TrackedTree(root, relDir string) ([]string, error) {
	out, err := sysexec.Command(context.Background(), "git", "-C", root, "ls-files", "--", relDir).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("git ls-files %s in %s: %w: %s", relDir, root, err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git ls-files %s in %s: %w", relDir, root, err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if rel := strings.TrimSpace(line); rel != "" {
			files = append(files, rel)
		}
	}
	return files, nil
}

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
