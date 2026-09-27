//go:build acs

// Package cycle1720 materializes the acceptance criteria for this fleet lane's
// two triage-committed tasks under inbox id triage-unified-solution-synthesis:
//
//   - transactional-unified-consume → the ship's in-commit consumption closes a
//     VALIDATED unified_commitment all-or-nothing: if any member cannot close,
//     every member stays pickable and one WARN names the commitment and the
//     failing member; ordinary ids keep the per-item fail-open contract.
//   - unified-commitment-validation-tests → processUnifiedCommitment gets direct
//     unit tests for its five branches, and those tests must DETECT a
//     regression in the branch each one names.
//
// consumeCommittedItems and processUnifiedCommitment are unexported, so the
// behavioral proofs are named in-package tests run as one-package `go test`
// subprocesses (the cycle-1507 shape): each predicate asserts the exact
// `--- PASS:` lines, never the exit code (`go test -run` over a pattern that
// matches nothing exits 0). The consume tests are TDD-authored and frozen;
// 004 drives the real cycle ship (shipFromWorktree over a real git worktree),
// so the landing commit itself is the evidence. For the test-only task the
// predicate is mutation-based (007): each branch of the production code is
// broken through `go test -overlay` — no byte of the tree changes — and the
// Builder's subtest for that branch must go RED.
//
// Flaky-shape hygiene: every subprocess names ONE package and narrows with
// -run, no wall-clock bounds, no literal PIDs, every git call is -C rooted.
package cycle1720

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTest runs `go test -count=1 -v` for one package from the module root with
// the extra flags, returning the combined output.
func goTest(t *testing.T, pkg string, flags ...string) string {
	t.Helper()
	args := append([]string{"test", "-count=1", "-v"}, flags...)
	cmd := exec.Command("go", append(args, pkg)...)
	cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// requirePass fails unless the output carries a `--- PASS:` line for the top
// test and for every named subtest, and no `--- FAIL:` line at all.
func requirePass(t *testing.T, out, top string, subtests ...string) {
	t.Helper()
	want := []string{"--- PASS: " + top + " "}
	for _, s := range subtests {
		want = append(want, "--- PASS: "+top+"/"+strings.ReplaceAll(s, " ", "_")+" ")
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("RED: missing %q", strings.TrimSpace(w))
		}
	}
	if strings.Contains(out, "--- FAIL:") || t.Failed() {
		t.Errorf("output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// transactional-unified-consume
// ---------------------------------------------------------------------------

const shipPkg = "./internal/phases/ship"

func TestC1720_001_UnifiedMembersRollBackTogetherOnPartialFailure(t *testing.T) {
	const top = "TestConsumeCommittedItems_UnifiedRollsBackAllOnPartialFailure"
	out := goTest(t, shipPkg, "-run", "^"+top+"$")
	requirePass(t, out, top,
		"stage failure after a sibling staged rolls the sibling back",
		"unresolvable member rolls every sibling back",
		"failing member first leaves later siblings unconsumed")
}

func TestC1720_002_UnifiedMembersAllConsumeWhenEveryMemberCloses(t *testing.T) {
	const top = "TestConsumeCommittedItems_UnifiedAllSucceed"
	requirePass(t, goTest(t, shipPkg, "-run", "^"+top+"$"), top)
}

func TestC1720_003_AtomicUnitIsExactlyTheValidatedMemberSet(t *testing.T) {
	const top = "TestConsumeCommittedItems_UnifiedScopeIsOnlyTheValidatedMembers"
	out := goTest(t, shipPkg, "-run", "^"+top+"$")
	requirePass(t, out, top,
		"an ordinary top_n batch stays per-item fail-open",
		"a claim triage rejected carries no projection and stays per-item",
		"an independent item still consumes beside a rolled-back commitment")
}

func TestC1720_004_LandingCommitCarriesAllOrNoneOfTheMembers(t *testing.T) {
	const top = "TestShipFromWorktree_UnifiedCommitmentClosesAllOrNothing"
	out := goTest(t, shipPkg, "-tags", "integration", "-run", "^"+top+"$")
	requirePass(t, out, top,
		"a member that cannot close keeps every member out of the landing commit",
		"members that all close ride the landing commit together")
}

// existingConsumeContract is the pre-existing consume regression family; the
// per-item fail-open default must survive the unified change unmodified.
var existingConsumeContract = []string{
	"TestShipFromWorktree_ConsumptionDriftIsSanctioned",
	"TestShipFromWorktree_UnsanctionedDriftStillRefuses",
	"TestShipFromWorktree_SecondUnconsumedInboxFileRefuses",
	"TestShipFromWorktree_TamperedItemIsNotSanctioned",
	"TestShipFromWorktree_NoBindingStillSkipsCheck",
	"TestCheckEGPSGate_WarnWithZeroRedCountNeverReachesConsumption",
	"TestShipFromWorktree_ConsumesCommittedItemInTheShipCommit",
	"TestShipFromWorktree_WarnVerdictDoesNotConsume",
	"TestShipFromWorktree_WarnWithRedsDoesNotConsume",
	"TestShipFromWorktree_MissingItemIsANoOp",
	"TestShipDirect_ConsumesCommittedItemFromHead",
	"TestConsume_ResolvesIDsLikePostShip",
	"TestManualShip_NonShippableVerdictKeepsItemPickable",
	"TestConsumeGate_OnlyVerdictPASSConsumes",
	"TestCommittedInboxIDs_UnionsLaneScopeWhenTriageDroppedTheScope",
	"TestCommittedInboxIDs_UnionsLaneScopeWithDecomposedTopN",
	"TestCommittedInboxIDs_TriageDeferredScopeIDStaysPickable",
	"TestCommittedInboxIDs_NilBodyStillResolvesLaneScope",
	"TestCommittedInboxIDs_DeclinedMenuUnmentionedScopeStaysPickable",
	"TestCommittedInboxIDs_PendingMenuMateStaysPickable",
	"TestCommittedInboxIDs_DropReasonGate",
	"TestCommittedInboxIDs_WarnLandingGetsNoScopeUnion",
	"TestInternalConsumedPaths_OnlyWrittenInConsumeGo",
	"TestConsumeCommittedItems_ReleasesRegistryBinding",
	"TestConsumeCommittedItems_NoBindingIsCleanNoOp",
	"TestManualShip_ClosesInboxItemInShipCommit",
}

func TestC1720_005_ExistingConsumeContractStaysGreen(t *testing.T) {
	out := goTest(t, shipPkg, "-tags", "integration", "-run", "^("+strings.Join(existingConsumeContract, "|")+")$")
	for _, name := range existingConsumeContract {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("pre-existing consume test %s did not PASS — the per-item default must be unchanged", name)
		}
	}
	if strings.Contains(out, "--- FAIL:") || t.Failed() {
		t.Errorf("output:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// unified-commitment-validation-tests
// ---------------------------------------------------------------------------

const (
	triagePkg     = "./internal/phases/triage"
	unifiedTopRun = "^TestProcessUnifiedCommitment"
)

// branchSubtests are the five branches of processUnifiedCommitment the unit
// test must name (hyphen or underscore separators both match).
var branchSubtests = []string{"accept-small", "accept-large", "reject-outside-top_n", "reject-heterogeneous", "reject-spoofed"}

// subtestLine matches a `--- <verdict>:` line for a TestProcessUnifiedCommitment*
// subtest whose path ends with the branch slug.
func subtestLine(verdict, slug string) *regexp.Regexp {
	sep := regexp.MustCompile(`[-_]`)
	parts := sep.Split(slug, -1)
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`--- ` + verdict + `: TestProcessUnifiedCommitment\S*/\S*` + strings.Join(parts, `[-_]`) + ` `)
}

func TestC1720_006_ProcessUnifiedCommitmentUnitTestsPassOnFiveBranches(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "phases", "triage", "unified_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing — processUnifiedCommitment still has no direct unit test", rel)
	}
	out := goTest(t, triagePkg, "-run", unifiedTopRun)
	for _, slug := range branchSubtests {
		if !subtestLine("PASS", slug).MatchString(out) {
			t.Errorf("RED: no passing TestProcessUnifiedCommitment subtest for branch %q", slug)
		}
	}
	if strings.Contains(out, "--- FAIL:") || t.Failed() {
		t.Errorf("output:\n%s", out)
	}
}

// branchMutant breaks exactly one branch of production code — through an
// overlay, never on disk — while keeping it compilable.
type branchMutant struct {
	slug, file, anchor, mutant string
}

var branchMutants = []branchMutant{
	{"accept-small", "internal/phases/triage/unified.go",
		`fields["unified_projection"] = projection`, `_ = projection`},
	{"accept-large", "internal/phases/triage/unified.go",
		`if claim.Size() == "large" {`, `if false && claim.Size() == "large" {`},
	{"reject-outside-top_n", "internal/phases/triage/unified.go",
		`if !committed[member.ID] {`, `if false && !committed[member.ID] {`},
	{"reject-heterogeneous", "internal/inboxbatch/unified.go",
		`if len(campaigns) > 1 {`, `if false && len(campaigns) > 1 {`},
	{"reject-heterogeneous", "internal/inboxbatch/unified.go",
		`if len(kinds) > 1 {`, `if false && len(kinds) > 1 {`},
	{"reject-spoofed", "internal/phases/triage/unified.go",
		`if _, spoofed := fields["unified_projection"]; spoofed {`, `if _, spoofed := fields["unified_projection"]; false && spoofed {`},
}

func TestC1720_007_ProcessUnifiedCommitmentUnitTestsKillBranchMutants(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	if !acsassert.FileExists(t, filepath.Join(goDir, "internal", "phases", "triage", "unified_test.go")) {
		t.Fatalf("RED: go/internal/phases/triage/unified_test.go missing — no test exists to kill a branch mutant")
	}
	// The heterogeneity mutant disables BOTH checks at once: a test pinning
	// either axis (campaigns or deliverable kinds) must notice.
	bySlug := map[string][]branchMutant{}
	for _, m := range branchMutants {
		bySlug[m.slug] = append(bySlug[m.slug], m)
	}
	for _, slug := range branchSubtests {
		t.Run(slug, func(t *testing.T) {
			replace := map[string]string{}
			sources := map[string]string{}
			for _, m := range bySlug[slug] {
				orig := filepath.Join(goDir, filepath.FromSlash(m.file))
				src, ok := sources[orig]
				if !ok {
					raw, err := os.ReadFile(orig)
					if err != nil {
						t.Fatalf("read %s: %v", m.file, err)
					}
					src = string(raw)
				}
				if n := strings.Count(src, m.anchor); n != 1 {
					t.Fatalf("mutation anchor %q occurs %d times in %s (want 1) — this task is test-only; the production branch it guards must not be rewritten", m.anchor, n, m.file)
				}
				sources[orig] = strings.Replace(src, m.anchor, m.mutant, 1)
			}
			dir := t.TempDir()
			for orig, src := range sources {
				mutated := filepath.Join(dir, filepath.Base(filepath.Dir(orig))+"-"+filepath.Base(orig))
				if err := os.WriteFile(mutated, []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
				replace[orig] = mutated
			}
			overlay, err := json.Marshal(map[string]any{"Replace": replace})
			if err != nil {
				t.Fatal(err)
			}
			overlayPath := filepath.Join(dir, "overlay.json")
			if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
				t.Fatal(err)
			}
			out := goTest(t, triagePkg, "-overlay="+overlayPath, "-run", unifiedTopRun)
			if strings.Contains(out, "[build failed]") || strings.Contains(out, "[setup failed]") {
				t.Fatalf("the %s mutant did not compile — predicate defect, not a test verdict:\n%s", slug, out)
			}
			if !subtestLine("FAIL", slug).MatchString(out) {
				t.Errorf("RED: with the %q branch broken, no TestProcessUnifiedCommitment subtest for that branch FAILed — the unit test does not guard it\n%s", slug, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ship-tree tracking (cycle-93 / cycle-1623 M1)
// ---------------------------------------------------------------------------

func TestC1720_008_CycleTestFilesAreGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"go/acs/cycle1720/predicates_test.go",
		"go/internal/phases/ship/consume_unified_test.go",
		"go/internal/phases/ship/consume_unified_integration_test.go",
		"go/internal/phases/triage/unified_test.go",
	} {
		if !acsassert.FileExists(t, filepath.Join(root, filepath.FromSlash(rel))) {
			t.Errorf("RED: %s missing on disk", rel)
			continue
		}
		if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
			t.Errorf("RED: %s is untracked (exit=%d err=%v) — it would be dropped from the ship tree", rel, code, err)
		}
	}
}
