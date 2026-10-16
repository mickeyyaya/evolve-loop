package fleet

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func PreflightControlPlane(repoRoot string) error {
	cmd := sysexec.Command(context.Background(), "git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fleet: control-plane preflight: git status in %q failed (an unverifiable tree never passes the guard): %w: %s",
			repoRoot, err, strings.TrimSpace(string(out)))
	}
	var dirty []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		for _, p := range strings.Split(line[3:], " -> ") {
			p = strings.Trim(strings.TrimSpace(p), `"`)
			if p != "" && guards.IsProtectedSurface(p) {
				dirty = append(dirty, p)
			}
		}
	}
	if len(dirty) == 0 {
		return nil
	}
	return fmt.Errorf("fleet: control-plane file(s) %s have uncommitted changes in %q; commit them via `evolve ship --class manual` before dispatching a wave",
		strings.Join(dirty, ", "), repoRoot)
}
