//go:build acs

// Package cycle308 materializes the cycle-308 acceptance criteria for the three
// committed top_n tasks (scout-report.md):
//
//	T1  inbox-promote-on-ship-missing — ReleaseCycleProcessing scoped-releases
//	    processing/cycle-<N>/ back to the inbox root; wired into BOTH the
//	    ship-success residual drain (postship.go) and the cycle-fail terminal
//	    (cmd_loop.go) so claimed items never strand (the cycle-124/234/240/...
//	    orphan class).
//	T2  companion-malformed-must-surface — a present-but-malformed
//	    triage-decision.json companion SURFACES a non-empty warning instead of
//	    silently falling through to the prose scanner; absent file / absent field
//	    stay silent (backward compat).
//	T3  cli-version-lifecycle-preflight — looppreflight captures a CLI version
//	    inventory into loop-preflight.json and WARNs on a version change vs the
//	    last batch (the claude 2.1.173 → 2.1.175 silent-drift incident).
//
// These predicates are BEHAVIORAL (cycle-85 lesson). The load-bearing checks RUN
// the system under test: they invoke `go test -v` on the white-box package tests
// that call the real functions (ReleaseCycleProcessing, MalformedCommittedFloor
// Warning, captureVersionInventory, checkCLIVersionDrift) and assert on the real
// `--- PASS:` / `--- FAIL:` lines. A magic string in a .go file can neither make
// a release move a file, make a malformed companion surface a parse error, nor
// make a drift check WARN — so none is gameable by source editing alone. The
// cmd_loop.go wiring check (C308_003) is MIXED: a behavioral inboxmover run plus
// an auxiliary grep that the production call site exists (cycle-307 seam-trap).
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1 fail-release       → C308_001 (named PASS lines, inboxmover)
//	T1 ship residual      → C308_002 (named PASS line, ship)
//	T1 cmd_loop wiring    → C308_003 (behavioral + wiring grep)
//	T2 committed surface  → C308_004 (named PASS lines, triagecap)
//	T2 deferred surface   → C308_005 (named PASS lines, triagecap)
//	T3 inventory + drift   → C308_006 (named PASS lines, looppreflight)
package cycle308

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the module dir; `go test -C <goDir>` makes every invocation
// cwd-independent (the audit lane may run from the worktree root or go/).
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var (
	passLineRe = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
	anyFailRe  = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
)

// topLevelPassed reports whether a `--- PASS: <name>` line names exactly `name`.
func topLevelPassed(out, name string) bool {
	for _, m := range passLineRe.FindAllStringSubmatch(out, -1) {
		if m[1] == name {
			return true
		}
	}
	return false
}

func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// --- shared one-shot subprocess runners (one `go test` per scope, reused) ---

var (
	inboxOnce  sync.Once
	inboxOut   string
	shipOnce   sync.Once
	shipOut    string
	triageOnce sync.Once
	triageOut  string
	preOnce    sync.Once
	preOut     string
)

func runInbox(t *testing.T) string {
	t.Helper()
	inboxOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestReleaseCycleProcessing|TestInboxRelease", "./internal/inboxmover/")
		inboxOut = stdout + "\n" + stderr
	})
	return inboxOut
}

func runShip(t *testing.T) string {
	t.Helper()
	shipOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestInboxPromote_UnfinishedClaimReleasedOnShip", "./internal/phases/ship/")
		shipOut = stdout + "\n" + stderr
	})
	return shipOut
}

func runTriage(t *testing.T) string {
	t.Helper()
	triageOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestCommittedFloorCount_Malformed|TestCommittedFloorCount_Absent|TestDeferredFloorPackagesDecl_Malformed|TestDeferredFloorPackagesDecl_Absent",
			"./internal/triagecap/")
		triageOut = stdout + "\n" + stderr
	})
	return triageOut
}

func runPreflight(t *testing.T) string {
	t.Helper()
	preOnce.Do(func() {
		stdout, stderr, _, _ := acsassert.SubprocessOutput(
			"go", "test", "-C", goDir(t), "-count=1", "-v",
			"-run", "TestCLIVersionInventory|TestVersionDrift", "./internal/looppreflight/")
		preOut = stdout + "\n" + stderr
	})
	return preOut
}

