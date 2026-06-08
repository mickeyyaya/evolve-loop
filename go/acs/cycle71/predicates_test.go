//go:build acs

// Package cycle71 ports the cycle-71 ACS predicates (1 bash file).
package cycle71

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC71_001_RoleGateRetrospective ports cycle-71/001.
func TestC71_001_RoleGateRetrospective(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "guards", "role-gate.sh")
	if !fixtures.FilePresent(gate) {
		t.Skip("role-gate.sh missing — skip cycle-71-001")
	}
	for _, marker := range []string{"retrospective)", "learn)"} {
		if !acsassert.FileContains(t, gate, marker) {
			return
		}
	}
	// AC4c: retrospective) case must allow instincts/lessons writes.
	// Approximate via DOTALL regex within ~500 chars of the case label.
	if !acsassert.FileMatchesRegex(t, gate, `(?s)retrospective\)[\s\S]{0,500}instincts/lessons`) {
		return
	}
}
