//go:build acs

// Package cycle72 ports the cycle-72 ACS predicates (1 bash file, 4 ACs).
package cycle72

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC72_001_P2InertCycle72 ports cycle-72/001 (P2 INERT marking verification).
// AC1: P2 row contains "INERT cycle 72"
// AC2: P2 row contains the C71 telemetry delta string
// AC3: ADR 0009 exists
// AC4: ADR 0009 contains a rollback section
func TestC72_001_P2InertCycle72(t *testing.T) {
	root := acsassert.RepoRoot(t)
	tokenEcon := filepath.Join(root, "docs", "architecture", "token-economics-2026.md")
	adr := filepath.Join(root, "docs", "architecture", "adr", "0009-p2-turn-budget-inert.md")

	if !fixtures.FilePresent(tokenEcon) {
		t.Skip("token-economics-2026.md missing — skip cycle-72-001")
	}
	if !acsassert.FileContains(t, tokenEcon, "INERT cycle 72") {
		return
	}
	if !acsassert.FileMatchesRegex(t, tokenEcon, `39 turns / \$0\.7305 vs.*26 turns / \$0\.5931`) {
		return
	}
	if !fixtures.FilePresent(adr) {
		t.Errorf("%s: ADR 0009 missing", adr)
		return
	}
	if !acsassert.FileMatchesRegex(t, adr, `(?i)rollback`) {
		return
	}
}
