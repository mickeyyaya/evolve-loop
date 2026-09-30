//go:build acs

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
	tierSHA    = "6b4e4096"
	overlaySHA = "daf993e8"
	modPrefix  = "github.com/mickeyyaya/evolve-loop/go/"
)

var genuineFailCycles = []string{"889", "894", "895", "896"}

func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

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

func TestC986_002_ledger_stamped_closed_with_real_landing_shas(t *testing.T) {
	ledger := filepath.Join(acsassert.RepoRoot(t), ledgerRel)

	if !acsassert.FileExists(t, ledger) {
		return
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

type featureSuite struct {
	feature   string
	pkg       string
	runFilter string
	mustPASS  []string
}

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

func TestC986_004_genuine_fails_not_resurrected(t *testing.T) {
	ledger := filepath.Join(acsassert.RepoRoot(t), ledgerRel)
	if !acsassert.FileExists(t, ledger) {
		return
	}
	if !acsassert.FileContainsAny(ledger, "GENUINE FAIL", "genuine FAIL", "do **NOT** land", "do NOT land") {
		t.Errorf("RED-guard: ledger no longer records that the genuine FAILs are do-not-land — " +
			"closure over-reached and erased the real-defect classification")
	}
	for _, c := range genuineFailCycles {
		if !acsassert.FileContains(t, ledger, c) {
			t.Errorf("RED-guard: ledger no longer references genuine-FAIL cycle %s — its record was dropped during closure", c)
		}
	}

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
			if strings.Contains(s, `"`+c+`"`) || strings.Contains(s, "recover-"+c) || strings.Contains(s, "land-"+c) {
				t.Errorf("RED-guard: live inbox item %s proposes recovering/landing genuine-FAIL cycle %s", f, c)
			}
		}
	}
}

func TestC986_005_internal_packages_vet_clean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", modPrefix+"internal/...")
	if code != 0 || err != nil {
		t.Errorf("`go vet ./internal/...` is not clean: exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}
}

type nopTB struct{}

func (nopTB) Helper()                       {}
func (nopTB) Errorf(string, ...interface{}) {}
func (nopTB) Fatalf(string, ...interface{}) {}
