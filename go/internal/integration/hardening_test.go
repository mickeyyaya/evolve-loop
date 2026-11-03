package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchID_AMergeOrAnEmptyCommitIsTheNoPatchRefusal(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose")
	repo := Repo{Dir: c.repo.Dir, Git: SysGit()}
	c.repo.Git("commit", "-q", "--allow-empty", "-m", "empty")
	empty := headOf(c.repo)
	c.repo.Git("merge", "-q", "--no-ff", "-m", "merge the lane", "lane")
	merge := headOf(c.repo)
	for name, sha := range map[string]string{"empty": empty, "merge": merge} {
		if id, err := repo.PatchID(context.Background(), sha); !errors.Is(err, ErrNoPatch) || id != "" {
			t.Errorf("%s commit: id=%q err=%v, want ErrNoPatch", name, id, err)
		}
	}
}

func TestWaveDir_RefusesAnIDOutsideTheSafeAlphabet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, wave := range []string{"", "../x", "..\\x", "a/b", "w 1", "w*", `w\x`, "w\n"} {
		if dir, err := WaveDir(root, wave); !errors.Is(err, ErrBadWaveID) || dir != "" {
			t.Errorf("WaveDir(%q) = %q, %v, want ErrBadWaveID", wave, dir, err)
		}
	}
	for _, wave := range []string{"w1", "wave-0147", "W.2_x", "..."} {
		dir, err := WaveDir(root, wave)
		if err != nil || filepath.Dir(dir) != filepath.Join(root, ".evolve", "waves") {
			t.Errorf("WaveDir(%q) = %q, %v, want a child of .evolve/waves", wave, dir, err)
		}
	}
}

func countRecordReads(t *testing.T) *int {
	t.Helper()
	reads := new(int)
	prev := readRecordFile
	readRecordFile = func(path string) ([]byte, error) {
		*reads++
		return prev(path)
	}
	t.Cleanup(func() { readRecordFile = prev })
	return reads
}

func TestFindRecordByCommit_ReadsOneRecordPerLookup(t *testing.T) {
	root := t.TempDir()
	for cycle := 1; cycle <= 5; cycle++ {
		writeRecordFor(t, root, "w1", cycle, strings.Repeat(string(rune('a'+cycle)), 40))
	}
	writeRecordFor(t, root, "w2", 6, strings.Repeat("f", 40))
	reads := countRecordReads(t)
	r, ok := FindRecordByCommit(root, strings.Repeat("d", 40))
	if !ok || r.Cycle != 3 || *reads != 1 {
		t.Fatalf("hit: cycle=%d ok=%v reads=%d, want cycle 3 with 1 read", r.Cycle, ok, *reads)
	}
	*reads = 0
	if _, ok := FindRecordByCommit(root, strings.Repeat("0", 40)); ok || *reads != 0 {
		t.Fatalf("miss: ok=%v reads=%d, want no record and 0 reads", ok, *reads)
	}
	*reads = 0
	if r, ok := FindRecord(root, 4); !ok || r.Cycle != 4 || *reads != 1 {
		t.Fatalf("FindRecord: cycle=%d ok=%v reads=%d, want cycle 4 with 1 read", r.Cycle, ok, *reads)
	}
}

func TestFindRecordByCommit_AStaleIndexEntryDoesNotVouch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	old, current := strings.Repeat("a", 40), strings.Repeat("b", 40)
	writeRecordFor(t, root, "w1", 7, old)
	writeRecordFor(t, root, "w1", 7, current)
	if r, ok := FindRecordByCommit(root, old); ok {
		t.Fatalf("a commit that the record no longer holds found %+v", r)
	}
	if r, ok := FindRecordByCommit(root, current); !ok || r.Cycle != 7 {
		t.Fatalf("FindRecordByCommit(current) = %+v, %v", r, ok)
	}
}

func TestFindRecordByCommit_RefusesAnythingButAnObjectID(t *testing.T) {
	root := t.TempDir()
	writeRecordFor(t, root, "w1", 7, strings.Repeat("a", 40))
	reads := countRecordReads(t)
	for _, sha := range []string{"*", "", "../a", strings.Repeat("a", 39) + "*"} {
		if _, ok := FindRecordByCommit(root, sha); ok || *reads != 0 {
			t.Errorf("FindRecordByCommit(%q) ok=%v reads=%d, want a refusal with no read", sha, ok, *reads)
		}
	}
}

func TestWriteRecord_RefusesACommitThatIsNotAnObjectID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rec := fullRecord()
	rec.Commits[1].SHA = "../escape"
	if _, err := WriteRecord(dir, rec); err == nil || !strings.Contains(err.Error(), "../escape") {
		t.Fatalf("err = %v, want a refusal that names the commit", err)
	}
	if _, err := os.Stat(RecordPath(dir, rec.Cycle)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused record was published: %v", err)
	}
}

func TestWriteRecord_ReportsAnIndexThatCannotBeWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, commitIndexDir), []byte("a file where the index belongs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRecord(dir, fullRecord()); err == nil || !strings.Contains(err.Error(), "index") {
		t.Fatalf("err = %v, want an index failure", err)
	}
}

