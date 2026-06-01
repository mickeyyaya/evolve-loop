//go:build e2e

package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// E2E tier example (//go:build e2e → excluded from default; run via
// `make test-e2e`). It builds the REAL evolve binary and runs a subcommand
// against it — the smallest honest end-to-end: does the artifact we ship
// actually start and respond? Heavier full-cycle e2e lives in cmd/evolve's
// e2e_*_test.go (also e2e-tagged).
//
// Pattern to copy when adding an e2e test:
//  1. skip if the toolchain needed to build isn't present.
//  2. build the binary into t.TempDir().
//  3. exec it and assert on real stdout/exit.
func TestE2E_VersionSubcommandRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH; skipping e2e build")
	}
	bin := filepath.Join(t.TempDir(), "evolve")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/evolve")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}

	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("run `evolve version`: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Fatal("`evolve version` produced no output")
	}
}
