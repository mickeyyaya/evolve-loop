package codequality

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ModuleDir resolves the Go module directory under root: the conventional
// `<root>/go` submodule when it exists, else root itself. It is the single
// source of truth for "where the .go files live", shared by the audit gofmt
// gate and the post-build gofmt normalizer so the two can never disagree on
// which tree to verify vs. format.
func ModuleDir(root string) string {
	if root == "" {
		return ""
	}
	if goDir := filepath.Join(root, "go"); isDir(goDir) {
		return goDir
	}
	return root
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// FormatGoFiles applies `gofmt -w -s` to dir, rewriting every .go file that is
// not already gofmt-s-clean (the same simplification CI and the audit gate
// require, via the shared gofmt binary). It returns the parseable files it ran
// gofmt over (those that were not already clean); the audit gofmt gate verifies
// the result, so the return value is for reporting, not a clean guarantee. An
// empty result means the tree was already clean. A missing gofmt binary is an
// error — the normalizer cannot run. An unparseable file is left untouched for
// the audit gate to reject: gofmt -w cannot fix code that does not parse, and
// such code must never ship.
func FormatGoFiles(dir string) ([]string, error) {
	dirty, err := UnformattedGoFiles(dir)
	if err != nil {
		return nil, err
	}
	var fixable []string
	for _, f := range dirty {
		if strings.HasPrefix(f, "gofmt parse error:") {
			continue // gofmt -w cannot fix unparseable files; leave for the gate
		}
		fixable = append(fixable, f)
	}
	if len(fixable) == 0 {
		return nil, nil
	}
	if _, werr := exec.Command("gofmt", "-w", "-s", dir).Output(); werr != nil {
		var exitErr *exec.ExitError
		if !errors.As(werr, &exitErr) {
			return nil, fmt.Errorf("gofmt -w -s %s: %w", dir, werr)
		}
		// gofmt RAN but a file failed to parse; the parseable files were still
		// rewritten. The unparseable file is left for the audit gate to reject.
	}
	return fixable, nil
}