func TestLandedSince_BoundsTheSearchByTheCommitBeforeTheCycle(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	repo := Repo{Dir: c.repo.Dir, Git: SysGit()}
	ctx := context.Background()
	if !repo.LandedSince(ctx, c.record, "HEAD", c.base) {
		t.Fatal("the compose came after the base, so the cycle landed since the base")
	}
	if repo.LandedSince(ctx, c.record, "HEAD", headOf(c.repo)) {
		t.Error("the compose is an ancestor of the bound, so it is not a landing since the bound")
	}
	for _, since := range []string{"", "HEAD", "--all", c.base[:12]} {
		if repo.LandedSince(ctx, c.record, "HEAD", since) {
			t.Errorf("LandedSince(%q) found a landing; a bound that is not a full object id must fail closed", since)
		}
	}
}

func TestDiffPatchID_IsTheOneStablePatchIDOfADiff(t *testing.T) {
	ctx := context.Background()
	repo := Repo{Dir: t.TempDir(), Git: SysGit()}
	diff := "diff --git a/f b/f\nnew file mode 100644\n--- /dev/null\n+++ b/f\n@@ -0,0 +1 @@\n+x\n"
	id, err := repo.DiffPatchID(ctx, diff)
	if err != nil || !objectID.MatchString(id) {
		t.Fatalf("DiffPatchID = %q, %v, want a 40-hex patch-id", id, err)
	}
	if got, err := repo.DiffPatchID(ctx, "not a diff\n"); err == nil || got != "" || !strings.Contains(err.Error(), "empty output") {
		t.Errorf("a non-diff: %q, %v, want the empty-output error", got, err)
	}
	t.Setenv("PATH", t.TempDir())
	if got, err := repo.DiffPatchID(ctx, diff); !errors.Is(err, exec.ErrNotFound) || !strings.HasPrefix(err.Error(), "git patch-id --stable: ") || got != "" {
		t.Errorf("no git: %q, %v, want a git patch-id error that wraps exec.ErrNotFound", got, err)
	}
}

func TestWriteRecord_ReportsAnIndexEntryThatCannotBeCreated(t *testing.T) {
	prev := openIndexEntry
	openIndexEntry = func(string, int, os.FileMode) (*os.File, error) { return nil, errors.New("disk full") }
	t.Cleanup(func() { openIndexEntry = prev })
	dir := t.TempDir()
	if _, err := WriteRecord(dir, fullRecord()); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want the index entry failure", err)
	}
	if _, err := os.Stat(RecordPath(dir, 42)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a record without its index was published: %v", err)
	}
}

func TestFindRecordByCommit_SkipsAMalformedEntryAndABadPattern(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sha := strings.Repeat("a", 40)
	writeRecordFor(t, root, "w1", 7, sha)
	for _, suffix := range []string{"-x", "--3", "-+7", "-99999999999999999999"} {
		planted := filepath.Join(root, ".evolve", "waves", "integration-w1", commitIndexDir, sha+suffix)
		if err := os.WriteFile(planted, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := indexedCycle(sha+"--3", sha); ok {
		t.Error("a signed cycle in an index entry must not parse")
	}
	if _, ok := indexedCycle("other-7", sha); ok {
		t.Error("an entry of another commit must not parse")
	}
	if r, ok := FindRecordByCommit(root, sha); !ok || r.Cycle != 7 {
		t.Fatalf("FindRecordByCommit = %+v, %v, want cycle 7 past the malformed entry", r, ok)
	}
	if _, ok := FindRecordByCommit("[", sha); ok {
		t.Error("a malformed root pattern must find nothing")
	}
}

func TestWriteRecord_ARewriteKeepsTheIndexEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sha := strings.Repeat("c", 40)
	writeRecordFor(t, root, "w1", 9, sha)
	writeRecordFor(t, root, "w1", 9, sha)
	entries, err := os.ReadDir(filepath.Join(root, ".evolve", "waves", "integration-w1", commitIndexDir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("index entries = %v, %v, want one entry after a rewrite", entries, err)
	}
	if r, ok := FindRecordByCommit(root, sha); !ok || r.Cycle != 9 {
		t.Fatalf("FindRecordByCommit = %+v, %v", r, ok)
	}
}

func TestFindRecordByCommit_AStaleEntryBeforeTheCurrentOneDoesNotHideIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	moved := strings.Repeat("1", 40)
	writeRecordFor(t, root, "w1", 7, moved)
	writeRecordFor(t, root, "w1", 7, strings.Repeat("2", 40))
	writeRecordFor(t, root, "w1", 8, moved)
	if r, ok := FindRecordByCommit(root, moved); !ok || r.Cycle != 8 {
		t.Fatalf("FindRecordByCommit = %+v, %v, want cycle 8 past the stale entry of cycle 7", r, ok)
	}
}

func TestFindRecordByCommit_ANonObjectIDNeverReachesTheIndex(t *testing.T) {
	root := t.TempDir()
	writeRecordFor(t, root, "w1", 7, strings.Repeat("a", 40))
	planted := filepath.Join(root, ".evolve", "waves", "integration-w1", commitIndexDir, "zz-7")
	if err := os.WriteFile(planted, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	reads := countRecordReads(t)
	if _, ok := FindRecordByCommit(root, "zz"); ok || *reads != 0 {
		t.Fatalf("ok=%v reads=%d, want a refusal before any read", ok, *reads)
	}
}

func TestErrBadWaveIDNamesThePatternWaveDirEnforces(t *testing.T) {
	t.Parallel()
	if !strings.HasSuffix(ErrBadWaveID.Error(), waveID.String()) {
		t.Fatalf("ErrBadWaveID = %q, want it to name %s", ErrBadWaveID, waveID)
	}
}
