package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func laneOneEightOneOne() mainTreeOwnership {
	return mainTreeOwnership{
		cycle: 1811,
		items: map[string]bool{"router-silent-errors": true, "router-digest-slug": true},
		mints: map[string]bool{"xlane-mint": true},
	}
}

func TestMainTreeOwnership_ForeignOwnerNamesEveryOwnerKeyedClass(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		foreign   bool
		ownerHint string
	}{
		{"own eval by committed id", ".evolve/evals/router-silent-errors.md", false, ""},
		{"own eval by materialized slug", ".evolve/evals/router-digest-slug.md", false, ""},
		{"sibling eval", ".evolve/evals/phasespec-roots-silent-policy-error.md", true, "phasespec-roots-silent-policy-error"},
		{"nested eval path is not slug-keyed", ".evolve/evals/archive/old.md", false, ""},
		{"non-markdown eval file is not slug-keyed", ".evolve/evals/helper.sh", false, ""},
		{"own predicate package", "go/acs/cycle1811/predicates_test.go", false, ""},
		{"sibling predicate package", "go/acs/cycle1812/predicates_test.go", true, "cycle 1812"},
		{"standing regression predicates are not cycle-keyed", "go/acs/regression/gate_test.go", false, ""},
		{"own change record", "docs/explain/builds/cycle-1811-01m48p8jh4bj25x1btqs19nssw.md", false, ""},
		{"sibling change record", "docs/explain/builds/cycle-1812-01m48p8jhdj97bg0evcbbetkyr.md", true, "cycle 1812"},
		{"a record of this cycle number is this cycle's", "docs/explain/builds/cycle-1811-01m48p8jhdj97bg0evcbbetkyr.md", false, ""},
		{"registered mint", ".evolve/phases/xlane-mint/phase.json", true, "mint"},
		{"unregistered phase config", ".evolve/phases/novel/phase.json", false, ""},
		{"source file", "go/internal/router/router.go", false, ""},
	}
	owner := laneOneEightOneOne()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, foreign := owner.foreignOwner(tc.path)
			if foreign != tc.foreign {
				t.Fatalf("foreignOwner(%q) foreign=%v, want %v (owner %q)", tc.path, foreign, tc.foreign, got)
			}
			if !strings.Contains(got, tc.ownerHint) || (tc.foreign && got == "") {
				t.Fatalf("foreignOwner(%q) owner=%q, want it to name %q", tc.path, got, tc.ownerHint)
			}
		})
	}
}

func TestMainTreeOwnership_UnknownIdentityClaimsLikeASingleWriter(t *testing.T) {
	unknown := mainTreeOwnership{}
	for _, p := range []string{
		".evolve/evals/phasespec-roots-silent-policy-error.md",
		"go/acs/cycle1812/predicates_test.go",
		"docs/explain/builds/cycle-1812-01m48p8jhdj97bg0evcbbetkyr.md",
		".evolve/phases/xlane-mint/phase.json",
	} {
		if owner, foreign := unknown.foreignOwner(p); foreign {
			t.Errorf("with no lane identity %q must stay claimable; got foreign owner %q", p, owner)
		}
	}
	if !unknown.ownsEval(".evolve/evals/anything.md") {
		t.Error("with no lane identity the lane owns every eval it finds (single-writer rule)")
	}
}

func TestMainTreeOwnership_OwnsEvalOnlyForTheLanesOwnSlugs(t *testing.T) {
	owner := laneOneEightOneOne()
	cases := map[string]bool{
		".evolve/evals/router-silent-errors.md":                true,
		".evolve/evals/router-digest-slug.md":                  true,
		".evolve/evals/phasespec-roots-silent-policy-error.md": false,
		"go/internal/router/router.go":                         false,
		".evolve/evals/archive/router-silent-errors.md":        false,
	}
	for p, want := range cases {
		if got := owner.ownsEval(p); got != want {
			t.Errorf("ownsEval(%q)=%v, want %v", p, got, want)
		}
	}
}

func TestLaneOwnership_ItemsAreTheCommittedSetPlusMaterializedEvals(t *testing.T) {
	workspace := t.TempDir()
	pin, err := json.Marshal(LaneScope{TodoIDs: []string{"pinned-item"}, GoalHash: "g"})
	if err != nil {
		t.Fatal(err)
	}
	writeTreeFile(t, workspace, LaneScopeFile, string(pin))
	writeTreeFile(t, workspace, "triage-decision.json", `{"top_n":[{"id":"working-sub-id"}]}`)
	writeTreeFile(t, workspace, ".evolve/evals/decomposed-slug.md", "eval\n")
	writeTreeFile(t, workspace, ".evolve/evals/notes.txt", "not an eval\n")
	mints := map[string]bool{"xlane-mint": true}

	got := laneOwnership(CycleState{CycleID: 1811, RunID: "01run", WorkspacePath: workspace}, mints)

	want := mainTreeOwnership{
		cycle: 1811,
		items: map[string]bool{"pinned-item": true, "decomposed-slug": true},
		mints: mints,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("laneOwnership = %+v, want %+v", got, want)
	}
}

func TestLaneOwnership_TriageDecisionBindsWhenNoPinAndDeferralsAreNotOwned(t *testing.T) {
	workspace := t.TempDir()
	writeTreeFile(t, workspace, "triage-decision.json",
		`{"top_n":[{"id":"committed-item"},{"id":"deferred-item"}],"deferred":[{"id":"deferred-item"}]}`)

	got := laneOwnership(CycleState{CycleID: 7, WorkspacePath: workspace}, nil)

	if want := map[string]bool{"committed-item": true}; !reflect.DeepEqual(got.items, want) {
		t.Fatalf("items = %v, want %v", got.items, want)
	}
}

