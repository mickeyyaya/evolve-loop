package gittest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrHomeNotHermetic = errors.New("the test HOME is not an isolated scratch dir")

var mkdirAll = os.MkdirAll

const scratchHomePrefix = "evhome"

func IsolateHome() (func() error, error) {
	dir, err := mkdirTemp("", scratchHomePrefix)
	if err != nil {
		return nil, fmt.Errorf("isolate home: %w", err)
	}
	backups := snapshotEnv([]string{"HOME", "CODEX_HOME", "GOCACHE", "GOPATH"})
	codexHome := filepath.Join(dir, ".codex")
	if err := mkdirAll(codexHome, 0o755); err != nil {
		_ = removeAll(dir)
		return nil, fmt.Errorf("isolate home: %w", err)
	}
	pinToolchainDirs()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("CODEX_HOME", codexHome)
	return func() error { return restoreEnv(backups, dir, "isolate home restore") }, nil
}

func pinToolchainDirs() {
	if os.Getenv("GOCACHE") == "" {
		if cache, err := os.UserCacheDir(); err == nil {
			_ = os.Setenv("GOCACHE", filepath.Join(cache, "go-build"))
		}
	}
	if os.Getenv("GOPATH") == "" {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Setenv("GOPATH", filepath.Join(home, "go"))
		}
	}
}

func HermeticHomeError() error {
	home := os.Getenv("HOME")
	if !strings.HasPrefix(home, filepath.Join(os.TempDir(), scratchHomePrefix)) {
		return fmt.Errorf("%w: HOME=%q", ErrHomeNotHermetic, home)
	}
	if codex := os.Getenv("CODEX_HOME"); !strings.HasPrefix(codex, home+string(filepath.Separator)) {
		return fmt.Errorf("%w: CODEX_HOME=%q", ErrHomeNotHermetic, codex)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: an auth file sits under HOME", ErrHomeNotHermetic)
	}
	return nil
}
