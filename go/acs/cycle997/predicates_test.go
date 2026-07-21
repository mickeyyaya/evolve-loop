//go:build acs

// Package cycle997 materialises the cycle-997 acceptance criteria for the
// fleet-scoped inbox item `carryover-consolidation-sweep`, which triage split
// into three dependency-chained committed tasks:
//
//   - carryover-decisions-authoring     → the judgment artifact
//   - carryover-apply-consolidation-cli → the sanctioned apply path
//   - carryover-sweep-group-filer       → the surviving-items re-filing
//
// The inbox item asks for a ONE-TIME judgment pass over
// state.json:carryoverTodos (135 live entries this cycle): drop stale failure
// echoes + landed duplicate shadows, and cluster genuine small items into
// 4–6-item sweep-group inbox filings, shrinking the array toward ~25. The TTL
// prune machinery already exists (failurelog.PruneExpiredCarryoverTodos, wired
// in cmd_loop.go) but only removes entries whose expiresAt is already past — it
// cannot perform the semantic keep/drop/cluster judgment. THAT judgment, plus a
// sanctioned locked-RMW path to apply it and the re-filing of survivors, is the
// gap these three tasks close.
//
// Predicate strategy — each predicate exercises a REAL emitted artifact or the
// system-under-test, never a source-grep of production code (the cycle-85
// degenerate-predicate ban):
//
//   - 001 / 002 parse the emitted decisions JSON artifact and cross-reference it
//     against the LIVE .evolve/state.json id set — a source-independent data
//     binding: the file must actually classify the real carryover population,
//     not merely contain a magic string.
//   - 003 shells `go test -race` over the DEFAULT build suite for the four
//     binding tests Builder must author against the new `evolve carryover`
//     subcommand, requiring a `--- PASS: <name>` line for each (the cycle-987
//     behavioural-via-subprocess precedent). A still-missing command, an
//     unregistered subcommand, an un-rejected missing-reason entry, or an apply
//     that bypasses the flock RMW path each leaves one PASS line absent → RED.
//   - 004 parses the emitted sweep-group inbox JSON files and cross-references
//     them against the decisions file's `cluster` entries: exact 1:1 coverage,
//     group sizing 4–6, weight band 0.7–0.8.
//
// Root resolution mirrors the established cycle-84/86 regression predicates:
// artifacts are read under acsassert.RepoRoot (the worktree, where Builder
// writes them per worktree-isolation), and a genuinely-absent runtime file is a
// SKIP, never a false PASS — but the decisions file / sweep files / CLI tests
// are Builder deliverables, so they must be PRESENT and correct at audit time.
package cycle997

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// decisionsRelPath is the emitted judgment artifact Task 1 (carryover-decisions-
// authoring) must produce. Slug/date match scout-report's targetFiles.
const decisionsRelPath = ".evolve/carryover-decisions-2026-07-21.json"

// cmdEvolvePkg is the DEFAULT-suite package that owns the new subcommand + its
// binding tests (Task 2).
const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

