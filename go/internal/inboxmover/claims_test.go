package inboxmover

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

var claimsNow = time.Date(2026, time.October, 8, 20, 32, 0, 0, time.UTC)

type holderRun struct {
	cycle      int
	goal       string
	phase      string
	checkpoint string
	worktree   bool
	lease      string
	disabled   bool
}

func claimsFixture(t *testing.T) (Options, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return Options{ProjectRoot: root, Now: func() time.Time { return claimsNow }}, root
}

func writeHolderRun(t *testing.T, root string, r holderRun) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", r.cycle))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(root, ".evolve", "worktrees", fmt.Sprintf("cycle-%d", r.cycle))
	if r.worktree {
		if err := os.MkdirAll(worktree, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	state := fmt.Sprintf(`{"cycle_id":%d,"phase":%q,"goal_hash":%q`, r.cycle, r.phase, r.goal)
	if r.checkpoint != "" {
		state += fmt.Sprintf(`,"checkpoint":{"enabled":%t,"reason":%q,"resumeFromPhase":"tdd","worktreePath":%q}`, !r.disabled, r.checkpoint, worktree)
	}
	writeFile(t, filepath.Join(runDir, "cycle-state.json"), state+"}")
	switch r.lease {
	case "live":
		writeLease(t, runDir, os.Getpid(), claimsNow.Add(-time.Minute))
	case "dead":
		writeLease(t, runDir, os.Getpid(), claimsNow.Add(-time.Hour))
	}
}

func writeLease(t *testing.T, runDir string, pid int, heartbeat time.Time) {
	t.Helper()
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: pid}, heartbeat); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func claimItem(t *testing.T, root string, cycle int, id string) string {
	t.Helper()
	path := filepath.Join(root, ".evolve", "inbox", "processing", fmt.Sprintf("cycle-%d", cycle), "2026-09-30T11-07-44Z-"+id+".json")
	writeFile(t, path, `{"id":"`+id+`","weight":0.5}`)
	return path
}

func rootItem(t *testing.T, root, id, body string) string {
	t.Helper()
	path := filepath.Join(root, ".evolve", "inbox", "2026-09-30T11-07-44Z-"+id+".json")
	writeFile(t, path, body)
	return path
}

func TestClassifyHolder_VerdictFollowsTheEvidenceOrder(t *testing.T) {
	cases := []struct {
		name        string
		runs        []holderRun
		closedOut   bool
		want        HolderVerdict
		wantReason  string
		wantSuperBy int
	}{
		{"a live lease wins over a closeout", []holderRun{{cycle: 1837, goal: "g81", phase: "build", lease: "live"}}, true, HolderLive, "lease live", 0},
		{"a dossier closeout is stale", []holderRun{{cycle: 1837, goal: "g81", phase: "retro", checkpoint: "quota-likely", worktree: true, lease: "dead"}}, true, HolderStale, "closed out", 0},
		{"phase end is stale", []holderRun{{cycle: 1837, goal: "g81", phase: "end", checkpoint: "quota-likely", worktree: true}}, false, HolderStale, "phase end", 0},
		{"a quota pause of the newest goal waits for its resume", []holderRun{{cycle: 1837, goal: "g80", phase: "tdd", checkpoint: "quota-likely", worktree: true, lease: "dead"}, {cycle: 1838, goal: "g80", phase: "build"}}, false, HolderResumePending, "resume", 0},
		{"a newer goal supersedes the pause", []holderRun{{cycle: 1837, goal: "g80", phase: "tdd", checkpoint: "quota-likely", worktree: true, lease: "dead"}, {cycle: 1838, goal: "g81", phase: "triage"}}, false, HolderStale, "superseded", 1838},
		{"an older cycle of another goal does not supersede", []holderRun{{cycle: 1836, goal: "g79", phase: "end"}, {cycle: 1837, goal: "g80", phase: "tdd", checkpoint: "operator-requested", worktree: true}}, false, HolderResumePending, "resume", 0},
		{"a checkpoint loop --resume refuses is stale", []holderRun{{cycle: 1837, goal: "g80", phase: "build", checkpoint: "phase-complete", worktree: true, lease: "dead"}}, false, HolderStale, "no resumable checkpoint", 0},
		{"a disabled checkpoint is stale", []holderRun{{cycle: 1837, goal: "g80", phase: "tdd", checkpoint: "quota-likely", worktree: true, disabled: true}}, false, HolderStale, "no resumable checkpoint", 0},
		{"a pause whose worktree is gone is stale", []holderRun{{cycle: 1837, goal: "g80", phase: "tdd", checkpoint: "quota-likely", worktree: false}}, false, HolderStale, "worktree", 0},
		{"no run dir at all is stale", nil, false, HolderStale, "no live lease", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, root := claimsFixture(t)
			for _, r := range tc.runs {
				writeHolderRun(t, root, r)
			}
			if tc.closedOut {
				writeFile(t, filepath.Join(root, "knowledge-base", "cycles", "cycle-1837.json"), "{}")
			}
			got := ClassifyHolder(opts, 1837)
			if got.Cycle != 1837 || got.Verdict != tc.want || !strings.Contains(got.Reason, tc.wantReason) || got.Evidence.SupersededBy != tc.wantSuperBy {
				t.Errorf("ClassifyHolder = %+v, want verdict %q, reason containing %q, superseded_by %d", got, tc.want, tc.wantReason, tc.wantSuperBy)
			}
			if got.Keeps() != (tc.want != HolderStale) {
				t.Errorf("Keeps() = %v for verdict %q", got.Keeps(), got.Verdict)
			}
		})
	}
}

