//go:build acs

// Package cycle986 materializes the cycle-986 acceptance criteria for the sole
// fleet lane this cycle is pinned to: recover-false-fail-features-876-897-898
// (goal 1f6d5bf8…). Per R9.3 no predicate here binds to any other lane's items.
//
// Task nature: CONVERGENCE-CONFIRMATION + CLOSEOUT. Scout found the three
// features the clean-exit false-FAIL bug discarded across cycles 862–899 —
// tier-fallback (876), skill-overlay/`/evo:fable` injection (897/884),
// scoped-review (898) — are ALREADY landed on main and green (tier-fallback via
// PR #331 6b4e4096; skill-overlay via PR #333 daf993e8; scoped-review in
// internal/core). The lane has converged. The only residual value is retiring
// the stale inbox item (weight 0.93) that keeps re-selecting completed work and
// stamping the recovery ledger CLOSED with the two real landing SHAs — the exact
// livelock class ADR-0072 exists to end.
//
// The Builder's deliverable (the RED contract — do NOT modify this file):
//
//	B1  remove the stale inbox JSON
//	    `.evolve/inbox/2026-07-17T14-35-00Z-recover-false-fail-features-876-897-898.json`
//	    from the STATE root (main's `.evolve/inbox/`; `.evolve/inbox` is
//	    untracked shared state — writable outside the worktree per the
//	    boundary-only-main-tree-writes rule). Suite reads it via
//	    EVOLVE_PROJECT_ROOT (the STATE root); see stateRoot below.
//	B2  stamp `docs/operations/false-fail-recovery-862-899.md` (a SOURCE doc, so
//	    edited in the WORKTREE) with CLOSED + both landing SHAs 6b4e4096 /
//	    daf993e8, WITHOUT dropping the genuine-FAIL (889/894/895/896)
//	    do-not-land classification.
//
// AC map (1:1 with scout-report.md ## Acceptance Criteria Summary):
//
//	AC1 "stale inbox JSON gone from active root"
//	    → C986_001 (negative/absence): the specific item is absent from the live
//	      inbox root AND no live inbox *.json still carries the lane id. RED now
//	      (the item is present in both the worktree and main inbox roots).
//	AC2 "ledger records CLOSED + both landing SHAs (bare CLOSED fails)"
//	    → C986_002: ledger contains CLOSED and BOTH SHAs, and each cited SHA is
//	      VERIFIED against git as a real landing commit whose subject names the
//	      recovered feature (tier-fallback / skill-overlay). This is what lifts
//	      the check above a degenerate magic-string grep (cycle-85 rule): a bare
//	      "CLOSED", a missing SHA, or a fabricated/wrong SHA all fail. RED now
//	      (ledger status is "QUEUED"; neither SHA appears).
//	AC3 "all 3 recovered features remain green"
//	    → C986_003 (behavioral): runs the recovered features' own tests as
//	      subprocesses across runner/bridge/guards/core and requires each named
//	      top-level PASS marker (a -run that matches zero tests exits 0 — that
//	      gaming vector is rejected by counting the markers). PRE-EXISTING GREEN
//	      by design: this IS the confirmation half of a convergence task, bound
//	      so audit re-proves the landing instead of trusting the scout.
//	AC4 "negative: the genuine FAILs (889/894/895/896) are not resurrected"
//	    → C986_004 (negative/semantic): closing the ledger must NOT flip the
//	      genuine FAILs to recovered — the ledger must STILL classify
//	      889/894/895/896 as GENUINE FAIL / do-not-land, and no live inbox item
//	      may propose LANDING them. Guards the real over-closure regression this
//	      edit risks. Currently GREEN; stays green iff the Builder preserves the
//	      do-not-land record.
//	AC5 "`go vet ./internal/...` clean"
//	    → C986_005 (behavioral): runs `go vet ./internal/...` and requires exit
//	      0. PRE-EXISTING GREEN by design.
//
// Adversarial axes: negative (C986_001 absence; C986_004 no-resurrection),
// edge (C986_003 rejects exit-0-with-zero-matched-tests), semantic (retire vs
// stamp vs preserve-genuine-FAIL are distinct behaviors). No degenerate
// source-grep predicates: C986_002 verifies the cited SHAs against real git
// history, C986_003/005 execute the system under test, C986_001/004 assert on
// real runtime/emitted artifacts (the live inbox, the ledger record).
package cycle986

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	laneID     = "recover-false-fail-features-876-897-898"
	inboxItem  = "2026-07-17T14-35-00Z-recover-false-fail-features-876-897-898.json"
	ledgerRel  = "docs/operations/false-fail-recovery-862-899.md"
	tierSHA    = "6b4e4096" // PR #331 — tier-fallback dispatch swap
	overlaySHA = "daf993e8" // PR #333 — skill-overlay injection
	// modPrefix is the Go module import prefix; subprocess `go test`/`go vet`
	// runs with cwd == this package dir, so a relative `./internal/...` target
	// does not resolve — use fully-qualified import paths, which work from
	// anywhere inside the module.
	modPrefix = "github.com/mickeyyaya/evolve-loop/go/"
)

