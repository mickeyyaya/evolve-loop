package swarm

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const testSocketRegistryDir = "evolve-test-socket-dirs"

func testSocketRegistry() string {
	return filepath.Join(os.TempDir(), testSocketRegistryDir)
}

func RegisterTestSocketDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	registry := testSocketRegistry()
	if err := os.MkdirAll(registry, 0o700); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(dir))
	entry := filepath.Join(registry, hex.EncodeToString(sum[:8]))
	tmp := entry + ".tmp"
	if err := os.WriteFile(tmp, []byte(dir), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, entry)
}

func registeredTestSocketDirs() []string {
	registry := testSocketRegistry()
	entries, err := os.ReadDir(registry)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		entry := filepath.Join(registry, e.Name())
		raw, err := os.ReadFile(entry)
		if err != nil {
			continue
		}
		dir := string(raw)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			_ = os.Remove(entry)
			continue
		}
		if filepath.IsAbs(dir) && !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}
