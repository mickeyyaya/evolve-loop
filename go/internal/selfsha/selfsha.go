// Package selfsha computes the sha256 of a file or of the running
// executable.
package selfsha

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("selfsha: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("selfsha: read %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func Running() (string, error) { return hashExecutable(Path) }

func hashExecutable(resolve func() (string, error)) (string, error) {
	p, err := resolve()
	if err != nil {
		return "", err
	}
	return Of(p)
}

func Path() (string, error) { return resolveExecutable(os.Executable) }

func resolveExecutable(executable func() (string, error)) (string, error) {
	p, err := executable()
	if err != nil {
		return "", fmt.Errorf("selfsha: resolve executable: %w", err)
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("selfsha: resolve symlinks of %s: %w", p, err)
	}
	return real, nil
}