func TestClassifyHolder_EvidenceNamesLeaseCloseoutAndCheckpoint(t *testing.T) {
	opts, root := claimsFixture(t)
	writeHolderRun(t, root, holderRun{cycle: 1828, goal: "g77", phase: "retro", checkpoint: "phase-complete", worktree: true, lease: "dead"})
	writeFile(t, filepath.Join(root, "knowledge-base", "cycles", "cycle-1828.json"), "{}")
	got := ClassifyHolder(opts, 1828)
	want := HolderEvidence{
		Lease:    fmt.Sprintf("stale: pid %d, heartbeat %s", os.Getpid(), claimsNow.Add(-time.Hour).UTC().Format(time.RFC3339Nano)),
		Phase:    "retro",
		Closeout: filepath.Join(root, "knowledge-base", "cycles", "cycle-1828.json"),
	}
	if got.Evidence != want {
		t.Errorf("evidence = %+v, want %+v", got.Evidence, want)
	}
}

func TestSurveyClaims_ListsEveryHeldItemWithItsHolderAndTheEmptyDirs(t *testing.T) {
	opts, root := claimsFixture(t)
	writeHolderRun(t, root, holderRun{cycle: 1837, goal: "g81", phase: "build", lease: "live"})
	claimItem(t, root, 1837, "live-item")
	claimItem(t, root, 1828, "stale-item")
	claimItem(t, root, 1836, "dup-item")
	rootItem(t, root, "dup-item", `{"id":"dup-item","weight":0.5}`)
	for _, c := range []int{1700, 1837} {
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox", "processing", fmt.Sprintf("cycle-%d", c)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var survey ClaimSurvey
	survey, err := SurveyClaims(opts)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]HeldClaim{}
	for _, c := range survey.Claims {
		got[c.ID] = c
	}
	if len(got) != 3 || got["live-item"].Holder.Verdict != HolderLive || got["stale-item"].Holder.Cycle != 1828 || got["stale-item"].Holder.Verdict != HolderStale {
		t.Errorf("claims = %+v, want live-item live and stale-item stale under 1828", survey.Claims)
	}
	if !got["dup-item"].Duplicate || got["live-item"].Duplicate {
		t.Errorf("only dup-item has a root copy: %+v", survey.Claims)
	}
	var empty []EmptyClaimDir = survey.EmptyDirs
	if len(empty) != 1 || survey.EmptyDirs[0].Holder.Cycle != 1700 || survey.EmptyDirs[0].Holder.Verdict != HolderStale || survey.EmptyDirs[0].Path != filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1700") {
		t.Errorf("empty dirs = %+v, want only cycle-1700 (stale); cycle-1837 holds an item", survey.EmptyDirs)
	}
}

