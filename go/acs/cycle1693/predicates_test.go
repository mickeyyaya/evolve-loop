//go:build acs

// Package cycle1693 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `iteration-state-coherence-sentinel`
// (.evolve/inbox/processing/cycle-1693/2026-07-22T14-02-00Z-iteration-state-coherence-sentinel.json).
//
// THE DEFECT. unfinishedCycle (go/cmd/evolve/cmd_loop_control.go) is the only
// guard that reads canonical cycle-state as possibly stale, and it runs once
// per batch in prepareFreshBatch. loopBatchCoordinator.prepareIteration
// (go/cmd/evolve/cmd_loop_window.go) — the chokepoint cmd_loop_batch.go runs
// before EVERY dispatch, sequential and fleet — never reads it. A fleet lane
// SIGKILLed by cmd_fleet.go's WaitDelay escalation skips its abnormalEpilogue
// state floor, so the canonical record keeps claiming a live phase for a dead
// cycle the batch already passed (CycleID <= lastCycleNumber — invisible to
// unfinishedCycle).
//
// WHY THESE PREDICATES SHELL A FROZEN IN-PACKAGE SUITE. prepareIteration is an
// unexported method of package main: no external package can call it, and a
// predicate that built the binary could not seed a SIGKILLed lane's state
// between two iterations. The behavioral contract therefore lives in the
// frozen, TDD-authored file
// go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go, which drives the
// REAL chokepoint (and the production entrypoint runLoop) against seeded
// canonical state. Each predicate below runs that suite (ONE named package,
// -run narrowed to the exact frozen names, -race per AC2) and requires the
// exact `--- PASS: <name> (` marker for every test and subtest it binds — a
// renamed, deleted or skipped test cannot pass vacuously, because
// `go test -run` matching nothing still exits 0.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - POSITIVE : 001 (stale record → WARN + aborted, three residue shapes).
//   - NEGATIVE : 002 (resumable / live-owner / completed / terminal records
//     byte-identical, no write, no WARN) — the over-correction guards, GREEN on
//     the unpatched tree by design and required to STAY green.
//   - EDGE/OOD : 005 (unreadable record never overwritten; read and write
//     errors surface on the loop console, never swallowed).
//   - WIRING   : 003 (runLoop entrypoint + wave + pool configs, under -race).
//   - SCOPE    : 004 (production diff confined to the three iteration-top
//     files, protected control-plane surfaces untouched, non-vacuous).
package cycle1693

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "./cmd/evolve"
	frozenFile   = "go/cmd/evolve/cmd_loop_iteration_state_coherence_test.go"
)

// Frozen test names, grouped by the criterion each binds.
var (
	positiveTests = []string{
		"TestPrepareIteration_StaleCanonicalState",
		"TestPrepareIteration_StaleCanonicalState/sigkilled_lane_fresh_heartbeat_dead_owner",
		"TestPrepareIteration_StaleCanonicalState/leaseless_record",
		"TestPrepareIteration_StaleCanonicalState/killed_mid_retro",
	}
	negativeTests = []string{
		"TestPrepareIteration_ResumableCycleUntouched",
		"TestPrepareIteration_ResumableCycleUntouched/dead_owner_lease=true",
		"TestPrepareIteration_ResumableCycleUntouched/dead_owner_lease=false",
		"TestPrepareIteration_OwnInFlightCycleUntouched",
		"TestPrepareIteration_CompletedCycleRecordUntouched",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched/fresh_tree",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched/already_aborted",
		"TestPrepareIteration_TerminalOrFreshRecordUntouched/end_marker",
	}
	wiringTests = []string{
		"TestPrepareIteration_WiredBeforeEveryIteration",
		"TestPrepareIteration_WiredBeforeEveryIteration/sequential_runLoop_entrypoint",
		"TestPrepareIteration_WiredBeforeEveryIteration/fleet_wave_config",
		"TestPrepareIteration_WiredBeforeEveryIteration/fleet_pool_config",
	}
	errorTests = []string{
		"TestPrepareIteration_CoherenceReadErrorSurfacesWithoutWrite",
		"TestPrepareIteration_ReconcileWriteErrorSurfaces",
	}
)

