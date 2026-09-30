//go:build acs

package flagprogress

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const registryTableRepoPath = "go/internal/flagregistry/registry_table.go"

const campaignEnvKey = "EVOLVE_FLAG_CAMPAIGN"

func countRowsInSource(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "{Name:") {
			n++
		}
	}
	return n
}

func TestCountRowsInSource_CountsNameRows(t *testing.T) {
	src := `var All = []Flag{
	{Name: "EVOLVE_A", Status: StatusActive},
	{Name: "EVOLVE_B", Status: StatusInternal},
	{Name: "EVOLVE_C", Status: StatusDeprecated},
}`
	if got := countRowsInSource(src); got != 3 {
		t.Errorf("countRowsInSource = %d, want 3", got)
	}
}

func TestCountRowsInSource_StrictReduction(t *testing.T) {
	cases := []struct {
		name    string
		current int
		parent  int
		want    bool
	}{
		{"no change", 35, 35, false},
		{"rose", 36, 35, false},
		{"one deleted", 34, 35, true},
		{"multi deleted", 23, 35, true},
	}
	for _, tc := range cases {
		if got := tc.current < tc.parent; got != tc.want {
			t.Errorf("%s: current=%d < parent=%d = %v, want %v", tc.name, tc.current, tc.parent, got, tc.want)
		}
	}
}

type reductionVerdict int

const (
	reductionDormant reductionVerdict = iota
	reductionQuarantine
	reductionNoProgress
	reductionProgress
)

func classifyReduction(campaignActive, parentReachable bool, current, parent int) reductionVerdict {
	if !campaignActive {
		return reductionDormant
	}
	if !parentReachable {
		return reductionQuarantine
	}
	if current >= parent {
		return reductionNoProgress
	}
	return reductionProgress
}

func TestClassifyReduction(t *testing.T) {
	cases := []struct {
		name                     string
		campaignActive, parentOK bool
		current, parent          int
		want                     reductionVerdict
	}{
		{"dormant when no campaign", false, true, 35, 35, reductionDormant},
		{"dormant even if parent unreachable", false, false, 0, 0, reductionDormant},
		{"M3 fail-closed: active + parent unreachable", true, false, 0, 0, reductionQuarantine},
		{"no progress: no change", true, true, 35, 35, reductionNoProgress},
		{"no progress: rose", true, true, 36, 35, reductionNoProgress},
		{"progress: one deleted", true, true, 34, 35, reductionProgress},
		{"progress: multi deleted", true, true, 23, 35, reductionProgress},
	}
	for _, tc := range cases {
		if got := classifyReduction(tc.campaignActive, tc.parentOK, tc.current, tc.parent); got != tc.want {
			t.Errorf("%s: classifyReduction(%v,%v,%d,%d) = %v, want %v",
				tc.name, tc.campaignActive, tc.parentOK, tc.current, tc.parent, got, tc.want)
		}
	}
}

func parentRowCount() (count int, ok bool) {
	stdout, _, code, err := acsassert.SubprocessOutput("git", "show", "HEAD:"+registryTableRepoPath)
	if err != nil || code != 0 {
		return 0, false
	}
	return countRowsInSource(stdout), true
}

func TestFlagCampaignCycle_StrictlyReducesRegistry(t *testing.T) {
	campaignActive := os.Getenv(campaignEnvKey) == "1"
	var parent int
	var parentOK bool
	if campaignActive {
		parent, parentOK = parentRowCount()
	}
	current := len(flagregistry.All)

	switch classifyReduction(campaignActive, parentOK, current, parent) {
	case reductionDormant:
		t.Skipf("%s != 1; strict-reduction gate dormant (not an active flag campaign)", campaignEnvKey)
	case reductionQuarantine:
		t.Fatalf("flag-campaign active but HEAD registry (%s) is unreachable — QUARANTINING the cycle "+
			"(fail-closed, ADR-0064 M3). A campaign cycle runs in a full clone where HEAD is always "+
			"reachable; an unreachable baseline means the strict-reduction check cannot run, so the cycle "+
			"must not pass. (Outside a campaign this gate is dormant.)", registryTableRepoPath)
	case reductionNoProgress:
		t.Fatalf("flag-campaign cycle made NO net registry reduction: rows HEAD=%d -> worktree=%d "+
			"(must strictly decrease). A conversion that does not DELETE the flag row is not progress — "+
			"delete the row from %s, and the cycle's own predicate must assert flagregistry.Lookup returns false.",
			parent, current, registryTableRepoPath)
	case reductionProgress:
		t.Logf("flag-campaign reduction OK: rows %d -> %d", parent, current)
	}
}

func TestRowCounter_AgreesWithStructOnHEAD(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, registryTableRepoPath))
	if err != nil {
		t.Fatalf("read %s: %v", registryTableRepoPath, err)
	}
	if bySource, byStruct := countRowsInSource(string(src)), len(flagregistry.All); bySource != byStruct {
		t.Errorf("source row-counter (%d) disagrees with flagregistry.All (%d) on HEAD — "+
			"registry_table.go row format changed; update countRowsInSource", bySource, byStruct)
	}
}
