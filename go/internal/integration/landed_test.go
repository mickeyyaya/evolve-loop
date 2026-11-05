package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const landedCycle = 4242

type composed struct {
	repo   *gittest.Repo
	base   string
	lane   string
	patch  string
	record DeliverableRecord
}

func headOf(r *gittest.Repo) string { return strings.TrimSpace(r.Git("rev-parse", "HEAD")) }

func composeLane(t *testing.T, composeMessage string) composed {
	t.Helper()
	r := gittest.Fixture(t)
	r.Write("base.txt", "base\n")
	r.Git("add", ".")
	r.Git("commit", "-q", "-m", "base")
	c := composed{repo: r, base: headOf(r)}
	r.Git("checkout", "-q", "-b", "lane")
	r.Write("lane.txt", "the lane work\n")
	r.Git("add", ".")
	r.Git("commit", "-q", "-m", "the lane work")
	c.lane = headOf(r)
	patch, err := Repo{Dir: r.Dir, Git: SysGit()}.PatchID(context.Background(), c.lane)
	if err != nil {
		t.Fatal(err)
	}
	c.patch = patch
	r.Git("checkout", "-q", "-")
	r.Git("cherry-pick", "--no-commit", c.lane)
	r.Git("commit", "-q", "-m", composeMessage)
	c.record = DeliverableRecord{Wave: "w1", Cycle: landedCycle, Trailer: CycleTrailer(landedCycle),
		Commits: []CommitIdentity{{SHA: c.lane, BaseSHA: c.base, PatchID: patch}}}
	return c
}

func TestPatchID_AMovedCommitKeepsItsPatchID(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work")
	moved, err := Repo{Dir: c.repo.Dir, Git: SysGit()}.PatchID(context.Background(), headOf(c.repo))
	if err != nil {
		t.Fatal(err)
	}
	if moved != c.patch || c.patch == "" {
		t.Fatalf("patch id after the compose = %q, want %q", moved, c.patch)
	}
	if headOf(c.repo) == c.lane {
		t.Fatal("precondition: the compose must give the commit a new SHA")
	}
}

func TestPatchID_FailsLoudly(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose")
	ctx := context.Background()
	if id, err := (Repo{Dir: c.repo.Dir, Git: SysGit()}).PatchID(ctx, "0000000000000000000000000000000000000000"); err == nil {
		t.Errorf("unknown commit: id=%q, want an error", id)
	}
	c.repo.Git("commit", "-q", "--allow-empty", "-m", "empty")
	if id, err := (Repo{Dir: c.repo.Dir, Git: SysGit()}).PatchID(ctx, headOf(c.repo)); err == nil {
		t.Errorf("empty commit: id=%q, want an error", id)
	}
	patchFault := func(_ context.Context, _, stdin string, args ...string) (string, error) {
		if args[0] == "patch-id" {
			return "", errors.New("patch-id broke")
		}
		return "diff --git a b\n+x\n", nil
	}
	if _, err := (Repo{Dir: c.repo.Dir, Git: patchFault}).PatchID(ctx, c.lane); err == nil {
		t.Error("patch-id fault: want an error")
	}
	silent := func(_ context.Context, _, _ string, args ...string) (string, error) {
		if args[0] == "patch-id" {
			return "\n", nil
		}
		return "diff --git a b\n+x\n", nil
	}
	if _, err := (Repo{Dir: c.repo.Dir, Git: silent}).PatchID(ctx, c.lane); err == nil {
		t.Error("silent patch-id: want an error")
	}
}

func TestSysGit_ReportsAFailedCommandWithItsStderr(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := SysGit()(context.Background(), dir, "", "rev-parse", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "rev-parse HEAD") {
		t.Fatalf("err = %v, want the failed command named", err)
	}
	out, err := SysGit()(context.Background(), dir, "", "init", "-q")
	if err != nil {
		t.Fatalf("init: %v %q", err, out)
	}
	id, err := SysGit()(context.Background(), dir, "diff --git a/f b/f\nnew file mode 100644\n--- /dev/null\n+++ b/f\n@@ -0,0 +1 @@\n+x\n", "patch-id", "--stable")
	if err != nil || strings.TrimSpace(id) == "" {
		t.Fatalf("stdin reaches git: %q %v", id, err)
	}
}