func TestLaneOwnership_NoRecordedIdentityIsUnknownNotEmpty(t *testing.T) {
	got := laneOwnership(CycleState{CycleID: 7, WorkspacePath: t.TempDir()}, nil)

	if got.items != nil {
		t.Fatalf("a lane with no pin, no decision and no materialized eval has unknown items, got %v", got.items)
	}
}

func TestMainTreeOwnership_HeldBySiblingNeedsALiveHolder(t *testing.T) {
	siblings := liveSiblings{
		projectRoot: "/plane",
		runs: map[string]bool{
			filepath.Clean(RunWorkspacePath("/plane", 1812)): true,
			filepath.Clean(RunWorkspacePath("/plane", 1811)): true,
		},
		items: map[string]bool{"phasespec-roots-silent-policy-error": true, "router-silent-errors": true},
	}
	cases := map[string]bool{
		".evolve/evals/phasespec-roots-silent-policy-error.md":         true,
		".evolve/evals/made-up-slug.md":                                false,
		".evolve/evals/router-silent-errors.md":                        false,
		"go/acs/cycle1811/predicates_test.go":                          false,
		"docs/explain/builds/cycle-1811-01m48p8jh4bj25x1btqs19nssw.md": false,
		"go/acs/cycle1812/predicates_test.go":                          true,
		"go/acs/cycle1700/predicates_test.go":                          false,
		"docs/explain/builds/cycle-1812-01m48p8jhdj97bg0evcbbetkyr.md": true,
		"docs/explain/builds/cycle-1700-01m48p8jhdj97bg0evcbbetkyr.md": false,
		".evolve/phases/xlane-mint/phase.json":                         true,
		"go/internal/router/router.go":                                 false,
	}
	owner := laneOneEightOneOne()
	for p, want := range cases {
		if got := owner.heldBySibling(p, siblings); got != want {
			t.Errorf("heldBySibling(%q)=%v, want %v", p, got, want)
		}
	}
}

func writeTreeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRunLease(t *testing.T, runDir string, heartbeat time.Time) {
	t.Helper()
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: filepath.Base(runDir), OwnerPID: os.Getpid()}, heartbeat); err != nil {
		t.Fatal(err)
	}
}

func TestLiveSiblingsOf_ReadsOnlyLiveRunsOtherThanThisLane(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	own, live, stale, unleased := RunWorkspacePath(root, 1811), RunWorkspacePath(root, 1812), RunWorkspacePath(root, 1700), RunWorkspacePath(root, 1800)
	for dir, slug := range map[string]string{own: "router-silent-errors", live: "phasespec-roots-silent-policy-error", stale: "stale-item", unleased: "unleased-item"} {
		pin, err := json.Marshal(LaneScope{TodoIDs: []string{slug}, GoalHash: "g"})
		if err != nil {
			t.Fatal(err)
		}
		writeTreeFile(t, dir, LaneScopeFile, string(pin))
	}
	writeTreeFile(t, live, ".evolve/evals/phasespec-decomposed.md", "eval\n")
	writeRunLease(t, own, now)
	writeRunLease(t, live, now)
	writeRunLease(t, stale, now.Add(-time.Hour))

	got := liveSiblingsOf(root, own, now)

	want := liveSiblings{
		projectRoot: root,
		runs:        map[string]bool{filepath.Clean(live): true},
		items:       map[string]bool{"phasespec-roots-silent-policy-error": true, "phasespec-decomposed": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("liveSiblingsOf = %+v, want %+v", got, want)
	}
}

func TestEvalFilePath_IsTheWorkspaceEvalHome(t *testing.T) {
	if got, want := EvalFilePath("/ws", "router-silent-errors"), filepath.Join("/ws", ".evolve", "evals", "router-silent-errors.md"); got != want {
		t.Fatalf("EvalFilePath = %q, want %q", got, want)
	}
}

func TestLaneOwnership_MaterializedEvalsAloneMakeTheIdentityKnown(t *testing.T) {
	workspace := t.TempDir()
	writeTreeFile(t, workspace, evalsDir+"materialized-slug.md", "eval\n")

	got := laneOwnership(CycleState{CycleID: 7, WorkspacePath: workspace}, nil)

	if want := map[string]bool{"materialized-slug": true}; !reflect.DeepEqual(got.items, want) {
		t.Fatalf("items = %v, want %v", got.items, want)
	}
	if owner, foreign := got.foreignOwner(evalsDir + "another-slug.md"); !foreign {
		t.Fatalf("an eval this lane never materialized must not be claimable once its identity is known; owner=%q", owner)
	}
}

func TestCycleRun_RecoveryBaselineNeedsASnapshotOfACheckout(t *testing.T) {
	guard := treediff.New(defaultGitDirtyPaths)
	checkout := &cycleRun{checkout: &checkoutDecision{decided: true, checkout: true}}
	if got, want := checkout.recoveryBaselineFor(guard, []string{"operator-notes.md"}), map[string]bool{"operator-notes.md": true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("recoveryBaselineFor = %v, want %v", got, want)
	}
	if got := checkout.recoveryBaselineFor(guard, nil); got == nil || len(got) != 0 {
		t.Fatalf("a snapshot of a clean tree is an empty baseline, not a missing one; got %v", got)
	}
	if got := checkout.recoveryBaselineFor(nil, nil); got != nil {
		t.Errorf("no snapshot taken: baseline = %v, want nil (recovery must be skipped)", got)
	}
	noCheckout := &cycleRun{checkout: &checkoutDecision{decided: true, checkout: false}}
	if got := noCheckout.recoveryBaselineFor(guard, []string{"operator-notes.md"}); got != nil {
		t.Errorf("a root with no git checkout has no main tree to recover: baseline = %v, want nil", got)
	}
}
