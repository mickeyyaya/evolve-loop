package consensusdispatch

import (
	"os"
	"os/exec"
	"path/filepath"
)

// resolveBashOrNative returns an exec.Cmd that prefers the native
// `evolve <subcmd>` binary over the legacy bash script at
// <dispatchDir>/<subcmd>.sh.
//
// Resolution order:
//
//	1. <EVOLVE_GO_BIN> if set + executable
//	2. <repo>/go/bin/evolve relative to dispatchDir's ancestor
//	3. `evolve` on PATH
//	4. bash <dispatchDir>/<subcmd>.sh (legacy fallback; absent in v12.0.0)
//
// v11.9.x → v12.0.0 transition: the bash fallback exists only while
// legacy/scripts/dispatch/ is present. In v12.0.0 the fallback is a no-op
// (cmd.Run will fail), and consensus dispatch requires the native binary.
func resolveBashOrNative(dispatchDir, subcmd string, subArgs []string) *exec.Cmd {
	binPath := resolveEvolveBin(dispatchDir)
	if binPath != "" {
		args := append([]string{subcmd}, subArgs...)
		return exec.Command(binPath, args...)
	}
	script := filepath.Join(dispatchDir, subcmd+".sh")
	bashArgs := append([]string{script}, subArgs...)
	return exec.Command("bash", bashArgs...)
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
