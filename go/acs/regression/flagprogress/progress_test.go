//go:build acs

// Package flagprogress is the ACS strict-reduction guard for the flag-reduction
// campaign. During an active campaign (EVOLVE_FLAG_CAMPAIGN=1) it fails the
// per-cycle gate when a cycle did NOT delete at least one registry row — i.e.
// len(flagregistry.All) at the working tree is not strictly less than at HEAD
// (the cycle's parent commit).
//
// Why this guard exists: the sibling flagceiling guard only blocks the live
// count from RISING; nothing required it to FALL. A cycle could therefore do a
// plausible refactor (e.g. rename a reader to a "EVOLVE_"+"X" split-const) that
// makes the diff/tests/audit/adversarial-review all pass while netting ZERO row
// deletions — exactly how relaunch cycle 10 (w2-phaserecovery-ipc) shipped a PASS
// with rows 35 -> 35. Gating the METRIC (len(All)) is unforgeable in a way that
// gating the diff shape is not: the one thing a cosmetic change cannot fake is
// the row count going down.
//
// Activation: keyed off EVOLVE_FLAG_CAMPAIGN=1, set at campaign launch and
// inherited by the per-cycle `go test -tags acs` subprocess. Dormant (skip)
// everywhere else, so normal main/dev cycles are unaffected. The env literal is
// read only here in an acs _test.go, which the flagreaders guard excludes, so it
// needs no registry row of its own.
//
// Fail-CLOSED during an active campaign (ADR-0064 M3): if HEAD's registry is
// unreachable while EVOLVE_FLAG_CAMPAIGN=1, the cycle is QUARANTINED (failed),
// not skipped — a campaign cycle runs in a full clone where HEAD is always
// reachable, so an unreachable baseline is anomalous and must not silently pass.
// Outside a campaign the gate is dormant regardless of git reachability.
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

// countRowsInSource counts registry rows in registry_table.go source by the
// "{Name:" row prefix — mirrors len(flagregistry.All) over raw source so an
// older git blob can be counted without linking it into the binary.
func countRowsInSource(src string) int {
	n := 0
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "{Name:") {
			n++
		}
	}
	return n
}

// --- pure-logic unit tests (TDD) ---

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
		// cycle-10 failure mode: no rows deleted (35 -> 35) must be flagged.
		{"no change", 35, 35, false},
		// a regression (count rose) is also not progress.
		{"rose", 36, 35, false},
		// a real deletion (35 -> 34) is progress.
		{"one deleted", 34, 35, true},
		// a multi-row deletion (35 -> 23) is progress.
		{"multi deleted", 23, 35, true},
	}
	for _, tc := range cases {
		if got := tc.current < tc.parent; got != tc.want {
			t.Errorf("%s: current=%d < parent=%d = %v, want %v", tc.name, tc.current, tc.parent, got, tc.want)
		}
	}
}

// reductionVerdict is the strict-reduction gate's decision for one cycle.
type reductionVerdict int

const (
	reductionDormant    reductionVerdict = iota // campaign inactive — skip
	reductionQuarantine                         // campaign active, parent unreachable — fail-closed (M3)
	reductionNoProgress                         // campaign active, no net row deletion — fail
	reductionProgress                           // campaign active, strict reduction — pass
)

// classifyReduction decides the gate outcome. M3 (ADR-0064 Pillar 2): during an
// active campaign an unreachable parent is QUARANTINE, not a skip — the prior
// fail-open let a cycle bypass the strict-reduction check by making HEAD's
// registry unreadable. Outside a campaign the gate is dormant regardless.
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

// TestClassifyReduction pins the gate's decision logic, including the M3
// fail-closed case: an active campaign with an unreachable parent must QUARANTINE
// (not skip), because a campaign cycle runs in a full clone where HEAD is always
// reachable — an unreachable baseline means integrity cannot be verified.
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

// --- the acs guard ---

// parentRowCount returns the registry row count at HEAD (the cycle's parent
// commit). ok=false when HEAD or git is unavailable — during an ACTIVE campaign
// this triggers fail-closed quarantine (see classifyReduction), not a skip.
func parentRowCount() (count int, ok bool) {
	stdout, _, code, err := acsassert.SubprocessOutput("git", "show", "HEAD:"+registryTableRepoPath)
	if err != nil || code != 0 {
		return 0, false
	}
	return countRowsInSource(stdout), true
}

// TestFlagCampaignCycle_StrictlyReducesRegistry is the missing-signal gate.
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
		// Fatalf (not Errorf): stop here so the success log below cannot also fire
		// on the failure path and contradict the verdict in CI output.
		t.Fatalf("flag-campaign cycle made NO net registry reduction: rows HEAD=%d -> worktree=%d "+
			"(must strictly decrease). A conversion that does not DELETE the flag row is not progress — "+
			"delete the row from %s, and the cycle's own predicate must assert flagregistry.Lookup returns false.",
			parent, current, registryTableRepoPath)
	case reductionProgress:
		t.Logf("flag-campaign reduction OK: rows %d -> %d", parent, current)
	}
}

// TestRowCounter_AgreesWithStructOnHEAD pins countRowsInSource to
// len(flagregistry.All) on the current tree, so a registry_table.go row-format
// change that breaks the counter fails loudly rather than silently miscounting.
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
