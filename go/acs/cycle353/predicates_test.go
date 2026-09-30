//go:build acs

package cycle353

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func TestC353_001_ObserverSpotChecksPass(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/flagregistry/...",
		"-run", "TestLookup_SpotChecks",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/flagregistry/... -run TestLookup_SpotChecks failed (exit=%d).\n"+
			"Builder must update registry_table.go: set Status=StatusActive for\n"+
			"  EVOLVE_OBSERVER_STALL_S, EVOLVE_OBSERVER_POLL_S,\n"+
			"  EVOLVE_OBSERVER_NUDGE_S, EVOLVE_OBSERVER_NUDGE_BODY.\n\nOutput:\n%s",
			code, combined)
	}
}

// acs-predicate: config-check
func TestC353_002_NudgeSDefaultIs300InRuntimeRef(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileMatchesRegex(t, path, `EVOLVE_OBSERVER_NUDGE_S[^|]*300`) {
		t.Errorf("RED: docs/operations/runtime-reference.md EVOLVE_OBSERVER_NUDGE_S row\n"+
			"does not contain '300'.\n"+
			"Builder must change the default column from '`0` (opt-in)' to\n"+
			"'`300` (opt-out via `=0`)' and update the description accordingly.\n"+
			"File: %s", path)
	}
}

// acs-predicate: config-check
func TestC353_003_ObserverStallSIsNotInternal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, path, `EVOLVE_OBSERVER_STALL_S", Status: StatusInternal`) {
		t.Errorf("RED: registry_table.go still contains StatusInternal for EVOLVE_OBSERVER_STALL_S.\n"+
			"Builder must promote all 4 Observer flags to StatusActive.\n"+
			"File: %s", path)
	}
}

func TestC353_004_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := strings.TrimSpace(out + "\n" + errOut)
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after updating registry_table.go\n"+
			"to regenerate the flag index in control-flags.md.",
			code, err, combined)
	}
}
