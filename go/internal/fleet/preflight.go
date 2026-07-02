package fleet

// preflight.go — FLEET-AS-POLICY S3(a): the dirty-control-plane wave
// preflight. Fixes the fleet-trial-#1 failure class (cycle-467 scout H1): a
// dirty .evolve/policy.json in the MAIN checkout killed an audit-PASSED lane
// at ship time; this guard surfaces the same condition at wave START, before
// any lane is planned or launched.
//
// It lives in internal/fleet — NOT internal/guards — because the guards
// package is itself pipeline-protected surface (an autonomous cycle editing
// it would trip its own integrity gate), and because keeping the helper
// importable leaves the door open to the generalized launch-path preflight
// (`evolve fleet --plan`, single-cycle loop; scout B2).

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

// PreflightControlPlane refuses wave dispatch while the git working tree at
// repoRoot (production: the MAIN checkout, cfg.ProjectRoot) carries any
// uncommitted change — a modified tracked file OR an untracked addition —
// touching the pipeline integrity control plane per
// guards.IsProtectedSurface. The refusal names every offending path and the
// remediation (`evolve ship --class manual`). A tree that cannot be verified
// (git status fails, e.g. repoRoot is not a git repository) fails loud: an
// unverifiable tree never silently passes the guard.
func PreflightControlPlane(repoRoot string) error {
	// --untracked-files=all expands untracked directories to per-file entries,
	// so a brand-new protected file inside a brand-new directory is still
	// named individually.
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
		// A rename entry reads `R  old -> new`; both sides are uncommitted
		// control-plane churn, so check each.
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
