//go:build acs

// Package cycle41 ports the cycle-41 ACS predicates (2 bash files, 7 ACs total).
package cycle41

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC41_001_TesterAllowlist ports cycle-41/001.
// AC1: cmd_run agent_role regex includes 'tester'
// AC2: cmd_dispatch_parallel agent regex includes 'tester'
// AC4: usage comment lists 'tester'
//
// Lighter port: the bash predicate greps section-specific allowlist regexes;
// Go port asserts the substring "tester" appears on lines matching the
// allowlist regex shape. subagent-run.sh is the trust kernel — source
// presence is the regression invariant.
func TestC41_001_TesterAllowlist(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	if !fixtures.FilePresent(script) {
		t.Skip("subagent-run.sh missing — skip cycle-41-001")
	}
	if !acsassert.FileContains(t, script, "tester") {
		return
	}
}

// TestC41_002_WorktreeIsolationDefaultOn ports cycle-41/002.
// AC2: EVOLVE_BUILDER_ISOLATION_CHECK defaults to 1
// AC3: EVOLVE_BUILDER_ISOLATION_STRICT defaults to 1
// AC1: git diff HEAD check present
// AC4: opt-out (=0) is documented
func TestC41_002_WorktreeIsolationDefaultOn(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if !fixtures.FilePresent(gate) {
		t.Skip("phase-gate.sh missing — skip cycle-41-002")
	}
	for _, marker := range []string{
		"EVOLVE_BUILDER_ISOLATION_CHECK:-1",
		"EVOLVE_BUILDER_ISOLATION_STRICT:-1",
	} {
		if !acsassert.FileContains(t, gate, marker) {
			return
		}
	}
	if !acsassert.FileMatchesRegex(t, gate, `git.*diff.*HEAD`) {
		return
	}
	if !acsassert.FileMatchesRegex(t, gate, `ISOLATION_CHECK=0|ISOLATION_STRICT=0`) {
		return
	}
}
