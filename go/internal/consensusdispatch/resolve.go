package consensusdispatch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// resolveNativeDispatch returns an exec.Cmd that runs `evolve <subcmd> <args...>`
// via the native binary.
//
// Resolution order:
//
//  1. <EVOLVE_GO_BIN> if set + executable
//  2. <repo>/go/bin/evolve relative to dispatchDir's ancestor
//  3. `evolve` on PATH
//
// The legacy bash fallback (bash <dispatchDir>/<subcmd>.sh) was removed in
// v12.0.0 together with legacy/scripts/dispatch/ (ADR-0062/T1.7): there is no
// script to fall back to, so a missing native binary is a hard error rather than
// a doomed `bash <deleted>.sh` invocation.
func resolveNativeDispatch(dispatchDir, subcmd string, subArgs []string) (*exec.Cmd, error) {
	binPath := resolveEvolveBin(dispatchDir)
	if binPath == "" {
		return nil, fmt.Errorf("consensusdispatch: native evolve binary not found (set EVOLVE_GO_BIN or build go/bin/evolve)")
	}
	args := append([]string{subcmd}, subArgs...)
	return exec.Command(binPath, args...), nil
}

// resolveEvolveBin walks up from dispatchDir looking for go/bin/evolve.
// Mirrors releasepipeline.resolveEvolveBin but takes a hint dir rather
// than a repo root.
func resolveEvolveBin(hintDir string) string {
	if p := os.Getenv("EVOLVE_GO_BIN"); p != "" {
		if info, err := os.Stat(p); err == nil && info.Mode()&0o111 != 0 {
			return p
		}
	}
	candidate := hintDir
	for i := 0; i < 8; i++ { // bounded walk
		gobin := filepath.Join(candidate, "go", "bin", "evolve")
		if info, err := os.Stat(gobin); err == nil && info.Mode()&0o111 != 0 {
			return gobin
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
		candidate = parent
	}
	if found, err := exec.LookPath("evolve"); err == nil {
		return found
	}
	return ""
}
