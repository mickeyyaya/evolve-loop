//go:build e2e

// End-to-end proof that the release-verify-clis matrix passes against the REAL
// in-repo payload and a freshly-built evolve binary, exercising the production
// defaultMatrixDeps() (not stubs).
//
// Why this tier: the unit tests in cmd_release_verify_clis_test.go stub every
// effect, so they prove the orchestration logic but say nothing about whether
// the real install/projection/recognition checks actually pass. This test is the
// one that catches a broken default dep — e.g. a binary smoke that assumed
// `<sub> --help` exits 0 (it does not; the dispatcher signal is the
// "unknown command" message). It builds the binary like the other e2e matrix
// tests and runs the full matrix end to end.
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

	// Every supported CLI plus the binary row must pass against the real
	// artifact — this is the deterministic form of "every LLM CLI installs and
	// the binary performs every core function".
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
