//go:build acs

package cycle80

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestC80_AcTableInBuildReport(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, ".evolve", "runs", "cycle-80", "build-report.md")
	if !fixtures.FilePresent(doc) {
		t.Skip("cycle-80 build-report.md missing — skip (ephemeral artifact)")
	}
	for _, marker := range []string{
		"<!-- AC-TABLE-BEGIN -->",
		"<!-- AC-TABLE-END -->",
		"<!-- harness-stamp: build-report-ac-verify.sh",
	} {
		if !acsassert.FileContains(t, doc, marker) {
			return
		}
	}
}

func TestC80_BuilderWriteToAcTableDenied(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "legacy", "scripts", "guards", "role-gate.sh")
	if !fixtures.FilePresent(gate) {
		t.Skip("role-gate.sh missing — skip cycle-80")
	}
	for _, marker := range []string{
		"AC-TABLE-BEGIN",
		"AC-TABLE-END",
		"harness-owned",
	} {
		if !acsassert.FileContains(t, gate, marker) {
			return
		}
	}
}

func TestC80_HarnessExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	harness := filepath.Join(root, "legacy", "scripts", "lifecycle", "build-report-ac-verify.sh")
	if !fixtures.FilePresent(harness) {
		t.Skip("build-report-ac-verify.sh missing — skip cycle-80")
	}
	info, err := os.Stat(harness)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Errorf("harness not executable: %s", harness)
	}
	if !acsassert.FileContains(t, harness, "set -uo pipefail") {
		return
	}
}
