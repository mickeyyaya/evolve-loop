package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestLaneItem_AnUnreadableInboxRecordIsUnresolvedWithItsPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cases := map[string]string{
		"missing file":  filepath.Join(root, "absent.json"),
		"corrupt JSON":  writeInboxRecord(t, root, "corrupt.json", "{not json"),
		"a directory":   filepath.Join(root, ".evolve"),
		"unicode named": filepath.Join(root, "ünïcødé-record.json"),
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			o := &Orchestrator{scopePathFor: func(_, id string) string {
				if id != "item-7" {
					t.Errorf("resolver got id %q, want item-7", id)
				}
				return path
			}}
			got := o.laneItem(root, "item-7")
			if got.ID != "item-7" || got.Kind != "" || got.Acceptance != nil {
				t.Fatalf("an unreadable record must carry only its id: %+v", got)
			}
			if !strings.HasPrefix(got.Unresolved, "inbox record unreadable at "+path+": ") {
				t.Fatalf("Unresolved = %q, want it to name the path %s and the read error", got.Unresolved, path)
			}
		})
	}
}

func TestRetireSupersededAuditArtifacts_AStaleSourceThatCannotBeRemovedIsKeptAndWarned(t *testing.T) {
	t.Parallel()

	ws := t.TempDir()
	src := filepath.Join(ws, acsverdict.Filename)
	writeFile(t, filepath.Join(src, "inner"), "x")
	dst := filepath.Join(ws, phasecontract.RoundArchiveFilename(acsverdict.Filename, 2))
	writeFile(t, dst, "first retirement")

	var sink injectedStderr
	retireSupersededAuditArtifacts(ws, 2, &sink)
	log := sink.String()

	if !strings.Contains(log, "exists but stale "+acsverdict.Filename+" could not be removed") {
		t.Fatalf("stderr %q must warn that the stale verdict survived", log)
	}
	if body, err := os.ReadFile(dst); err != nil || string(body) != "first retirement" {
		t.Fatalf("the first retirement's archive was clobbered: %q, %v", body, err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("the unremovable source should still exist: %v", err)
	}
}

func TestRetireSupersededAuditArtifacts_ARenameAndRemoveRefusalLeavesTheSourceWithAWarning(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	ws := t.TempDir()
	src := filepath.Join(ws, acsverdict.Filename)
	writeFile(t, src, `{"verdict":"FAIL"}`)
	if err := os.Chmod(ws, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o700) })

	var sink injectedStderr
	retireSupersededAuditArtifacts(ws, 3, &sink)
	log := sink.String()

	if !strings.Contains(log, "could not retire superseded "+acsverdict.Filename+" (rename: ") || !strings.Contains(log, "; remove: ") {
		t.Fatalf("stderr %q must name both the rename and the remove failure", log)
	}
	if body, err := os.ReadFile(src); err != nil || string(body) != `{"verdict":"FAIL"}` {
		t.Fatalf("the source must stay untouched when neither rename nor remove works: %q, %v", body, err)
	}
}

func pathMaxForTest(t *testing.T) int {
	t.Helper()
	switch runtime.GOOS {
	case "darwin":
		return 1024
	case "linux":
		return 4096
	}
	t.Skipf("no known PATH_MAX for %s", runtime.GOOS)
	return 0
}

func workspaceWithPathLength(t *testing.T, total int) string {
	t.Helper()
	ws, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for len(ws)+1 < total {
		seg := min(200, total-len(ws)-1)
		ws = filepath.Join(ws, strings.Repeat("w", seg))
	}
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir long workspace: %v", err)
	}
	return ws
}

func TestRetireSupersededAuditArtifacts_AnArchivePathPastPathMaxRemovesTheSourceUnarchived(t *testing.T) {
	t.Parallel()

	limit := pathMaxForTest(t)
	const round = 123456789012
	name := acsverdict.Filename
	ws := workspaceWithPathLength(t, limit-8-len(name))
	src := filepath.Join(ws, name)
	if len(src) >= limit || len(filepath.Join(ws, phasecontract.RoundArchiveFilename(name, round))) < limit {
		t.Fatalf("fixture lengths: src %d, limit %d", len(src), limit)
	}
	writeFile(t, src, "stale")

	var sink injectedStderr
	retireSupersededAuditArtifacts(ws, round, &sink)
	log := sink.String()

	if !strings.Contains(log, "superseded "+name+" removed, not archived (rename: ") || !strings.Contains(log, "round-123456789012 forensic evidence was lost") {
		t.Fatalf("stderr %q must warn that the round's evidence was removed unarchived", log)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("the stale source must be gone so the next round cannot read it: %v", err)
	}
}

type erroringAfterIterLedger struct {
	fakeLedger
	rows []LedgerEntry
	err  error
}

type erroringIterator struct {
	rows []LedgerEntry
	err  error
}

func (it *erroringIterator) Next() (LedgerEntry, bool, error) {
	if len(it.rows) == 0 {
		return LedgerEntry{}, false, it.err
	}
	next := it.rows[0]
	it.rows = it.rows[1:]
	return next, true, nil
}

func (it *erroringIterator) Close() error { return nil }

func (l *erroringAfterIterLedger) Iter(context.Context) (LedgerIterator, error) {
	return &erroringIterator{rows: l.rows, err: l.err}, nil
}

