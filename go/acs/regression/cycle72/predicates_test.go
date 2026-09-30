//go:build acs

package cycle72

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

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
