//go:build acs

package flagceiling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const registryTableRepoPath = "go/internal/flagregistry/registry_table.go"

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

func baselineLiveCount() (count int, ref string, ok bool) {
	for _, r := range []string{"origin/main", "main"} {
		stdout, _, code, err := acsassert.SubprocessOutput("git", "show", r+":"+registryTableRepoPath)
		if err != nil || code != 0 {
			continue
		}
		return countLiveInSource(stdout), r, true
	}
	return 0, "", false
}

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
