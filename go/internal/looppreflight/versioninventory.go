package looppreflight

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// versionCaptureTimeout drops a hung binary from the inventory instead of stalling batch start.
const versionCaptureTimeout = 5 * time.Second

// execVersion runs "<bin> --version"; tests replace it.
var execVersion = func(bin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionCaptureTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// versionTokenRE matches the first M.N or M.N.P token, as in "codex-cli 0.139.0".
var versionTokenRE = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// captureVersionInventory omits bins whose probe fails or prints no version token.
func captureVersionInventory(bins []string) map[string]string {
	inv := make(map[string]string, len(bins))
	for _, b := range bins {
		raw, err := execVersion(b)
		if err != nil || raw == "" {
			continue
		}
		if m := versionTokenRE.FindString(raw); m != "" {
			inv[b] = m
		}
	}
	return inv
}

// loadVersionCache returns nil, nil when the file is absent, as on a first batch.
func loadVersionCache(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// saveVersionCache writes through a temp file and rename, so a reader never sees a torn file.
func saveVersionCache(path string, versions map[string]string) error {
	data, err := json.Marshal(versions)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
