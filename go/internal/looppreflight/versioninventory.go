package looppreflight

// versioninventory.go — CLI version capture and drift-detection cache.
//
// Motivation (cycle-308 inbox item 2026-06-12T16-08-42Z): claude moved
// 2.1.173→2.1.175 between batches despite autoUpdates:false. loop-preflight.json
// recorded no version strings, so the silent change was invisible. This file
// provides:
//
//  1. execVersion — replaceable seam for `<bin> --version`
//  2. captureVersionInventory — parses the version token from --version output
//  3. loadVersionCache / saveVersionCache — persist last-seen to .evolve/cli-versions.json

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// versionCaptureTimeout bounds each `--version` exec; a hung binary must
// degrade gracefully (bin omitted from inventory), not stall every batch start.
const versionCaptureTimeout = 5 * time.Second

// execVersion shells out to "<bin> --version" and returns trimmed stdout.
// It is a package-level variable so tests can replace it without threading
// seams through the full Options stack.
var execVersion = func(bin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionCaptureTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// versionTokenRE matches the first semver-like version token (M.N or M.N.P)
// in a --version output line. Applied to strings like "claude 2.1.175 (release
// build)" or "codex-cli 0.139.0".
var versionTokenRE = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)

// captureVersionInventory calls execVersion for each bin and returns a
// bin→version map. Bins whose probe errors or whose output has no semver-like
// token are omitted (best-effort; never a crash).
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

// loadVersionCache reads the last-seen version map from path. A missing file
// is not an error — it means this is the first batch (nil, nil). Other read or
// parse errors are returned so the caller can log and degrade gracefully.
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

// saveVersionCache atomically writes the current version map to path via
// a temp+rename (mirrors the atomic-write convention in this repo).
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