func TestLandedOnRefFindsACycleByTrailerAndPatchIDAfterACompose(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	if !(Repo{Dir: c.repo.Dir, Git: SysGit()}).LandedOnRef(context.Background(), c.record, "HEAD") {
		t.Fatal("the cycle landed under a new SHA with its trailer and patch, want found")
	}
	if headOf(c.repo) == c.lane {
		t.Fatal("precondition: the SHA must differ")
	}
}

func TestLandedOnRefFindsACycleNamedBySubject(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "evolve-cycle 4242: the lane work")
	if !(Repo{Dir: c.repo.Dir, Git: SysGit()}).LandedOnRef(context.Background(), c.record, "HEAD") {
		t.Fatal("the subject form of the trailer must match")
	}
}

func TestLandedOnRefRejectsAnythingShortOfBothTrailerAndPatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	noTrailer := composeLane(t, "compose: the lane work")
	if (Repo{Dir: noTrailer.repo.Dir, Git: SysGit()}).LandedOnRef(ctx, noTrailer.record, "HEAD") {
		t.Error("a patch without the trailer must not match")
	}

	other := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	rec := other.record
	rec.Commits = []CommitIdentity{{SHA: other.lane, BaseSHA: other.base, PatchID: "0000000000000000000000000000000000000000"}}
	if (Repo{Dir: other.repo.Dir, Git: SysGit()}).LandedOnRef(ctx, rec, "HEAD") {
		t.Error("a trailer with another patch must not match")
	}
	wrongCycle := other.record
	wrongCycle.Cycle = landedCycle + 1
	if (Repo{Dir: other.repo.Dir, Git: SysGit()}).LandedOnRef(ctx, wrongCycle, "HEAD") {
		t.Error("another cycle's trailer must not match")
	}
	if (Repo{Dir: other.repo.Dir, Git: SysGit()}).LandedOnRef(ctx, other.record, "no-such-ref") {
		t.Error("an unknown ref must not match")
	}
	unknownBase := other.record
	unknownBase.Commits = []CommitIdentity{{SHA: other.lane, BaseSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", PatchID: other.patch}}
	if (Repo{Dir: other.repo.Dir, Git: SysGit()}).LandedOnRef(ctx, unknownBase, "HEAD") {
		t.Error("an unknown base must not match")
	}
	if (Repo{Dir: other.repo.Dir, Git: SysGit()}).LandedOnRef(ctx, DeliverableRecord{Cycle: landedCycle}, "HEAD") {
		t.Error("a record with no commits must not match")
	}
}

func TestLandedOnRefNeverReadsABaseAsAGitOption(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	planted := filepath.Join(t.TempDir(), "planted")
	forged := c.record
	forged.Commits = []CommitIdentity{{SHA: c.lane, BaseSHA: "--output=" + planted, PatchID: c.patch}}
	if (Repo{Dir: c.repo.Dir, Git: SysGit()}).LandedOnRef(context.Background(), forged, "HEAD") {
		t.Error("a base that is a git option matched")
	}
	if _, err := os.Stat(planted + "..HEAD"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("git read the base as an option and wrote %s: %v", planted+"..HEAD", err)
	}
}

func TestLandedOnRefSkipsACandidateWhoseDiffCannotBeRead(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	sys := SysGit()
	broken := func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
		if args[0] == "diff-tree" && args[len(args)-1] != c.lane {
			return "", errors.New("diff-tree broke")
		}
		return sys(ctx, dir, stdin, args...)
	}
	if (Repo{Dir: c.repo.Dir, Git: broken}).LandedOnRef(context.Background(), c.record, "HEAD") {
		t.Error("an unreadable diff must not match")
	}
}

