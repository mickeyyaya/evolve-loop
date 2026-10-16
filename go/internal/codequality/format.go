package codequality

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func ModuleDir(root string) string {
	if root == "" {
		return ""
	}
	if goDir := filepath.Join(root, "go"); isDir(goDir) {
		return goDir
	}
	return root
}

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

func FormatGoFiles(dir string) ([]string, error) {
	dirty, err := UnformattedGoFiles(dir)
	if err != nil {
		return nil, err
	}
	var fixable []string
	for _, f := range dirty {
		if strings.HasPrefix(f, "gofmt parse error:") {
			continue
		}
		fixable = append(fixable, f)
	}
	if len(fixable) == 0 {
		return nil, nil
	}
	if _, werr := sysexec.Command(context.Background(), "gofmt", "-w", "-s", dir).Output(); werr != nil {
		var exitErr *exec.ExitError
		if !errors.As(werr, &exitErr) {
			return nil, fmt.Errorf("gofmt -w -s %s: %w", dir, werr)
		}
	}
	return fixable, nil
}