func TestLatestAuditEntry_AMidStreamReadErrorIsReturnedNotABoundRow(t *testing.T) {
	t.Parallel()
	torn := errors.New("torn ledger line 42")
	ledger := &erroringAfterIterLedger{
		rows: []LedgerEntry{{Kind: "agent_subprocess", Role: "auditor", RunID: "run-1", WorktreeTreeSHA: "tree-a"}},
		err:  torn,
	}
	o := &Orchestrator{ledger: ledger}

	entry, err := o.latestAuditEntry(context.Background(), "run-1")

	if !errors.Is(err, torn) {
		t.Fatalf("err = %v, want the iterator's read error", err)
	}
	if entry.WorktreeTreeSHA != "" || entry.RunID != "" {
		t.Fatalf("a failed read must not bind a partial row: %+v", entry)
	}
}

func TestCollectBatchFailureDigests_SkipsWhatIsNotADigestOfTheBatch(t *testing.T) {
	t.Parallel()
	if got := CollectBatchFailureDigests(filepath.Join(t.TempDir(), "no-evolve"), 1); got != nil {
		t.Fatalf("an absent runs dir = %+v, want nil", got)
	}
	evolve := t.TempDir()
	runs := filepath.Join(evolve, "runs")
	writeFile(t, filepath.Join(runs, "cycle-5", "failure-digest.json"), `{"cycle":5,"fingerprint":"fp-5"}`)
	writeFile(t, filepath.Join(runs, "cycle-6", "other.json"), `{}`)
	writeFile(t, filepath.Join(runs, "cycle-7", "failure-digest.json"), `{"cycle":`)
	writeFile(t, filepath.Join(runs, "cycle-4", "failure-digest.json"), `{"cycle":4,"fingerprint":"fp-4"}`)
	writeFile(t, filepath.Join(runs, "cycle-9"), "a file, not a cycle dir")
	writeFile(t, filepath.Join(runs, "lane-8", "failure-digest.json"), `{"cycle":8,"fingerprint":"fp-8"}`)

	got := CollectBatchFailureDigests(evolve, 5)

	if len(got) != 1 || got[0].Fingerprint != "fp-5" || got[0].Cycle != 5 {
		t.Fatalf("digests = %+v, want only cycle 5's", got)
	}
}

func TestBatchCycleNumber_OnlyCycleDirsAtOrAfterTheBatchStartCount(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, d := range []string{"cycle-9", "cycle-10", "cycle-x", "Cycle-12", "cycle-", "cycle--3", "run-11"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dir, "cycle-20"), "file")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"cycle-10": 10}
	for _, e := range entries {
		n, ok := batchCycleNumber(e, 10)
		w, wantOK := want[e.Name()]
		if ok != wantOK || n != w {
			t.Errorf("batchCycleNumber(%s, 10) = (%d, %v), want (%d, %v)", e.Name(), n, ok, w, wantOK)
		}
	}
}

func TestRouteAfterExplanationRefresh_ACleanRefreshRoutesByWhetherBuildIsRequired(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	plain := errors.New("disk full")
	cases := []struct {
		name          string
		requiresBuild bool
		refreshErr    error
		want          Phase
		wantErr       error
	}{
		{"clean refresh without material scope change continues", false, nil, "", nil},
		{"clean refresh after a scope change routes to build", true, nil, PhaseBuild, nil},
		{"a non-content refresh error is returned as-is", false, plain, "", plain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := o.routeAfterExplanationRefresh(&CycleState{CycleID: 3}, PhaseAudit, tc.requiresBuild, tc.refreshErr)
			if got != tc.want || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && err != nil) {
				t.Fatalf("route = (%q, %v), want (%q, %v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
	if errors.Is(plain, explanationdocs.ErrContent) {
		t.Fatal("fixture error must not be a content failure")
	}
}

func TestCommentFloorFailures_AnOffStageOrNoWorktreeJudgesNothing(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// restates the helper.\nfunc a() {}\n")
	enforce := floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`)
	off := floorRoot(t, `{"comment_floor":{"stage":"off"}}`)

	if got := commentFloorFailures(context.Background(), floorInput(t, off, repo, base)); got != nil {
		t.Errorf("stage off returned failures %v", got)
	}
	noWorktree := floorInput(t, enforce, repo, base)
	noWorktree.Worktree = ""
	if got := commentFloorFailures(context.Background(), noWorktree); got != nil {
		t.Errorf("no worktree returned failures %v", got)
	}
	if got := commentFloorFailures(context.Background(), floorInput(t, enforce, repo, base)); len(got) != 1 {
		t.Fatalf("control: enforce on the same worktree must name the comment, got %v", got)
	}
}

func TestCommentFloorFailures_AnOffStageNeverReadsTheDiff(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo.Dir, "d"), filepath.Join(repo.Dir, "go", "p", "dir.go")); err != nil {
		t.Fatal(err)
	}
	off := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"off"}}`), repo, base)
	enforce := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)

	var offLog, enforceLog injectedStderr
	off.Stderr, enforce.Stderr = &offLog, &enforceLog

	commentFloorFailures(context.Background(), off)
	if log := offLog.String(); log != "" {
		t.Fatalf("stage off read the diff and warned: %q", log)
	}
	commentFloorFailures(context.Background(), enforce)
	if log := enforceLog.String(); !strings.Contains(log, "could not read the diff") {
		t.Fatalf("control: enforce must read the unreadable diff and warn, got %q", log)
	}
}
