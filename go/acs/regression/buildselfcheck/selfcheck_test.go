//go:build acs

// Package buildselfcheck is the ACS toolchain-green hard gate. It fails the
// per-cycle audit when the deterministic post-build self-check
// (core.buildSelfCheck) recorded any changed-package unit-test failure in
// .evolve/build-selfcheck.json.
//
// Why this guard exists: buildSelfCheck already runs `go test` (which runs
// `go vet`) on every changed Go package after the build phase and writes the
// failing packages to .evolve/build-selfcheck.json — but it is best-effort and
// NEVER aborts ("audit is the backstop"). Nothing enforced the artifact, so the
// LLM auditor was the only thing standing between a broken build and a ship —
// and it missed relaunch cycle 12, which shipped a PASS with a `go vet` failure
// (`string(Stage)` → a 1-rune garbage string) and a failing unit test. This
// guard makes the existing detection a DETERMINISTIC red_count failure: a cycle
// whose changed packages don't build/vet/test green cannot ship, regardless of
// what the LLM auditor concludes.
//
// Naturally inert: when no cycle ran (main/CI), the artifact is absent → PASS.
// buildSelfCheck clears the artifact at the start of every build, so a passing
// retry never inherits a stale failure (the gate would otherwise loop).
package buildselfcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// artifactRepoPath is the build-selfcheck artifact, relative to the worktree root.
const artifactRepoPath = ".evolve/build-selfcheck.json"

// pkgFailure mirrors core.selfCheckFailure (the artifact's element shape): a
// changed package whose unit tests failed, with the captured `go test` output.
type pkgFailure struct {
	Pkg    string `json:"pkg"`
	Output string `json:"output"`
}

// parseSelfCheckFailures decodes the build-selfcheck artifact. A nil/empty array
// means the last build was green; a decode error returns nil (fail-open — a
// malformed artifact is not a build failure, and the format is pinned by the
// core writer's test).
func parseSelfCheckFailures(data []byte) []pkgFailure {
	var fails []pkgFailure
	if err := json.Unmarshal(data, &fails); err != nil {
		return nil
	}
	return fails
}

func TestParseSelfCheckFailures(t *testing.T) {
	if got := parseSelfCheckFailures([]byte(`[]`)); len(got) != 0 {
		t.Errorf("empty array → %d failures, want 0", len(got))
	}
	two := `[{"pkg":"./cmd/evolve","output":"vet: string(Stage)"},{"pkg":"./internal/flagregistry","output":"--- FAIL"}]`
	got := parseSelfCheckFailures([]byte(two))
	if len(got) != 2 {
		t.Fatalf("two entries → %d failures, want 2", len(got))
	}
	if got[0].Pkg != "./cmd/evolve" {
		t.Errorf("first failure pkg = %q, want ./cmd/evolve", got[0].Pkg)
	}
	if got := parseSelfCheckFailures([]byte(`not json`)); got != nil {
		t.Errorf("malformed → %v, want nil (fail-open)", got)
	}
}

// TestChangedPackagesToolchainGreen is the hard gate: any changed-package
// build/vet/test failure recorded by buildSelfCheck fails the cycle.
func TestChangedPackagesToolchainGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, artifactRepoPath))
	if err != nil {
		// Absent artifact = no cycle ran or the build was green (buildSelfCheck
		// clears it at the start and only writes on failure). Either way: PASS.
		t.Skipf("no build-selfcheck artifact (%v); toolchain gate inert", err)
	}
	fails := parseSelfCheckFailures(data)
	if len(fails) == 0 {
		return
	}
	names := make([]string, len(fails))
	for i, f := range fails {
		names[i] = f.Pkg
	}
	t.Fatalf("toolchain gate: %d changed package(s) FAIL build/vet/test — a cycle cannot ship code that does not compile/vet/test green: %s\n"+
		"first failure output:\n%s",
		len(fails), strings.Join(names, ", "), truncate(fails[0].Output, 1200))
}

// truncate bounds the captured `go test` output so the failure message stays
// readable in the acs verdict.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}