func writeRecordFor(t *testing.T, root, wave string, cycle int, sha string) {
	t.Helper()
	rec := DeliverableRecord{Wave: wave, Cycle: cycle, Trailer: CycleTrailer(cycle),
		Commits: []CommitIdentity{{SHA: sha, BaseSHA: "b", PatchID: "p"}}}
	if _, err := WriteRecord(filepath.Join(root, ".evolve", "waves", "integration-"+wave), rec); err != nil {
		t.Fatal(err)
	}
}

func TestFindRecord_FindsByCycleAcrossWaves(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecordFor(t, root, "w1", 7, "aaa")
	writeRecordFor(t, root, "w2", 8, "bbb")
	r, ok := recordOfCycle(root, 8)
	if !ok || r.Wave != "w2" || r.Commits[0].SHA != "bbb" {
		t.Fatalf("recordOfCycle(8) = %+v, %v", r, ok)
	}
	if _, ok := recordOfCycle(root, 9); ok {
		t.Error("recordOfCycle(9) found a record that does not exist")
	}
}

func TestFindRecord_SkipsAnUnreadableRecordAndABadPattern(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bad := filepath.Join(root, ".evolve", "waves", "integration-w0")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "deliverable-7.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRecordFor(t, root, "w1", 7, "aaa")
	r, ok := recordOfCycle(root, 7)
	if !ok || r.Wave != "w1" {
		t.Fatalf("recordOfCycle(7) = %+v, %v, want the readable record", r, ok)
	}
	if _, ok := recordOfCycle("[", 7); ok {
		t.Error("a malformed root pattern must find nothing")
	}
}

func TestFindRecordByCommit_FindsByAnyCommitSHA(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecordFor(t, root, "w1", 7, "aaa")
	writeRecordFor(t, root, "w1", 8, "bbb")
	r, ok := FindRecordByCommit(root, "bbb")
	if !ok || r.Cycle != 8 {
		t.Fatalf("FindRecordByCommit(bbb) = %+v, %v", r, ok)
	}
	if _, ok := FindRecordByCommit(root, "ccc"); ok {
		t.Error("an unknown commit found a record")
	}
}

func TestLandedOnRefReadsEachShaFromGitNotFromAMessage(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work")
	planted := filepath.Join(t.TempDir(), "planted")
	forgedOption := "x\x1e--output=" + planted + "\x1f" + CycleTrailer(landedCycle) + "\n--output=" + planted
	forgedLanding := "y\x1e" + c.lane + "\x1f" + CycleTrailer(landedCycle) + "\n" + c.lane + "\n" + CycleTrailer(landedCycle)
	for _, message := range []string{forgedOption, forgedLanding} {
		c.repo.Git("commit", "-q", "--allow-empty", "-m", message)
	}
	if (Repo{Dir: c.repo.Dir, Git: SysGit()}).LandedOnRef(context.Background(), c.record, "HEAD") {
		t.Error("a message that names the lane sha made an unlanded cycle count as landed")
	}
	if _, err := os.Stat(planted); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a message reached git as an option and wrote %s: %v", planted, err)
	}
}

func TestLandedOnRefRefusesARecordWhosePatchIsNotItsOwnCommit(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	c.repo.Git("checkout", "-q", "-b", "unlanded", c.base)
	c.repo.Write("unlanded.txt", "never landed\n")
	c.repo.Git("add", ".")
	c.repo.Git("commit", "-q", "-m", "unlanded work")
	planted := c.record
	planted.Commits = []CommitIdentity{{SHA: headOf(c.repo), BaseSHA: c.base, PatchID: c.patch}}
	c.repo.Git("checkout", "-q", "main")
	if (Repo{Dir: c.repo.Dir, Git: SysGit()}).LandedOnRef(context.Background(), planted, "HEAD") {
		t.Error("a record that pairs an unlanded sha with the patch of a landed commit counts as landed")
	}
}

