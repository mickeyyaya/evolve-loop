// Package selfsha computes the sha256 of a file or of the running
// executable. It is a stdlib-only leaf (no internal imports) so any package —
// checkpoint, ship, core — can depend on it without an import cycle. The
// per-phase integrity design (ADR-0065) hashes the running evolve binary at
// each phase boundary; this is the single shared implementation.
package selfsha

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Of returns the hex-encoded sha256 of the file at path.
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

// Running returns the hex-encoded sha256 of the currently-running executable
// (resolved via os.Executable()).
func Running() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("selfsha: resolve executable: %w", err)
	}
	return Of(p)
}