var (
	suiteOnce sync.Once
	suiteOut  string
	suiteCode int
	suiteErr  error
)

// frozenSuite runs the frozen contract ONCE for every predicate:
// `go -C <root>/go test -race -count=1 -v -run '^(top-level names)$' ./cmd/evolve`.
// One named package, -run narrowed (cmd/evolve is a known-slow suite; the
// narrowing is what keeps this cheap), -race because AC2 demands it.
func frozenSuite(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	suiteOnce.Do(func() {
		var tops []string
		seen := map[string]bool{}
		for _, group := range [][]string{positiveTests, negativeTests, wiringTests, errorTests} {
			for _, n := range group {
				top := strings.SplitN(n, "/", 2)[0]
				if !seen[top] {
					seen[top] = true
					tops = append(tops, top)
				}
			}
		}
		pattern := "^(" + strings.Join(tops, "|") + ")$"
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "-C", filepath.Join(root, "go"), "test", "-race", "-count=1", "-v", "-run", pattern, cmdEvolvePkg)
		suiteOut, suiteCode, suiteErr = stdout+stderr, code, err
	})
	if suiteErr != nil && suiteCode == -1 {
		t.Fatalf("could not run go test: %v", suiteErr)
	}
	return suiteOut
}

// requirePass demands the exact PASS marker for each bound test and no FAIL
// marker for it. The trailing " (" pins the exact name, so a parent's marker
// never satisfies a subtest and vice versa.
func requirePass(t *testing.T, out string, names []string) {
	t.Helper()
	for _, n := range names {
		if strings.Contains(out, "--- FAIL: "+n+" (") {
			t.Errorf("RED: %s FAILED", n)
			continue
		}
		if !strings.Contains(out, "--- PASS: "+n+" (") {
			t.Errorf("RED: %s did not PASS (missing, renamed, skipped or not compiled)", n)
		}
	}
	if t.Failed() {
		t.Logf("frozen suite output (exit=%d):\n%s", suiteCode, tail(out, 120))
	}
}

func tail(s string, lines int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n")
}

// TestC1693_001_StaleCanonicalStateReconciledWithWarn — AC1 (positive half).
// A canonical record claiming a live phase for a dead cycle the batch already
// passed is, at the iteration top, WARNed on the loop console (naming the
// cycle and phase) and reconciled with the epilogue's state floor
// (Phase="aborted", ActiveAgent cleared, identity preserved) — for the
// SIGKILL shape (fresh heartbeat, dead owner pid), a leaseless record, and the
// phase=retro residue; a second iteration top is quiet (idempotent).
func TestC1693_001_StaleCanonicalStateReconciledWithWarn(t *testing.T) {
	requirePass(t, frozenSuite(t), positiveTests)
}

// TestC1693_002_ResumableAndLiveRecordsUntouched — AC1 (negative half) plus
// the in-flight-lane safety rule. The unfinishedCycle (resumable) shape, a
// record whose run lease is held by a LIVE owner, a cleanly completed lane's
// closed-out record, and terminal/fresh records are all left byte-identical
// with zero writes and no WARN. Green on the unpatched tree by design; binds
// the fix against over-correction.
func TestC1693_002_ResumableAndLiveRecordsUntouched(t *testing.T) {
	requirePass(t, frozenSuite(t), negativeTests)
}

// TestC1693_003_WiredAtIterationTopForEveryPath — AC2. The check runs on the
// loopBatchCoordinator.prepareIteration call path itself: through the
// production entrypoint runLoop (reconcile lands before the batch's own cycle
// writes any state) and at a non-first iteration under both the wave and the
// pool fleet schedulers — all under -race, with no data race reported.
func TestC1693_003_WiredAtIterationTopForEveryPath(t *testing.T) {
	out := frozenSuite(t)
	requirePass(t, out, wiringTests)
	if strings.Contains(out, "WARNING: DATA RACE") {
		t.Errorf("RED: the race detector reported a data race in the frozen suite:\n%s", tail(out, 120))
	}
}