func TestPatchID_RefusesAnythingButAFullObjectID(t *testing.T) {
	t.Parallel()
	called := false
	spy := func(context.Context, string, string, ...string) (string, error) { called = true; return "", nil }
	for _, sha := range []string{"--output=/tmp/x", "HEAD", "abc123", strings.Repeat("g", 40)} {
		if _, err := (Repo{Dir: t.TempDir(), Git: spy}).PatchID(context.Background(), sha); err == nil || !strings.Contains(err.Error(), "not a full object id") {
			t.Errorf("PatchID(%q) err = %v, want a refusal", sha, err)
		}
	}
	if called {
		t.Error("git ran for a sha that is not a full object id")
	}
}

type peerFixture struct {
	composeFixture
	record DeliverableRecord
}

func peerEditNearALaneChange(t *testing.T, laneMessage string) peerFixture {
	t.Helper()
	f := newComposeFixture(t)
	rec := f.lane(t, landedCycle, map[string]string{sharedFile: "one\ntwo\nthree\nfour\nFIVE\nsix\nseven\neight\nnine\nten\n"})
	f.g.Write(sharedFile, "one\ntwo\nthree\nfour\nfive\nsix\nSEVEN\neight\nnine\nten\n")
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", "peer edit two lines away")
	f.g.Git("cherry-pick", "--no-commit", rec.Commits[0].SHA)
	f.g.Git("commit", "-q", "-m", laneMessage)
	return peerFixture{composeFixture: f, record: rec}
}

func TestLandedOnRefSurvivesAPeerEditTwoLinesFromTheCycleChange(t *testing.T) {
	t.Parallel()
	p := peerEditNearALaneChange(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle))
	if !p.repo.LandedOnRef(context.Background(), p.record, "HEAD") {
		t.Fatal("a peer edit two lines from the cycle change made a composed cycle read as not landed")
	}
}

func TestLandedOnRefRejectsACommitWithOtherContentAfterAPeerEdit(t *testing.T) {
	t.Parallel()
	p := peerEditNearALaneChange(t, "compose: the lane work\n\n"+CycleTrailer(landedCycle+1))
	p.g.Write(sharedFile, "one\ntwo\nthree\nfour\nOTHER\nsix\nSEVEN\neight\nnine\nten\n")
	p.g.Git("add", ".")
	p.g.Git("commit", "-q", "-m", "other content\n\n"+CycleTrailer(landedCycle))
	if p.repo.LandedOnRef(context.Background(), p.record, "HEAD") {
		t.Fatal("a commit with the trailer and other content read as landed")
	}
}