// genuineFailCycles are the cycles whose on-disk verdict was a REAL auditor FAIL
// (zero-coverage / live-reproduced defect / cross-lane contamination). Closing
// the recovery ledger must NOT reclassify these as recovered.
var genuineFailCycles = []string{"889", "894", "895", "896"}

// stateRoot resolves the MAIN project root (the STATE root): the ACS suite
// exports EVOLVE_PROJECT_ROOT (issue #12) so `.evolve/` runtime data resolves to
// main, not the worktree; else fall back to the repo root (the redteam idiom).
func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

// AC1: the stale inbox item is gone from the LIVE inbox root and no other live
// item still re-proposes the lane. Only top-level `.evolve/inbox/*.json` are
// live proposals (processed/ items never re-surface). RED until the Builder
// removes the item from the state root.
func TestC986_001_stale_inbox_item_retired(t *testing.T) {
	inboxDir := filepath.Join(stateRoot(t), ".evolve", "inbox")

	if acsassert.FileExists(nopTB{}, filepath.Join(inboxDir, inboxItem)) {
		t.Errorf("RED: stale inbox item %s still lives in the active root %s — "+
			"triage will keep re-selecting completed work (the ADR-0072 livelock class)",
			inboxItem, inboxDir)
	}

	live, err := filepath.Glob(filepath.Join(inboxDir, "*.json"))
	if err != nil {
		t.Fatalf("glob live inbox: %v", err)
	}
	for _, f := range live {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read live inbox item %s: %v", f, err)
			continue
		}
		if strings.Contains(string(data), `"id": "`+laneID+`"`) {
			t.Errorf("RED: live inbox item %s still proposes %s — closure did not stick", f, laneID)
		}
	}
}

// AC2: the ledger is stamped CLOSED and cites BOTH landing SHAs, and each cited
// SHA is a REAL commit whose subject names the recovered feature. Bare "CLOSED",
// a missing SHA, or a fabricated/wrong SHA all fail — this is what makes the
// check exercise the system (git history) rather than accept a magic string.
// RED until the Builder stamps the ledger.
func TestC986_002_ledger_stamped_closed_with_real_landing_shas(t *testing.T) {
	ledger := filepath.Join(acsassert.RepoRoot(t), ledgerRel) // SOURCE doc → worktree

	if !acsassert.FileExists(t, ledger) {
		return // FileExists already failed with the path
	}
	if !acsassert.FileContains(t, ledger, "CLOSED") {
		t.Errorf("RED: ledger is not stamped CLOSED (scout found status still QUEUED)")
	}

	for feature, sha := range map[string]string{
		"tier-fallback": tierSHA,
		"skill-overlay": overlaySHA,
	} {
		if !acsassert.FileContains(t, ledger, sha) {
			t.Errorf("RED: ledger does not cite the %s landing SHA %s (bare CLOSED is insufficient)", feature, sha)
			continue
		}
		verifyRealLandingCommit(t, sha, feature)
	}
}

// verifyRealLandingCommit rejects a fabricated or wrong SHA: the cited commit
// must exist AND its subject must name the recovered feature keyword.
func verifyRealLandingCommit(t *testing.T, sha, featureKeyword string) {
	t.Helper()
	typ, _, code, err := acsassert.SubprocessOutput("git", "cat-file", "-t", sha)
	if err != nil || code != 0 || strings.TrimSpace(typ) != "commit" {
		t.Errorf("RED: cited SHA %s does not resolve to a real commit (type=%q code=%d err=%v) — fabricated evidence",
			sha, strings.TrimSpace(typ), code, err)
		return
	}
	subj, _, code, err := acsassert.SubprocessOutput("git", "log", "-1", "--format=%s", sha)
	if err != nil || code != 0 {
		t.Errorf("RED: cannot read subject of cited SHA %s (code=%d err=%v)", sha, code, err)
		return
	}
	if !strings.Contains(strings.ToLower(subj), featureKeyword) {
		t.Errorf("RED: cited SHA %s subject %q does not name the %q feature — wrong landing commit",
			sha, strings.TrimSpace(subj), featureKeyword)
	}
}

// featureSuite is one recovered feature's behavioral confirmation: its package,
// a -run filter, and the top-level PASS markers that MUST appear (so a filter
// matching zero tests — exit 0 with no PASS — is rejected).
type featureSuite struct {
	feature   string
	pkg       string
	runFilter string
	mustPASS  []string
}