// TestC1693_004_ScopeConfinedToIterationTopFiles — AC3. Every production .go
// file this lane changes is one of the three iteration-top files; the
// protected control-plane surfaces are untouched; the fix actually landed in
// the allowed surface (non-vacuous); and the frozen contract is git-tracked
// (an untracked test is dropped at ship — cycle-93).
func TestC1693_004_ScopeConfinedToIterationTopFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	allowed := map[string]bool{
		"go/cmd/evolve/cmd_loop_window.go":         true,
		"go/cmd/evolve/cmd_loop_blockerbreaker.go": true,
		"go/cmd/evolve/cmd_loop_control.go":        true,
	}
	protected := []string{
		"go/internal/loopwave/",
		"go/cmd/evolve/cmd_loop_wave.go",
		"go/cmd/evolve/cmd_loop_chain.go",
		"go/internal/core/cyclerun.go",
		"go/internal/core/cyclerun_dispatch.go",
	}

	changed := changedPaths(t, root)
	landed := 0
	for _, p := range changed {
		for _, prot := range protected {
			if p == prot || (strings.HasSuffix(prot, "/") && strings.HasPrefix(p, prot)) {
				t.Errorf("RED: protected control-plane surface touched: %s", p)
			}
		}
		if !strings.HasPrefix(p, "go/") || !strings.HasSuffix(p, ".go") {
			continue // cycle bookkeeping (dossiers, evals, docs) is out of scope here
		}
		if strings.HasSuffix(p, "_test.go") {
			if !strings.HasPrefix(p, "go/cmd/evolve/") && !strings.HasPrefix(p, "go/acs/cycle1693/") {
				t.Errorf("RED: test file outside the lane's scope changed: %s", p)
			}
			continue
		}
		if !allowed[p] {
			t.Errorf("RED: production file outside go/cmd/evolve/cmd_loop_{window,blockerbreaker,control}.go changed: %s", p)
			continue
		}
		landed++
	}
	if landed == 0 {
		t.Errorf("RED: no change to cmd_loop_window.go / cmd_loop_blockerbreaker.go / cmd_loop_control.go — the fix has not landed in the iteration-top surface (changed: %v)", changed)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", frozenFile); code != 0 {
		t.Errorf("RED: %s is not git-tracked — the frozen contract would be dropped at ship", frozenFile)
	}
}

// TestC1693_005_CoherenceErrorsSurfaceLoudly — AC1 edge (fail loudly). An
// unreadable canonical record is reported on the loop console and never
// overwritten; a failed reconcile write is reported, not dropped.
func TestC1693_005_CoherenceErrorsSurfaceLoudly(t *testing.T) {
	requirePass(t, frozenSuite(t), errorTests)
}

// changedPaths is the lane's change set: committed since the fork point with
// main (the nearest merge-base across main / origin/main, so a lagging remote
// ref never drags sibling lanes' landed work in) plus the working tree.
func changedPaths(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	add := func(p string) {
		if p = strings.TrimSpace(p); p != "" {
			seen[p] = true
		}
	}

	var bases []string
	for _, ref := range []string{"main", "origin/main"} {
		if mb, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "HEAD", ref); code == 0 {
			bases = append(bases, strings.TrimSpace(mb))
		}
	}
	if len(bases) == 0 {
		t.Fatalf("no merge-base with main or origin/main in %s", root)
	}
	base := bases[0]
	for _, cand := range bases[1:] {
		// Prefer the nearer fork point: cand is nearer when base is its ancestor.
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "merge-base", "--is-ancestor", base, cand); code == 0 {
			base = cand
		}
	}
	out, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base, "HEAD")
	if code != 0 {
		t.Fatalf("git diff %s HEAD: exit %d: %s", base, code, errOut)
	}
	for _, line := range strings.Split(out, "\n") {
		add(line)
	}

	status, errOut, code, _ := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all")
	if code != 0 {
		t.Fatalf("git status: exit %d: %s", code, errOut)
	}
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		p := line[3:]
		if i := strings.Index(p, " -> "); i >= 0 { // rename: count the destination
			p = p[i+4:]
		}
		add(p)
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
