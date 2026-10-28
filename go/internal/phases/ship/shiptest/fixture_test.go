//go:build integration

package shiptest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type recordingTB struct {
	testing.TB
	mu      sync.Mutex
	errs    []string
	skipped []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

func (r *recordingTB) Skip(args ...any) {
	r.mu.Lock()
	r.skipped = append(r.skipped, fmt.Sprint(args...))
	r.mu.Unlock()
	runtime.Goexit()
}

func untilExit(body func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		body()
	}()
	<-done
}

func TestCheck_AFailureStopsTheTestWithItsContext(t *testing.T) {
	t.Parallel()
	rec := &recordingTB{TB: t}
	untilExit(func() { check(rec, nil, "a step that worked") })
	untilExit(func() { check(rec, errors.New("boom"), "write %s", "x.txt") })
	if len(rec.errs) != 1 || rec.errs[0] != "write x.txt: boom" {
		t.Fatalf("failures = %q, want exactly one naming the step and the cause", rec.errs)
	}
}

func TestSkipIf_SkipsOnlyWhenTheConditionHolds(t *testing.T) {
	t.Parallel()
	rec := &recordingTB{TB: t}
	untilExit(func() { skipIf(rec, false, "never") })
	untilExit(func() { skipIf(rec, true, "a root run") })
	if len(rec.skipped) != 1 || rec.skipped[0] != "a root run" {
		t.Fatalf("skips = %q, want only the one whose condition holds", rec.skipped)
	}
	SkipWhenRoot(t)
}

func TestJoinFailures_NoFailureIsNoError(t *testing.T) {
	t.Parallel()
	if err := joinFailures(nil); err != nil {
		t.Fatalf("no failures gave %v, want nil", err)
	}
	if err := joinFailures([]string{"a", "b"}); err == nil || err.Error() != "a; b" {
		t.Fatalf("two failures gave %v, want both in one error", err)
	}
}

func TestWantShipErr_RefusesEveryMismatch(t *testing.T) {
	t.Parallel()
	err := core.NewShipError(core.CodeGitPushRejected, core.ShipClassTransient, core.StageAtomicShip, "push race")
	for name, call := range map[string]func(testing.TB){
		"not a ship error": func(tb testing.TB) { MustShipErr(tb, errors.New("plain")) },
		"another code":     func(tb testing.TB) { WantShipErr(tb, err, core.CodeStateIO, core.ShipClassTransient, "") },
		"another class":    func(tb testing.TB) { WantShipErr(tb, err, core.CodeGitPushRejected, core.ShipClassIntegrity, "") },
		"another message":  func(tb testing.TB) { WantShipErr(tb, err, core.CodeGitPushRejected, core.ShipClassTransient, "absent") },
	} {
		rec := &recordingTB{TB: t}
		untilExit(func() { call(rec) })
		if len(rec.errs) != 1 {
			t.Errorf("%s: failures = %q, want exactly one", name, rec.errs)
		}
	}
	if se := WantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "race"); se.Message != "push race" {
		t.Errorf("a matching error returned %+v", se)
	}
}

func TestRemoteHeadSHA_AnOriginWithoutMainHasNoHead(t *testing.T) {
	t.Parallel()
	repo := MakeRepo(t)
	AddRemote(t, repo)
	if got := RemoteHeadSHA(t, repo); got != "" {
		t.Fatalf("an empty origin reported head %q", got)
	}
	RunGit(t, repo, "push", "-q", "origin", "main")
	if got, want := RemoteHeadSHA(t, repo), HeadSHA(t, repo); got != want {
		t.Fatalf("origin head %q, want the pushed main %q", got, want)
	}
}

func TestIntent_AMissingIntentIsAbsentAndStopsTheTest(t *testing.T) {
	t.Parallel()
	l := Lane{Repo: t.TempDir()}
	if in, found := ReadIntentAt(t, l.IntentFile()); found || in != nil {
		t.Fatalf("a missing intent read as %v found=%v", in, found)
	}
	rec := &recordingTB{TB: t}
	untilExit(func() { l.Intent(rec) })
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "no landing intent") {
		t.Fatalf("failures = %q, want the missing intent named", rec.errs)
	}
}