// requirePass asserts each named test produced a `--- PASS:` line and no test
// FAILed (a compile failure or a real RED produces neither → caught here).
func requirePass(t *testing.T, out string, names ...string) {
	t.Helper()
	if anyFailRe.MatchString(out) {
		t.Errorf("RED/REGRESSION: a gated test FAILed:\n%s", tail(out, 40))
	}
	for _, n := range names {
		if !topLevelPassed(out, n) {
			t.Errorf("RED: missing `--- PASS: %s` (build failure or assertion not met):\n%s", n, tail(out, 40))
		}
	}
}

// ===================== T1 — inbox-promote-on-ship-missing ====================

// C308_001 (T1 fail-release): ReleaseCycleProcessing scope + fail-release +
// double-move WARN are exercised against the real filesystem.
func TestC308_001_InboxReleaseCycleProcessing(t *testing.T) {
	out := runInbox(t)
	requirePass(t, out,
		"TestReleaseCycleProcessing_ReleasesScopedCycle",
		"TestInboxRelease_FailedCycleReleasesAllClaimed",
		"TestInboxRelease_DoubleClaimRaceIsWarn",
	)
}

// C308_002 (T1 ship residual): a claimed-but-dropped item is drained back to the
// inbox root on a successful ship (real promoteInbox over a real temp tree).
func TestC308_002_InboxResidualReleasedOnShip(t *testing.T) {
	out := runShip(t)
	requirePass(t, out, "TestInboxPromote_UnfinishedClaimReleasedOnShip")
}

// C308_003 (T1 cmd_loop wiring): MIXED. The behavioral portion re-runs the real
// inboxmover release test; the auxiliary grep confirms the production call site
// is wired into cmd_loop.go's cycle-fail path — the cycle-307 seam-trap guard (a
// helper built but never wired). The grep alone is auxiliary: the behavioral
// inboxmover run carries the weight.
func TestC308_003_CmdLoopFailTerminalWiresRelease(t *testing.T) {
	out := runInbox(t)
	requirePass(t, out, "TestInboxRelease_FailedCycleReleasesAllClaimed")

	cmdLoop := filepath.Join(acsassert.RepoRoot(t), "go", "cmd", "evolve", "cmd_loop.go")
	if !acsassert.FileContains(t, cmdLoop, "ReleaseCycleProcessing") {
		t.Errorf("RED: cmd_loop.go does not call inboxmover.ReleaseCycleProcessing — the cycle-fail terminal release is unwired (cycle-307 seam trap)")
	}
}

// ===================== T2 — companion-malformed-must-surface =================

// C308_004 (T2 committed): a malformed committed_floors companion surfaces; an
// absent companion stays silent and falls back to prose.
func TestC308_004_CommittedFloorMalformedSurfaces(t *testing.T) {
	out := runTriage(t)
	requirePass(t, out,
		"TestCommittedFloorCount_MalformedFieldSurfaces",
		"TestCommittedFloorCount_AbsentCompanionFallsBackSilently",
	)
}

// C308_005 (T2 deferred): same three-case separation for deferred_floors.
func TestC308_005_DeferredFloorMalformedSurfaces(t *testing.T) {
	out := runTriage(t)
	requirePass(t, out,
		"TestDeferredFloorPackagesDecl_MalformedFieldSurfaces",
		"TestDeferredFloorPackagesDecl_AbsentFieldFallsBackSilently",
	)
}

// ===================== T3 — cli-version-lifecycle-preflight ==================

// C308_006 (T3 inventory + drift): version capture lands in loop-preflight.json
// and the drift check WARNs on the synthetic 2.1.173 → 2.1.175 transition while
// staying quiet on unchanged / no-prior-record.
func TestC308_006_VersionInventoryAndDrift(t *testing.T) {
	out := runPreflight(t)
	requirePass(t, out,
		"TestCLIVersionInventory",
		"TestCLIVersionInventory_LandsInPreflight",
		"TestVersionDrift_Fires_On_Synthetic_Transition",
		"TestVersionDrift_NoWarnWhenVersionUnchanged",
		"TestVersionDrift_NoWarnWhenNoPriorRecord",
	)
}