// carryoverDecision mirrors the on-disk schema Task 1 must emit. cluster_group
// is required ONLY when decision=="cluster" (enforced below), so it is a plain
// string that is "" for keep/drop entries.
type carryoverDecision struct {
	ID           string `json:"id"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	ClusterGroup string `json:"cluster_group"`
}

type decisionsFile struct {
	SourceCount int                 `json:"source_count"`
	Decisions   []carryoverDecision `json:"decisions"`
}

// loadDecisions reads + parses the emitted decisions artifact under the worktree
// root. It t.Fatalf's (RED) when the file is absent or unparseable — unlike a
// pure runtime-state file, this artifact is a mandatory Builder deliverable, so
// its absence is a failure, not a skip.
func loadDecisions(t *testing.T) decisionsFile {
	t.Helper()
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, decisionsRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("decisions artifact %s not readable (Task 1 carryover-decisions-authoring must emit it): %v", decisionsRelPath, err)
	}
	var df decisionsFile
	if err := json.Unmarshal(raw, &df); err != nil {
		t.Fatalf("decisions artifact %s is not valid JSON: %v", decisionsRelPath, err)
	}
	if len(df.Decisions) == 0 {
		t.Fatalf("decisions artifact %s has an empty `decisions` array", decisionsRelPath)
	}
	return df
}

// liveCarryoverIDs reads the id set of state.json:carryoverTodos under the
// worktree root. Returns (ids, true) on success; (nil, false) when state.json is
// genuinely absent (SKIP-worthy — cannot verify coverage without the source
// population), following the cycle-84/86 regression-predicate convention.
func liveCarryoverIDs(t *testing.T) (map[string]bool, bool) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	if err != nil {
		return nil, false
	}
	var state struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}
	ids := make(map[string]bool, len(state.CarryoverTodos))
	for _, e := range state.CarryoverTodos {
		if e.ID != "" {
			ids[e.ID] = true
		}
	}
	return ids, true
}

// TestC997_001_CarryoverDecisionsWellFormedAndPruning — AC1 (Task 1). The
// judgment artifact must be internally valid AND actually delete a substantial
// stale population: every entry carries a valid decision enum
// (keep|drop|cluster) and a non-empty reason, ids are unique, each `cluster`
// entry names a non-empty cluster_group, and the file records >= 60 `drop`
// decisions (the inbox item's convergence target: 135 → ~25 requires dropping
// the ~104 unpicked>=15 / 26 stale-echo population, of which >= 60 is a
// deliberately conservative floor). A no-op "keep everything" file — which would
// leave the priority-inversion unaddressed — fails the drop floor.
func TestC997_001_CarryoverDecisionsWellFormedAndPruning(t *testing.T) {
	df := loadDecisions(t)

	const validEnum = "keep|drop|cluster"
	seen := make(map[string]bool, len(df.Decisions))
	drops := 0
	for i, d := range df.Decisions {
		if d.ID == "" {
			t.Errorf("decision[%d] has empty id", i)
			continue
		}
		if seen[d.ID] {
			t.Errorf("decision id %q appears more than once (must be 1:1)", d.ID)
		}
		seen[d.ID] = true

		switch d.Decision {
		case "keep", "drop", "cluster":
			// valid
		default:
			t.Errorf("decision id %q has invalid decision %q (want one of %s)", d.ID, d.Decision, validEnum)
		}
		if strings.TrimSpace(d.Reason) == "" {
			t.Errorf("decision id %q has empty reason (every classification must justify itself)", d.ID)
		}
		if d.Decision == "drop" {
			drops++
		}
		if d.Decision == "cluster" && strings.TrimSpace(d.ClusterGroup) == "" {
			t.Errorf("decision id %q is `cluster` but names no cluster_group", d.ID)
		}
	}

	const minDrops = 60
	if drops < minDrops {
		t.Errorf("decisions file records only %d `drop` decisions; want >= %d to converge the 135-entry array toward ~25", drops, minDrops)
	}
}

// TestC997_002_CarryoverDecisionsCoverEveryLiveEntry — AC1 (Task 1), the
// data-binding half. Every id currently in state.json:carryoverTodos MUST have
// exactly one decision row — no live entry may be left unclassified. This is the
// predicate that makes the artifact real: it cannot be satisfied by a plausible
// hand-written stub that omits the actual population; it must enumerate the true
// carryover ids. (Robust to Task 2's apply shrinking the live array: survivors
// are `keep` entries, still covered.)
func TestC997_002_CarryoverDecisionsCoverEveryLiveEntry(t *testing.T) {
	live, ok := liveCarryoverIDs(t)
	if !ok {
		t.Skip("state.json absent — cannot verify decision coverage against the live carryover population")
	}
	if len(live) == 0 {
		t.Skip("state.json carryoverTodos is empty — nothing to classify")
	}
	df := loadDecisions(t)
	classified := make(map[string]bool, len(df.Decisions))
	for _, d := range df.Decisions {
		classified[d.ID] = true
	}
	missing := 0
	for id := range live {
		if !classified[id] {
			missing++
			if missing <= 10 {
				t.Errorf("live carryoverTodos id %q has no decision row (unclassified)", id)
			}
		}
	}
	if missing > 10 {
		t.Errorf("... and %d more unclassified live ids (total %d of %d uncovered)", missing-10, missing, len(live))
	}
}

// assertRaceSuiteTestsPass shells `go test -run '^(names)$' -race -count=1 -v pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to print a
// `--- PASS: <name>` line. Asserting on the PASS line (not merely exit 0) is
// essential: `go test -run` on a pattern matching no test exits 0 with "no tests
// to run", so a still-missing binding test would otherwise false-GREEN. -count=1
// defeats the cache; -race satisfies Task 2's `-race` acceptance criterion.
func assertRaceSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-race", "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// TestC997_003_CarryoverApplyCLIBoundAndRaceClean — AC2 (Task 2). The new
// `evolve carryover apply-decisions` subcommand and its guarantees are bound by
// four DEFAULT-suite, -race-clean tests Builder must author against the real
// command (not stubs). Each named test drives a distinct load-bearing property:
//
//   - TestCarryoverSubcommandRegistered — `carryover` is wired into the CLI
//     dispatch table (an unregistered command is dead code).
//   - TestCarryoverApplyDecisions_DropsEntriesToCeiling — applying a fixture
//     decisions set to a fixture state.json removes the `drop` ids and lands the
//     live count at/under the ceiling (the actual convergence behaviour).
//   - TestCarryoverApplyDecisions_RejectsMissingReason — NEGATIVE: a decision
//     entry with an empty reason is rejected and state.json is left UNMUTATED
//     (the anti-hand-edit / anti-unjustified-drop guard).
//   - TestCarryoverApplyDecisions_UsesLockedRMW — the write goes through the
//     sanctioned flock.WithPathLock(statePath, …) RMW path (no ad-hoc unlocked
//     state.json write), matching the cmd_loop.go / reset.go single-writer
//     contract.
func TestC997_003_CarryoverApplyCLIBoundAndRaceClean(t *testing.T) {
	assertRaceSuiteTestsPass(t, cmdEvolvePkg,
		"TestCarryoverSubcommandRegistered",
		"TestCarryoverApplyDecisions_DropsEntriesToCeiling",
		"TestCarryoverApplyDecisions_RejectsMissingReason",
		"TestCarryoverApplyDecisions_UsesLockedRMW",
	)
}

// sweepInboxFile mirrors the fields a sweep-group inbox filing must carry (Task
// 3). `items` is the list of clustered carryover ids the group amortises.
type sweepInboxFile struct {
	ID     string   `json:"id"`
	Weight float64  `json:"weight"`
	Kind   string   `json:"kind"`
	Items  []string `json:"items"`
}

// TestC997_004_SweepGroupsCoverEveryClusterExactlyOnce — AC3 (Task 3). Every
// `cluster` decision from Task 1's artifact must land in EXACTLY ONE
// sweep-group inbox file (no orphan, no duplicate), each group must hold 4–6
// items, and each group's weight must sit in the 0.7–0.8 band the inbox item
// specifies. This cross-references two emitted artifacts (decisions file ⋈ sweep
// files) — a genuine end-to-end binding, not a source-grep.
func TestC997_004_SweepGroupsCoverEveryClusterExactlyOnce(t *testing.T) {
	df := loadDecisions(t)
	wantCluster := make(map[string]bool)
	for _, d := range df.Decisions {
		if d.Decision == "cluster" {
			wantCluster[d.ID] = true
		}
	}
	if len(wantCluster) == 0 {
		t.Skip("decisions file classifies nothing as `cluster` — no sweep groups expected")
	}

	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*carryover-sweep*.json"))
	if err != nil {
		t.Fatalf("glob sweep inbox files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no .evolve/inbox/*carryover-sweep*.json files emitted, but %d ids are classified `cluster` (Task 3 must file them)", len(wantCluster))
	}

	covered := make(map[string]string) // clusterID -> sweep file that claims it
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read sweep file %s: %v", filepath.Base(path), err)
			continue
		}
		var sf sweepInboxFile
		if err := json.Unmarshal(raw, &sf); err != nil {
			t.Errorf("sweep file %s is not valid JSON: %v", filepath.Base(path), err)
			continue
		}
		if n := len(sf.Items); n < 4 || n > 6 {
			t.Errorf("sweep file %s has %d items; sweep groups must hold 4–6", filepath.Base(path), n)
		}
		if sf.Weight < 0.7 || sf.Weight > 0.8 {
			t.Errorf("sweep file %s has weight %.3f; must be in the 0.7–0.8 band", filepath.Base(path), sf.Weight)
		}
		for _, id := range sf.Items {
			if prev, dup := covered[id]; dup {
				t.Errorf("cluster id %q appears in two sweep files (%s and %s) — must be exactly once", id, filepath.Base(prev), filepath.Base(path))
			}
			covered[id] = path
		}
	}

	for id := range wantCluster {
		if _, ok := covered[id]; !ok {
			t.Errorf("cluster id %q is classified `cluster` but appears in no sweep-group inbox file (orphan)", id)
		}
	}
	for id := range covered {
		if !wantCluster[id] {
			t.Errorf("sweep files reference id %q which is NOT classified `cluster` in the decisions file", id)
		}
	}
}