func TestPredicateVerdictFixture_CountsEachOutcome(t *testing.T) {
	t.Parallel()
	v := PredicateVerdictFixture(7, 1, 1, 1)
	if v.Verdict != "FAIL" || v.ShipEligible || len(v.Results) != 3 {
		t.Fatalf("verdict %q eligible=%v results=%d, want FAIL, not eligible, three results", v.Verdict, v.ShipEligible, len(v.Results))
	}
	if strings.Join(v.RedIDs, ",") != "predicate-1" || strings.Join(v.SkipIDs, ",") != "predicate-2" {
		t.Fatalf("red %v skip %v, want predicate-1 red and predicate-2 skipped", v.RedIDs, v.SkipIDs)
	}
	if v.Results[0].ResultStr != "green" || v.Results[1].ExitCode != 1 || v.Results[2].ResultStr != "skip" {
		t.Fatalf("results %+v", v.Results)
	}
}

func readLedgerEntry(t *testing.T, repo string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestSeedAudit_TheCycleStateAndTheOverridesShapeTheEntry(t *testing.T) {
	t.Parallel()
	repo := MakeRepo(t)
	MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"cycle_id":4,"run_id":"run-4"}`)
	bound := strings.TrimSpace(RunGitOut(t, repo, "rev-parse", "HEAD^{tree}"))
	SeedAudit(t, repo, "PASS", map[string]string{"head": "h", "tree": "t", "exit_code": "3", "bound_tree": bound})
	entry := readLedgerEntry(t, repo)
	for key, want := range map[string]any{"cycle": 4.0, "run_id": "run-4", "git_head": "h", "tree_state_sha": "t", "exit_code": 3.0, "worktree_tree_sha": bound} {
		if entry[key] != want {
			t.Errorf("ledger %s = %v, want %v", key, entry[key], want)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".evolve", "runs", "cycle-4", "audit-report.md")); err != nil {
		t.Errorf("the report of cycle 4 is missing: %v", err)
	}
}

func TestSeedAudit_AnUnreadableCycleStateFallsBackToTheDefaultsUnlessOverridden(t *testing.T) {
	t.Parallel()
	repo := MakeRepo(t)
	MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), "{")
	SeedAudit(t, repo, "FAIL", map[string]string{"cycle": "9"})
	entry := readLedgerEntry(t, repo)
	if entry["cycle"] != 9.0 || entry["run_id"] != "test-run" || entry["exit_code"] != 0.0 {
		t.Fatalf("ledger %v, want cycle 9 from the override, the default run and exit 0", entry)
	}
	rec := &recordingTB{TB: t}
	untilExit(func() { SeedAudit(rec, repo, "PASS", map[string]string{"cycle": "nine"}) })
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "override cycle") {
		t.Fatalf("failures = %q, want the bad override named", rec.errs)
	}
}

func TestTempRepoDir_RemovesEvenALockedTreeAtCleanup(t *testing.T) {
	t.Parallel()
	var dir string
	t.Run("use", func(t *testing.T) {
		dir = TempRepoDir(t)
		locked := filepath.Join(dir, "locked")
		MustWrite(t, filepath.Join(locked, "f.txt"), "x\n")
		if err := os.Chmod(locked, 0o555); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the temp repo dir %s survived its cleanup: %v", dir, err)
	}
}

func TestLaneReadyToLand_CommitsTheLaneItemAndPendsTheAuditedTree(t *testing.T) {
	t.Parallel()
	l := LaneReadyToLand(t)
	if got := strings.TrimSpace(RunGitOut(t, l.Repo, "ls-files", LandingItem)); got != LandingItem {
		t.Fatalf("the lane item %s is not committed on main: %q", LandingItem, got)
	}
	triage, err := os.ReadFile(filepath.Join(l.WS, "triage-decision.json"))
	if err != nil || !strings.Contains(string(triage), `"id":"`+LandingItemID+`"`) {
		t.Fatalf("the triage decision %q (err %v) must select %s", triage, err, LandingItemID)
	}
	if HeadSHA(t, l.WT) != l.Base || strings.TrimSpace(RunGitOut(t, l.WT, "write-tree")) != l.Audited || l.View == nil {
		t.Fatalf("the lane %+v must hold the audited tree pending on its base with a sealed view", l)
	}
}
