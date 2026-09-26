package fleet

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// PreflightControlPlane refuses a wave while repoRoot has uncommitted protected-surface changes or cannot be checked.
func PreflightControlPlane(repoRoot string) error {
	// --untracked-files=all names a new protected file inside a new directory individually.
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fleet: control-plane preflight: git status in %q failed (an unverifiable tree never passes the guard): %w: %s",
			repoRoot, err, strings.TrimSpace(string(out)))
	}
	var dirty []string
	for _, line := range strings.Split(string(out), "\n") {
		// Porcelain v1: two status characters, a space, then the path.
		if len(line) < 4 {
			continue
		}
		// A rename reads `R  old -> new`; both sides are uncommitted churn.
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