func TestReleaseClaim_MovesAStaleClaimToTheRootOnceAndRecordsIt(t *testing.T) {
	opts, root := claimsFixture(t)
	rec := &recordingAppender{}
	opts.Ledger = rec
	src := claimItem(t, root, 1828, "stale-item")
	res, err := ReleaseClaim(opts, "stale-item", "holder sealed FAIL in wave 77")
	if err != nil || res.Outcome != ClaimReleased || res.Cycle != 1828 {
		t.Fatalf("ReleaseClaim = %+v, %v; want released from cycle 1828", res, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("the claim copy must be gone: %v", err)
	}
	if loc, err := Locate(opts.ProjectRoot+"/.evolve/inbox", "stale-item"); err != nil || loc.Cycle != 0 {
		t.Errorf("the item must be pending at the root: %+v %v", loc, err)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "release" || rec.records[0].Cycle != 1828 || !strings.Contains(rec.records[0].Message, "holder sealed FAIL in wave 77") {
		t.Fatalf("ledger = %+v, want one release line with the reason", rec.records)
	}
	again, err := ReleaseClaim(opts, "stale-item", "again")
	if err != nil || again.Outcome != ClaimNotHeld || len(rec.records) != 1 {
		t.Errorf("a second release is a no-op success with no ledger line: %+v %v %d", again, err, len(rec.records))
	}
}

func TestReleaseClaim_RefusesAHolderThatCanStillUseIt(t *testing.T) {
	for _, r := range []holderRun{
		{cycle: 1837, goal: "g81", phase: "build", lease: "live"},
		{cycle: 1837, goal: "g81", phase: "tdd", checkpoint: "quota-likely", worktree: true, lease: "dead"},
	} {
		opts, root := claimsFixture(t)
		writeHolderRun(t, root, r)
		src := claimItem(t, root, 1837, "held-item")
		_, err := ReleaseClaim(opts, "held-item", "operator")
		if !errors.Is(err, ErrClaimHeld) {
			t.Errorf("lease %q checkpoint %q: err = %v, want ErrClaimHeld", r.lease, r.checkpoint, err)
		}
		if _, statErr := os.Stat(src); statErr != nil {
			t.Errorf("a refused release must not move the claim: %v", statErr)
		}
	}
}

func TestReleaseClaim_ResolvesAnEqualRootDuplicateAndRefusesADifferentOne(t *testing.T) {
	opts, root := claimsFixture(t)
	rec := &recordingAppender{}
	opts.Ledger = rec
	src := claimItem(t, root, 1836, "dup-item")
	kept := rootItem(t, root, "dup-item", `{"id":"dup-item","weight":0.5}`)
	res, err := ReleaseClaim(opts, "dup-item", "restored by hand")
	if err != nil || res.Outcome != ClaimDuplicateRemoved {
		t.Fatalf("ReleaseClaim = %+v, %v; want the duplicate removed", res, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("the claim copy must be removed: %v", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("the root copy must stay: %v", err)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "release" || !strings.Contains(rec.records[0].Message, "duplicate") {
		t.Errorf("ledger = %+v, want one release line that names the duplicate", rec.records)
	}
	other := claimItem(t, root, 1836, "diff-item")
	rootItem(t, root, "diff-item", `{"id":"diff-item","weight":0.9}`)
	if _, err := ReleaseClaim(opts, "diff-item", "restored by hand"); !errors.Is(err, ErrClaimConflict) {
		t.Errorf("a root copy with other bytes must refuse: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("a conflict changes nothing: %v", err)
	}
}

func TestReleaseClaim_RefusesAnUnknownIDAndAnEmptyReason(t *testing.T) {
	opts, root := claimsFixture(t)
	claimItem(t, root, 1828, "stale-item")
	if _, err := ReleaseClaim(opts, "never-filed", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v, want ErrNotFound", err)
	}
	if _, err := ReleaseClaim(opts, "stale-item", "  "); !errors.Is(err, ErrBadArgs) {
		t.Errorf("empty reason: %v, want ErrBadArgs", err)
	}
}

func TestReleaseStaleClaims_ReleasesOnlyStaleHolders(t *testing.T) {
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	writeHolderRun(t, root, holderRun{cycle: 1837, goal: "g81", phase: "build", lease: "live"})
	writeHolderRun(t, root, holderRun{cycle: 1835, goal: "g81", phase: "tdd", checkpoint: "quota-likely", worktree: true})
	live := claimItem(t, root, 1837, "live-item")
	paused := claimItem(t, root, 1835, "paused-item")
	claimItem(t, root, 1828, "stale-item")
	var released []ClaimRelease
	released, err := ReleaseStaleClaims(opts, "wave-plan")
	if err != nil {
		t.Fatal(err)
	}
	if outcomes := []ClaimOutcome{ClaimReleased, ClaimDuplicateRemoved, ClaimNotHeld}; outcomes[0] != "released" || outcomes[1] != "duplicate-removed" || outcomes[2] != "not-claimed" {
		t.Errorf("the outcomes are spelled as the --json output: %v", outcomes)
	}
	if len(released) != 1 || released[0].ID != "stale-item" || released[0].Cycle != 1828 || released[0].Outcome != ClaimReleased || released[0].Holder.Verdict != HolderStale {
		t.Errorf("released = %+v, want only stale-item", released)
	}
	for _, p := range []string{live, paused} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s must keep its claim: %v", p, err)
		}
	}
}

func TestResolveDispatchState_AClaimWinsOverARootDuplicate(t *testing.T) {
	opts, root := claimsFixture(t)
	claimItem(t, root, 1836, "dup-item")
	rootItem(t, root, "dup-item", `{"id":"dup-item","weight":0.5}`)
	if got := ResolveDispatchState(opts, "dup-item"); got.State != StateProcessing || got.Detail != "cycle-1836" {
		t.Errorf("ResolveDispatchState = %+v, want processing in cycle-1836", got)
	}
	if d := ResolveDispatchability(opts, "dup-item"); d.Dispatchable {
		t.Errorf("a held duplicate is never dispatchable: %+v", d)
	}
}

func TestPlaceOnLaneMenu_AHeldRootCopyWaitsForItsClaim(t *testing.T) {
	opts, root := claimsFixture(t)
	claimItem(t, root, 1836, "dup-item")
	rootItem(t, root, "dup-item", `{"id":"dup-item","weight":0.5}`)
	rootItem(t, root, "free-item", `{"id":"free-item","weight":0.5}`)
	place, reason := PlaceOnLaneMenu(opts, inboxbatch.Item{ID: "dup-item"}, nil)
	if place != MenuWaiting || reason != "held by the claim of cycle-1836" {
		t.Errorf("PlaceOnLaneMenu(dup-item) = %v, %q; want waiting, held by the claim of cycle-1836", place, reason)
	}
	if place, reason := PlaceOnLaneMenu(opts, inboxbatch.Item{ID: "free-item"}, nil); place != MenuReady || reason != "" {
		t.Errorf("PlaceOnLaneMenu(free-item) = %v, %q; want ready", place, reason)
	}
}

func TestClassifyHolder_ReadsTheFallbackEvidence(t *testing.T) {
	opts, root := claimsFixture(t)
	runs := filepath.Join(root, ".evolve", "runs")
	writeFile(t, filepath.Join(runs, "cycle-1700.polluted-20260901", "cycle-state.json"), `{"cycle_id":1700,"goal_hash":"other"}`)
	writeFile(t, filepath.Join(runs, "notes.txt"), "x")
	writeFile(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":1836,"phase":"tdd","goal_hash":"g","checkpoint":{"enabled":true,"reason":"phase-complete","resumeFromPhase":"build"}}`)
	writeFile(t, filepath.Join(runs, "cycle-1836", ".lease"), "not json")

	got := ClassifyHolder(Options{InboxDir: filepath.Join(root, ".evolve", "inbox"), Now: opts.Now}, 1836)

	if got.Verdict != HolderResumePending || got.Evidence.Phase != "tdd" || got.Evidence.Checkpoint != "phase-complete" || !strings.HasPrefix(got.Evidence.Lease, "unreadable: ") || got.Evidence.SupersededBy != 0 {
		t.Errorf("ClassifyHolder = %+v; want resume-pending from the primary state (loop --resume takes any enabled primary checkpoint), an unreadable lease, and no supersede by a polluted dir", got)
	}
	writeFile(t, filepath.Join(runs, "cycle-1836", "cycle-state.json"), "{broken")
	writeFile(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":1999}`)
	if got := ClassifyHolder(opts, 1836); got.Verdict != HolderStale || got.Evidence.Phase != "" {
		t.Errorf("a broken per-run state and a primary state of another cycle give no evidence: %+v", got)
	}
}

func TestSurveyAndReleaseStaleClaims_ReportAFaultAndKeepTheClaim(t *testing.T) {
	_, root := claimsFixture(t)
	notADir := filepath.Join(root, "inbox-file")
	writeFile(t, notADir, "x")
	bad := Options{InboxDir: notADir}
	if _, err := SurveyClaims(bad); err == nil {
		t.Error("SurveyClaims of a root that is not a dir must fail")
	}
	if _, err := ReleaseStaleClaims(bad, "x"); err == nil {
		t.Error("ReleaseStaleClaims of a root that is not a dir must fail")
	}
	opts, root := claimsFixture(t)
	opts.Ledger = &recordingAppender{}
	claim := claimItem(t, root, 1828, "stale-item")
	rootItem(t, root, "stale-item", `{"id":"stale-item","weight":0.9}`)
	released, err := ReleaseStaleClaims(opts, "boundary")
	if !errors.Is(err, ErrClaimConflict) || !strings.Contains(err.Error(), "release stale-item from cycle 1828") || len(released) != 0 {
		t.Errorf("ReleaseStaleClaims = %+v, %v; want a conflict that names the claim", released, err)
	}
	if _, statErr := os.Stat(claim); statErr != nil {
		t.Errorf("the claim must stay: %v", statErr)
	}
}

func TestClassifyHolder_TheRunningLoopsGoalSupersedesAPauseOfAnotherGoal(t *testing.T) {
	opts, root := claimsFixture(t)
	writeHolderRun(t, root, holderRun{cycle: 1836, goal: "goal-wave-80", phase: "tdd", checkpoint: "quota-likely", worktree: true, lease: "dead"})

	if got := ClassifyHolder(opts, 1836); got.Verdict != HolderResumePending {
		t.Fatalf("with no running goal and no newer cycle the pause waits: %+v", got)
	}
	opts.CurrentGoal = "goal-wave-80"
	if got := ClassifyHolder(opts, 1836); got.Verdict != HolderResumePending || got.Evidence.SupersededByLoop {
		t.Errorf("a running loop of the same goal can still resume it: %+v", got)
	}
	opts.CurrentGoal = "goal-wave-81"
	got := ClassifyHolder(opts, 1836)
	if got.Verdict != HolderStale || !got.Evidence.SupersededByLoop || got.Reason != "paused (quota-likely), but the running loop has a newer goal" {
		t.Errorf("ClassifyHolder = %+v; want stale, superseded by the running loop", got)
	}
}

func TestClassifyHolder_FollowsLoopResumeOnAMovedHeadAndAMissingPhase(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "seed")
	root := repo.Dir
	opts := Options{ProjectRoot: root, Now: func() time.Time { return claimsNow }}
	head := repo.Git("rev-parse", "HEAD")
	worktree := filepath.Join(root, "wt")
	writeFile(t, filepath.Join(worktree, ".keep"), "")
	cases := []struct {
		name, checkpoint string
		want             HolderVerdict
		wantReason       string
	}{
		{"the head did not move", `{"enabled":true,"reason":"quota-likely","resumeFromPhase":"tdd","gitHead":"` + head + `","worktreePath":"` + worktree + `"}`, HolderResumePending, "paused (quota-likely)"},
		{"the boundary moved the head", `{"enabled":true,"reason":"quota-likely","resumeFromPhase":"tdd","gitHead":"0000000000000000000000000000000000000000","worktreePath":"` + worktree + `"}`, HolderStale, "git HEAD moved"},
		{"no resumeFromPhase", `{"enabled":true,"reason":"quota-likely","gitHead":"` + head + `"}`, HolderStale, "resumeFromPhase missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFile(t, filepath.Join(root, ".evolve", "runs", "cycle-1836", "cycle-state.json"), `{"cycle_id":1836,"phase":"tdd","goal_hash":"g","checkpoint":`+tc.checkpoint+`}`)
			got := ClassifyHolder(opts, 1836)
			if got.Verdict != tc.want || !strings.Contains(got.Reason, tc.wantReason) {
				t.Errorf("ClassifyHolder = %+v; want %s with a reason containing %q", got, tc.want, tc.wantReason)
			}
		})
	}
}

func TestAbsorbRootCopies_ParksADifferentRootCopyAndKeepsTheClaim(t *testing.T) {
	opts, root := claimsFixture(t)
	rec := &recordingAppender{}
	opts.Ledger = rec
	claim := claimItem(t, root, 1836, "held")
	rootItem(t, root, "held", `{"id":"held","weight":0.9}`)

	var absorbed []Absorbed
	absorbed, err := AbsorbRootCopies(opts)

	if err != nil || len(absorbed) != 1 || absorbed[0].Outcome != AbsorbParked || absorbed[0].ID != "held" {
		t.Fatalf("AbsorbRootCopies = %+v, %v; want held parked", absorbed, err)
	}
	if body, _ := os.ReadFile(claim); string(body) != `{"id":"held","weight":0.5}` {
		t.Errorf("the claim copy changed: %s", body)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "absorb-conflict" || AbsorbDropped != "dropped" {
		t.Errorf("ledger = %+v; want one absorb-conflict line", rec.records)
	}
}
