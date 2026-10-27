package consensusdispatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func resolveNativeDispatch(dispatchDir, subcmd string, subArgs []string) (*exec.Cmd, error) {
	binPath := resolveEvolveBin(dispatchDir)
	if binPath == "" {
		return nil, fmt.Errorf("consensusdispatch: native evolve binary not found (build go/bin/evolve, or put evolve on PATH)")
	}
	args := append([]string{subcmd}, subArgs...)
	return sysexec.Command(context.Background(), binPath, args...), nil
}

func resolveEvolveBin(hintDir string) string {
	candidate := hintDir
	for i := 0; i < 8; i++ {
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
