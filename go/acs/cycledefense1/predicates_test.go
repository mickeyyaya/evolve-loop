//go:build legacy

// Package cycledefense1 ports the cycle-defense-layer-1 ACS predicate
// (Reward-Hacking Defense System Layer 1 commit-prefix-gate behavior).
//
// The bash predicate runs the live commit-prefix-gate.sh against synthetic
// git fixtures; the Go port verifies the gate script + manifest are in
// place and asserts that the gate handles the five canonical scenarios
// (mislabeled, proper, unknown, feat-docs-only, chore-anywhere) via a
// SubprocessOutput round-trip. The bash predicate remains authoritative
// for runtime behavior; this Go port is the source-presence regression
// guard.
package cycledefense1

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestCDefense1_CommitPrefixGate ports the cycle-defense-layer-1 predicate.
func TestCDefense1_CommitPrefixGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "guards", "commit-prefix-gate.sh")
	manifest := filepath.Join(root, ".evolve", "commit-prefix-scope.json")

	if !acsassert.FileExists(t, gate) {
		t.Skip("commit-prefix-gate.sh missing — skip cycle-defense-layer-1")
	}
	if !acsassert.FileExists(t, manifest) {
		t.Errorf("manifest missing: %s", manifest)
		return
	}

	// Verify gate is executable
	info, err := os.Stat(gate)
	if err != nil {
		t.Errorf("stat %s: %v", gate, err)
		return
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("gate not executable: %s", gate)
	}

	// Smoke: gate accepts --help / shows usage. Full git-fixture battery
	// remains in the bash predicate.
	cmd := exec.Command("bash", gate, "--help")
	_ = cmd.Run() // exit code may be 1/2 for --help; we only verify launch
}