func TestPatchID_AsksGitForADiffOfOneContextLine(t *testing.T) {
	t.Parallel()
	var seen []string
	spy := func(_ context.Context, _, _ string, args ...string) (string, error) {
		if args[0] == "diff-tree" {
			seen = args
			return "diff --git a b\n+x\n", nil
		}
		return "abc 000\n", nil
	}
	if _, err := (Repo{Dir: t.TempDir(), Git: spy}).PatchID(context.Background(), strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(seen, "-U1") {
		t.Fatalf("diff-tree args = %v, want -U1", seen)
	}
}

func landOnMain(p composeFixture, content, message string) {
	p.g.Write(sharedFile, content)
	p.g.Git("add", ".")
	p.g.Git("commit", "-q", "-m", message)
}

func TestLandedOnRefRejectsTheSameLinesAtAnotherPosition(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, landedCycle, map[string]string{sharedFile: "one\ntwo\nNEW\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	landOnMain(f, "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nNEW\nnine\nten\n", "other place\n\n"+CycleTrailer(landedCycle))
	if f.repo.LandedOnRef(context.Background(), rec, "HEAD") {
		t.Fatal("the same added line at another place in the file read as landed")
	}
}

func TestLandedOnRefRejectsAChangeThatDiffersOnlyInWhitespace(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, landedCycle, map[string]string{sharedFile: "one\ntwo\n\tNEW\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	landOnMain(f, "one\ntwo\n    NEW\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n", "other spacing\n\n"+CycleTrailer(landedCycle)+"\n")
	if f.repo.LandedOnRef(context.Background(), rec, "HEAD") {
		t.Fatal("a change that differs only in whitespace read as landed")
	}
}

func TestLandedOnRefReadsTheSameChangeAtTheSamePlaceAsLanded(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	rec := f.lane(t, landedCycle, map[string]string{sharedFile: "one\ntwo\nNEW\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"})
	landOnMain(f, "one\ntwo\nNEW\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n", "same place\n\n"+CycleTrailer(landedCycle))
	if !f.repo.LandedOnRef(context.Background(), rec, "HEAD") {
		t.Fatal("the same change at the same place read as not landed")
	}
}

func TestLandedOnRefNeedsACommitForEachCommitOfTheRecord(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	f.g.Write("dup.txt", "a\nb\na\nb\n")
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", "dup base")
	f.base = headOf(f.g)
	rec := f.lane(t, landedCycle, map[string]string{"dup.txt": "a\nX\nb\na\nb\n"}, map[string]string{"dup.txt": "a\nX\nb\na\nX\nb\n"})
	if rec.Commits[0].PatchID != rec.Commits[1].PatchID {
		t.Fatal("fixture: the two commits must share a patch id")
	}
	f.g.Git("cherry-pick", "--no-commit", rec.Commits[0].SHA)
	f.g.Git("commit", "-q", "-m", "half\n\n"+CycleTrailer(landedCycle))
	if f.repo.LandedOnRef(context.Background(), rec, "HEAD") {
		t.Fatal("one landed commit satisfied two commits of the record")
	}
}

func TestFindLanded_ConsumesEachLandedCommitOnce(t *testing.T) {
	t.Parallel()
	rec := DeliverableRecord{Cycle: 9, Commits: []CommitIdentity{{PatchID: "p"}, {PatchID: "p"}}}
	one := []LandedCommit{{Message: "m\n\n" + CycleTrailer(9), PatchID: "p"}}
	if FindLanded(one, rec) {
		t.Fatal("one commit satisfied two entries")
	}
	if !FindLanded(append(one, one...), rec) {
		t.Fatal("two commits did not satisfy two entries")
	}
}

func TestIsAncestor_TrueWhenTheCommitIsOnTheRef(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work")
	got, err := Repo{Dir: c.repo.Dir, Git: SysGit()}.IsAncestor(context.Background(), c.base, "HEAD")
	if err != nil || !got {
		t.Fatalf("IsAncestor(base, HEAD) = %v, %v, want true, nil", got, err)
	}
}

func TestIsAncestor_FalseWhenTheCommitIsNotOnTheRef(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work")
	got, err := Repo{Dir: c.repo.Dir, Git: SysGit()}.IsAncestor(context.Background(), c.lane, c.base)
	if err != nil || got {
		t.Fatalf("IsAncestor(lane, base) = %v, %v, want false, nil", got, err)
	}
}

func TestIsAncestor_AGitFailureIsAnErrorNotFalse(t *testing.T) {
	t.Parallel()
	c := composeLane(t, "compose: the lane work")
	repo := Repo{Dir: c.repo.Dir, Git: SysGit()}
	for _, ref := range []string{"refs/heads/absent", "--output=x"} {
		if got, err := repo.IsAncestor(context.Background(), c.base, ref); err == nil || got {
			t.Errorf("IsAncestor(base, %q) = %v, %v, want false and an error", ref, got, err)
		}
	}
	if got, err := repo.IsAncestor(context.Background(), "abc", "HEAD"); err == nil || got {
		t.Errorf("IsAncestor(abc, HEAD) = %v, %v, want false and an error for a short id", got, err)
	}
}

func TestIsAncestor_ExitOneIsFalseAndOtherCodesAreErrors(t *testing.T) {
	t.Parallel()
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name    string
		err     error
		want    bool
		wantErr bool
	}{
		{"exit 0", nil, true, false},
		{"exit 1", &GitError{Code: 1}, false, false},
		{"exit 128", &GitError{Code: 128}, false, true},
		{"exit 1 with an error", &GitError{Code: 1, Err: errors.New("boom")}, false, true},
		{"not a git error", errors.New("spawn"), false, true},
	} {
		repo := Repo{Git: func(context.Context, string, string, ...string) (string, error) { return "", tc.err }}
		got, err := repo.IsAncestor(context.Background(), sha, "HEAD")
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("%s: IsAncestor = %v, %v, want %v, error %v", tc.name, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestFirstRecord_PoolDirectoriesSortNumericallyNotLexically(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecordFor(t, root, "1000", 7, "later")
	writeRecordFor(t, root, "999", 7, "earlier")
	writeRecordFor(t, root, "console", 7, "named")
	r, ok := recordOfCycle(root, 7)
	if !ok || r.Wave != "999" || r.Commits[0].SHA != "earlier" {
		t.Fatalf("recordOfCycle(7) = %+v, %v, want the record of pool 999", r, ok)
	}
	byCommit, ok := FindRecordByCommit(root, "later")
	if !ok || byCommit.Wave != "1000" {
		t.Fatalf("FindRecordByCommit(later) = %+v, %v, want pool 1000", byCommit, ok)
	}
}

func TestFirstRecord_NamedWaveDirectoriesFollowTheNumericOnes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecordFor(t, root, "current", 7, "named")
	writeRecordFor(t, root, "5", 7, "pool")
	if r, ok := recordOfCycle(root, 7); !ok || r.Wave != "5" {
		t.Fatalf("recordOfCycle(7) = %+v, %v, want pool 5 before current", r, ok)
	}
}

func TestCompareWaveDirs_NumericPoolsFirstInNumericOrderThenNamesInTextOrder(t *testing.T) {
	t.Parallel()
	dir := func(name string) string { return filepath.Join("w", "integration-"+name, "deliverable-1.json") }
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"999", "1000", -1},
		{"1000", "999", 1},
		{"5", "current", -1},
		{"current", "5", 1},
		{"console", "current", -1},
		{"05", "5", -1},
		{"7", "7", 0},
	} {
		if got := compareWaveDirs(dir(tc.a), dir(tc.b)); got != tc.want {
			t.Errorf("compareWaveDirs(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestRecordForCommit_RequiresTheCycleAndTheCommitOfTheSameRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecordFor(t, root, "5", 7, "oldcommit")
	writeRecordFor(t, root, "9", 7, "newcommit")
	writeRecordFor(t, root, "3", 8, "othercycle")
	for _, tc := range []struct {
		name     string
		cycle    int
		sha      string
		wantWave string
		want     bool
	}{
		{"later pool of the cycle", 7, "newcommit", "9", true},
		{"earlier pool of the cycle", 7, "oldcommit", "5", true},
		{"commit of another cycle", 7, "othercycle", "", false},
		{"unknown commit", 7, "nope", "", false},
	} {
		r, ok := RecordForCommit(root, tc.cycle, tc.sha)
		if ok != tc.want || r.Wave != tc.wantWave {
			t.Errorf("%s: RecordForCommit(%d, %q) = wave %q, %v, want %q, %v", tc.name, tc.cycle, tc.sha, r.Wave, ok, tc.wantWave, tc.want)
		}
	}
}

func recordOfCycle(root string, cycle int) (DeliverableRecord, bool) {
	return firstRecord(root, strconv.Itoa(cycle)+".json", func(r DeliverableRecord) bool { return r.Cycle == cycle })
}