// AC3: every recovered feature's own tests are green in THIS tree. Behavioral —
// executes the SUT and counts each named PASS marker. PRE-EXISTING GREEN by
// design (this is a convergence-confirmation task).
func TestC986_003_recovered_features_remain_green(t *testing.T) {
	suites := []featureSuite{
		{
			feature:   "tier-fallback (876)",
			pkg:       modPrefix + "internal/phases/runner",
			runFilter: "TierFallback|Tiered|QuotaExhausted",
			mustPASS:  []string{"TestRun_QuotaExhaustedAcrossChain_NeverStepsDownTier"},
		},
		{
			feature:   "skill-overlay injection (897)",
			pkg:       modPrefix + "internal/adapters/bridge",
			runFilter: "InjectSkillOverlays|Launch_InjectsSkillOverlay|Launch_MissingSkill|Launch_NoSkills",
			mustPASS: []string{
				"TestInjectSkillOverlays_PrependsPersonaAboveBody",
				"TestLaunch_InjectsSkillOverlay",
				"TestLaunch_MissingSkill_StillDispatches",
				"TestLaunch_NoSkills_ByteIdenticalDefault",
			},
		},
		{
			feature:   "fable ProtectedSurface gate (884)",
			pkg:       modPrefix + "internal/guards",
			runFilter: "TestProtectedSurface_FableSkillOverlay",
			mustPASS:  []string{"TestProtectedSurface_FableSkillOverlay"},
		},
		{
			feature:   "scoped-review (898)",
			pkg:       modPrefix + "internal/core",
			runFilter: "Scoped",
			mustPASS: []string{
				"TestScopedReview_SeesOnlyIntersectingHunks",
				"TestScopedReview_MalformedDiffFailsClosed",
				"TestOrchestrator_ScopedMergeReviewWired",
			},
		},
	}

	for _, s := range suites {
		stdout, stderr, code, err := acsassert.SubprocessOutput(
			"go", "test", s.pkg, "-run", s.runFilter, "-count=1", "-v")
		if code != 0 || err != nil {
			t.Errorf("recovered feature %s: `go test %s -run %s` exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
				s.feature, s.pkg, s.runFilter, code, err, stdout, stderr)
			continue
		}
		for _, name := range s.mustPASS {
			if !strings.Contains(stdout, "--- PASS: "+name) {
				t.Errorf("recovered feature %s: predicate %s did not report PASS (renamed, skipped, or the -run matched nothing)",
					s.feature, name)
			}
		}
	}
}

// AC4 (negative): closing the ledger must NOT resurrect the genuine FAILs. The
// ledger must STILL classify 889/894/895/896 as GENUINE FAIL / do-not-land, and
// no live inbox item may propose LANDING those cycles. Guards the over-closure
// regression this edit risks.
func TestC986_004_genuine_fails_not_resurrected(t *testing.T) {
	ledger := filepath.Join(acsassert.RepoRoot(t), ledgerRel)
	if !acsassert.FileExists(t, ledger) {
		return
	}
	// The do-not-land record must survive closure.
	if !acsassert.FileContainsAny(ledger, "GENUINE FAIL", "genuine FAIL", "do **NOT** land", "do NOT land") {
		t.Errorf("RED-guard: ledger no longer records that the genuine FAILs are do-not-land — " +
			"closure over-reached and erased the real-defect classification")
	}
	for _, c := range genuineFailCycles {
		if !acsassert.FileContains(t, ledger, c) {
			t.Errorf("RED-guard: ledger no longer references genuine-FAIL cycle %s — its record was dropped during closure", c)
		}
	}

	// No live inbox item may propose recovering/landing the genuine FAILs.
	live, err := filepath.Glob(filepath.Join(stateRoot(t), ".evolve", "inbox", "*.json"))
	if err != nil {
		t.Fatalf("glob live inbox: %v", err)
	}
	for _, f := range live {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read live inbox item %s: %v", f, err)
			continue
		}
		s := string(data)
		if !strings.Contains(s, "recover") && !strings.Contains(s, "land") {
			continue
		}
		for _, c := range genuineFailCycles {
			// A recovery/landing proposal that targets a genuine-FAIL cycle id.
			if strings.Contains(s, `"`+c+`"`) || strings.Contains(s, "recover-"+c) || strings.Contains(s, "land-"+c) {
				t.Errorf("RED-guard: live inbox item %s proposes recovering/landing genuine-FAIL cycle %s", f, c)
			}
		}
	}
}

// AC5: `go vet ./internal/...` is clean in this tree. Behavioral. PRE-EXISTING
// GREEN by design.
func TestC986_005_internal_packages_vet_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", modPrefix+"internal/...")
	if code != 0 || err != nil {
		t.Errorf("`go vet ./internal/...` is not clean: exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
}

// nopTB is a throwaway TB for a pure-boolean FileExists probe (absence is the
// GREEN state here, so the built-in Errorf-on-missing must be suppressed).
type nopTB struct{}

func (nopTB) Helper()                       {}
func (nopTB) Errorf(string, ...interface{}) {}
func (nopTB) Fatalf(string, ...interface{}) {}
