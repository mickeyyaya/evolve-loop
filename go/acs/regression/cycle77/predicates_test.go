//go:build acs

package cycle77

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestC77_001_AuditorColdMoveStage8(t *testing.T) {
	root := acsassert.RepoRoot(t)
	auditor := filepath.Join(root, "agents", "evolve-auditor.md")
	ref := filepath.Join(root, "agents", "evolve-auditor-reference.md")
	adr := filepath.Join(root, "docs", "architecture", "adr", "0015-auditor-cold-move-stage8.md")

	if !fixtures.FilePresent(auditor) {
		t.Skip("evolve-auditor.md missing — skip cycle-77-001")
	}
	if lines := countLines(t, auditor); lines > 300 {
		t.Errorf("AC1: %s has %d lines (expected ≤ 300)", auditor, lines)
	}
	if acsassert.FileExists(t, ref) {
		if !acsassert.FileContains(t, ref, "## Section: output-template") {
			return
		}
	}
	if !acsassert.FileMatchesRegex(t, auditor, `evolve-auditor-reference\.md.*output-template`) {
		return
	}
	if !fixtures.FilePresent(adr) {
		t.Errorf("AC4: ADR-0015 missing: %s", adr)
		return
	}
	if lines := countLines(t, adr); lines > 200 {
		t.Errorf("AC4: %s has %d lines (expected ≤ 200)", adr, lines)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<22)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}
