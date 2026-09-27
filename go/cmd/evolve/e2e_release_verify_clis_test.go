//go:build e2e

package main

import "testing"

func TestReleaseVerifyCLIMatrix_RealPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("E2E test (builds the evolve binary); skipped in -short mode")
	}
	repoRoot := mustRepoRoot(t)
	// The install/projection payload (.claude-plugin, agents, skills) lives at
	// the repo root — that is the srcDir installer.Install and runSkillsPublish
	// read from.
	srcDir := repoRoot
	binPath := buildBinary(t, t.TempDir(), "evolve", "./cmd/evolve", repoRoot)

	results := verifyReleaseCLIMatrix(srcDir, binPath, defaultMatrixDeps())

	want := len(releaseVerifyCLIs) + 1 // every CLI + the binary row
	if len(results) != want {
		t.Fatalf("expected %d rows (CLIs + binary), got %d", want, len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("%s: not OK against real payload+binary: %s", r.CLI, r.Detail)
		}
	}
}
