//go:build acs

// Package flagceiling is the ACS monotonic-decrease guard for the flag-reduction
// campaign. It fails the per-cycle gate if the count of live operator-facing
// feature flags (flagregistry.LiveFeatureFlags = StatusActive minus
// core-infrastructure) ROSE versus the campaign baseline (main).
//
// Why a guard and not just the LiveFeatureFlagCeiling unit ratchet: a same-metric
// unit test (count <= const) can be defeated by editing the const in the same
// diff — exactly how cycle-5 raised FlagCeiling 47->48 to absorb a net addition.
// This guard derives the baseline from git history (origin/main, else main), so a
// cycle cannot grant itself headroom by editing in-tree files.
//
// Fail-OPEN when no baseline ref is reachable (offline / shallow clone): the
// in-tree LiveFeatureFlagCeiling ratchet remains the floor and CI / the per-cycle
// worktree (full clone) always have the ref, so the teeth bite where it matters.
package flagceiling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// registryTableRepoPath is the registry data file, relative to the repo root.
const registryTableRepoPath = "go/internal/flagregistry/registry_table.go"

// countLiveInSource counts live feature-flag rows in registry_table.go source:
// a row line (trimmed prefix "{Name:") that is StatusActive and is NOT
// core-infrastructure. It mirrors flagregistry.LiveFeatureFlags over raw source
// so an older git blob can be counted without linking it into the binary.
// TestBaselineCounter_AgreesWithStructOnHEAD pins it to the struct counter.
func countLiveInSource(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "{Name:") {
			continue
		}
		if !strings.Contains(t, "Status: StatusActive") {
			continue
		}
		if strings.Contains(t, flagregistry.ClusterCoreInfra) {
			continue
		}
		n++
	}
	return n
}

// baselineLiveCount returns the live-feature-flag count on the campaign baseline
// ref. ok=false (fail-open) when neither origin/main nor main is reachable, or
// git itself is unavailable.
func baselineLiveCount() (count int, ref string, ok bool) {
	// `git show <ref>:<path>` already exits non-zero when the ref (or the path on
	// that ref) is absent or git is unavailable, so it is both the existence check
	// and the read — no separate rev-parse guard needed.
	for _, r := range []string{"origin/main", "main"} {
		stdout, _, code, err := acsassert.SubprocessOutput("git", "show", r+":"+registryTableRepoPath)
		if err != nil || code != 0 {
			continue
		}
		return countLiveInSource(stdout), r, true
	}
	return 0, "", false
}

// TestLiveFeatureFlags_DoesNotExceedBaseline is the monotonic-decrease ratchet:
// the live count may fall (progress) or hold, but never rise versus main.
func TestLiveFeatureFlags_DoesNotExceedBaseline(t *testing.T) {
	current := len(flagregistry.LiveFeatureFlags())
	baseline, ref, ok := baselineLiveCount()
	if !ok {
		t.Skip("no campaign baseline ref (origin/main|main) reachable; ratchet enforced by LiveFeatureFlagCeiling + CI")
	}
	if current > baseline {
		t.Errorf("live feature flags ROSE %d -> %d versus baseline (%s) — the campaign ratchet is one-way; "+
			"deprecate flags (env read -> policy.json/DI), never add operator dials", baseline, current, ref)
	}
	t.Logf("live feature flags: current=%d baseline=%d (%s)", current, baseline, ref)
}

// TestBaselineCounter_AgreesWithStructOnHEAD pins the raw-source line counter to
// flagregistry.LiveFeatureFlags on the current tree, so a future change to the
// registry_table.go row format that breaks countLiveInSource fails loudly here
// rather than silently miscounting the baseline.
func TestBaselineCounter_AgreesWithStructOnHEAD(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, registryTableRepoPath))
	if err != nil {
		t.Fatalf("read %s: %v", registryTableRepoPath, err)
	}
	if bySource, byStruct := countLiveInSource(string(src)), len(flagregistry.LiveFeatureFlags()); bySource != byStruct {
		t.Errorf("source line-counter (%d) disagrees with LiveFeatureFlags struct (%d) on HEAD — "+
			"registry_table.go row format changed; update countLiveInSource", bySource, byStruct)
	}
}
