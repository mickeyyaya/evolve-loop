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
// `<root>/go` submodule when it exists, else root itself. Formatting callers
// retain the historical root fallback on inspection errors. Safety decisions
// use ResolveModuleDir so those errors remain visible.
func ModuleDir(root string) string {
	if root == "" {
		return ""
	}
	if goDir := filepath.Join(root, "go"); isDir(goDir) {
		return goDir
	}
	return root
}

// ResolveModuleDir resolves a nested Go module only when go/go.mod exists and
// preserves filesystem errors. Callers that use the result for a safety
// decision must use this checked form: a support-only go/ directory is not a
// module, and an unreadable path cannot be treated as absent.
func ResolveModuleDir(root string) (string, error) {
	if root == "" {
		return "", nil
	}
	goDir := filepath.Join(root, "go")
	info, err := os.Stat(goDir)
	if errors.Is(err, os.ErrNotExist) {
		return root, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect Go module directory %s: %w", goDir, err)
	}
	if !info.IsDir() {
		return root, nil
	}
	moduleFile := filepath.Join(goDir, "go.mod")
	_, err = os.Stat(moduleFile)
	if errors.Is(err, os.ErrNotExist) {
		return root, nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect nested Go module file %s: %w", moduleFile, err)
	}
	return goDir, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
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
