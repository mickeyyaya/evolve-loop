// Package selfsha computes the sha256 of a file or of the running
// executable.
package selfsha

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
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

func Running() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("selfsha: resolve executable: %w", err)
	}
	return Of(p)
}
