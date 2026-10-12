package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
)

func TestMergeOrigin_EveryOutcomeOfTheMerge(t *testing.T) {
	boom := errors.New("exit 1")
	cases := []struct {
		name      string
		fail      map[string]bool
		wantCode  int
		wantErr   string
		wantCalls []string
	}{
		{"clean merge", nil, 0, "", []string{"merge --no-edit origin/main"}},
		{"git refused before starting a merge", map[string]bool{"merge --no-edit origin/main": true, "rev-parse -q --verify MERGE_HEAD": true}, 1,
			"evolve sync-main: refused — git would not merge origin/main:\nCONFLICT out", []string{"merge --no-edit origin/main", "rev-parse -q --verify MERGE_HEAD"}},
		{"conflict aborted cleanly", map[string]bool{"merge --no-edit origin/main": true}, 1,
			"conflicts with local history (aborted cleanly, tree unchanged)", []string{"merge --no-edit origin/main", "rev-parse -q --verify MERGE_HEAD", "merge --abort"}},
		{"conflict and the abort failed", map[string]bool{"merge --no-edit origin/main": true, "merge --abort": true}, 1,
			"evolve sync-main: merge conflicted AND abort failed: exit 1\nCONFLICT out", []string{"merge --no-edit origin/main", "rev-parse -q --verify MERGE_HEAD", "merge --abort"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			git := func(args ...string) (string, error) {
				call := strings.Join(args, " ")
				calls = append(calls, call)
				if tc.fail[call] {
					return "CONFLICT out", boom
				}
				return "", nil
			}
			var errb bytes.Buffer

			code := mergeOrigin(git, "main", &errb)

			if code != tc.wantCode || !strings.Contains(errb.String(), tc.wantErr) || (tc.wantErr == "" && errb.Len() != 0) {
				t.Fatalf("code %d stderr %q, want %d with %q", code, errb.String(), tc.wantCode, tc.wantErr)
			}
			if strings.Join(calls, "|") != strings.Join(tc.wantCalls, "|") {
				t.Errorf("calls = %q, want %q", calls, tc.wantCalls)
			}
			if strings.Contains(errb.String(), "aborted cleanly") && strings.Contains(errb.String(), "abort failed") {
				t.Errorf("both abort outcomes reported: %q", errb.String())
			}
		})
	}
}

func TestFetchSyncBranch_ARepositoryWithoutOriginFailsWithTheGitDetail(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	var errb bytes.Buffer

	err := fetchSyncBranch(repo.Dir, "main", &errb)

	if err == nil || !strings.HasPrefix(err.Error(), "git fetch origin main failed: rc=") || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(errb.String(), "retry") {
		t.Errorf("a missing remote is not ref-lock contention, yet it retried: %q", errb.String())
	}
}

func e3SyncPreparedPlan(t *testing.T) (repo, item string, plan inboxstamps.Plan) {
	t.Helper()
	repo, bare, item := smStampedPlane(t)
	smOriginWrites(t, bare, ".evolve/inbox/x.json", `{"id":"x","weight":0.9}`)
	smGit(t, repo, "fetch", "-q", "origin")
	stamps := []inboxstamps.Stamp{{Path: ".evolve/inbox/x.json", Set: map[string]json.RawMessage{"route": json.RawMessage(`"console-manual"`)}}}
	plan, err := inboxstamps.PlanAgainst(context.Background(), gitexec.Default(repo), stamps, "origin/main")
	if err != nil {
		t.Fatalf("PlanAgainst: %v", err)
	}
	if len(plan.Replay) != 1 {
		t.Fatalf("plan = %+v, want the stamp prepared for replay", plan)
	}
	return repo, item, plan
}

func TestStampLandingRestore_PutsThePreparedStampBack(t *testing.T) {
	repo, item, plan := e3SyncPreparedPlan(t)
	var errb bytes.Buffer

	stampLanding{root: repo, stdout: &bytes.Buffer{}, stderr: &errb}.restore(plan)

	if errb.Len() != 0 || !strings.Contains(smRead(t, item), `"route":"console-manual"`) {
		t.Fatalf("stderr %q item %s", errb.String(), smRead(t, item))
	}
}

func TestStampLandingRestore_AnUnwritableItemIsReportedNotSwallowed(t *testing.T) {
	repo, item, plan := e3SyncPreparedPlan(t)
	if err := os.Remove(item); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(item, 0o755); err != nil {
		t.Fatal(err)
	}
	var errb bytes.Buffer

	stampLanding{root: repo, stdout: &bytes.Buffer{}, stderr: &errb}.restore(plan)

	if !strings.HasPrefix(errb.String(), "evolve sync-main: restoring the loop's inbox stamps failed: apply the stamp on .evolve/inbox/x.json: ") {
		t.Fatalf("stderr = %q", errb.String())
	}
}
