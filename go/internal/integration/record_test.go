package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type closerSpy struct {
	closed   []int
	branches []string
	err      error
}

var _ PRCloser = (*closerSpy)(nil)

func (c *closerSpy) ClosePR(_ context.Context, number int, headBranch string) error {
	c.closed = append(c.closed, number)
	c.branches = append(c.branches, headBranch)
	return c.err
}

type originFixture struct {
	composeFixture
	remote *gittest.Repo
}

func newOriginFixture(t *testing.T) originFixture {
	t.Helper()
	f := newComposeFixture(t)
	remote := gittest.Bare(t)
	f.g.Git("remote", "add", "origin", remote.Dir)
	f.g.Git("push", "-q", "origin", "main")
	f.g.Git("fetch", "-q", "origin")
	return originFixture{composeFixture: f, remote: remote}
}

func (f originFixture) commitOn(t *testing.T, branch, name string) string {
	t.Helper()
	f.g.Git("checkout", "-q", "-B", branch, f.base)
	f.g.Write(name, name+"\n")
	f.g.Git("add", ".")
	f.g.Git("commit", "-q", "-m", name)
	return headOf(f.g)
}

func sampleRecord() IntegrationRecord {
	return IntegrationRecord{
		Wave:    "135",
		Index:   1,
		Base:    strings.Repeat("b", 40),
		Branch:  "integration/w135-b1",
		Tip:     strings.Repeat("a", 40),
		PR:      77,
		Members: []int{12, 13},
		Epoch:   3,
	}
}

func TestIntegrationRecord_WritesAndReadsEveryField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := sampleRecord()
	path, err := WriteIntegrationRecord(root, want)
	if err != nil {
		t.Fatalf("WriteIntegrationRecord: %v", err)
	}
	if path != IntegrationRecordPath(root, "135") {
		t.Errorf("path = %q, want %q", path, IntegrationRecordPath(root, "135"))
	}
	if filepath.Base(path) != "integration-135.json" || filepath.Base(filepath.Dir(path)) != "waves" {
		t.Errorf("path %q is not .evolve/waves/integration-<wave>.json", path)
	}
	got, err := ReadIntegrationRecord(path)
	if err != nil {
		t.Fatalf("ReadIntegrationRecord: %v", err)
	}
	if got.Schema != IntegrationSchema || got.Wave != want.Wave || got.Index != 1 || got.Base != want.Base || got.Branch != want.Branch || got.Tip != want.Tip ||
		got.PR != want.PR || got.Epoch != want.Epoch || len(got.Members) != 2 || got.Members[1] != 13 {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestIntegrationRecord_RefusesAnInvalidRecordOnWrite(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*IntegrationRecord){
		"a wave that is not a name": func(r *IntegrationRecord) { r.Wave = "../x" },
		"an empty branch":           func(r *IntegrationRecord) { r.Branch = "" },
		"a tip that is not an id":   func(r *IntegrationRecord) { r.Tip = "main" },
		"a negative pr":             func(r *IntegrationRecord) { r.PR = -1 },
		"an epoch of zero":          func(r *IntegrationRecord) { r.Epoch = 0 },
		"the zero object id":        func(r *IntegrationRecord) { r.Tip = strings.Repeat("0", 40) },
		"a base that is not an id":  func(r *IntegrationRecord) { r.Base = "" },
		"an index of zero":          func(r *IntegrationRecord) { r.Index = 0 },
		"a member below one":        func(r *IntegrationRecord) { r.Members = []int{12, 0} },
		"a repeated member":         func(r *IntegrationRecord) { r.Members = []int{12, 13, 12} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			r := sampleRecord()
			mutate(&r)
			if _, err := WriteIntegrationRecord(root, r); err == nil {
				t.Fatal("WriteIntegrationRecord accepted an invalid record")
			}
			if _, err := os.Stat(filepath.Join(root, ".evolve")); err == nil {
				t.Error("an invalid record left files behind")
			}
		})
	}
}

func TestReadIntegrationRecord_RefusesWhatDoesNotParseOrValidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"missing.json": "",
		"junk.json":    "{",
		"schema.json":  `{"schema":"other/1","wave":"1"}`,
		"fields.json":  `{"schema":"` + IntegrationSchema + `","wave":"1"}`,
	} {
		path := filepath.Join(dir, name)
		if body != "" {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := ReadIntegrationRecord(path); err == nil {
			t.Errorf("%s: ReadIntegrationRecord accepted it", name)
		}
	}
}

func TestWriteIntegrationRecord_ReplacesAtomicallyAndLeavesNoStagingFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := WriteIntegrationRecord(root, sampleRecord()); err != nil {
		t.Fatal(err)
	}
	next := sampleRecord()
	next.Epoch = 4
	path, err := WriteIntegrationRecord(root, next)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadIntegrationRecord(path)
	if err != nil || got.Epoch != 4 {
		t.Fatalf("read after replace = %+v, %v; want epoch 4", got, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want only the record", len(entries))
	}
}

func TestRecordFile_ReportsAFailedDirectoryCreation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f := recordFile{Path: filepath.Join(blocker, "x.json"), Kind: "integration record", Prefix: integrationRecordPrefix}
	if _, err := f.publish(sampleRecord()); err == nil {
		t.Fatal("publish hid a directory failure")
	}
}

func TestRecordFile_ReportsAFailedRenameAndRemovesItsStagingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocking := filepath.Join(dir, "integration-135.json")
	if err := os.MkdirAll(blocking, 0o755); err != nil {
		t.Fatal(err)
	}
	f := recordFile{Path: blocking, Kind: "integration record", Prefix: integrationRecordPrefix}
	if _, err := f.publish(sampleRecord()); err == nil {
		t.Fatal("publish hid a rename failure")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("a failed publish left %d entries, want the blocking directory only", len(entries))
	}
}

func TestWriteIntegrationRecord_NeverReplacesAFileItCannotReadBack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path, err := WriteIntegrationRecord(root, sampleRecord())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := sampleRecord()
	next.Epoch = 4
	if _, err := WriteIntegrationRecord(root, next); err == nil {
		t.Fatal("WriteIntegrationRecord replaced a corrupt record")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{" {
		t.Errorf("the corrupt file changed to %q", raw)
	}
}

func TestAdopter_ASecondLeaseHolderAdoptsTheRecordedBranchAndClosesTheOldPR(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	tip := f.commitOn(t, "integration/w135-b1", "a.txt")
	rec := sampleRecord()
	rec.Tip = tip
	spy := &closerSpy{}
	a := Adopter{Repo: f.repo, Closer: spy}

	got, err := a.Adopt(context.Background(), rec, 4)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if got.Epoch != 4 || got.Branch != rec.Branch || got.Tip == tip || got.PR != 0 {
		t.Errorf("adopted = %+v, want epoch 4, same branch, a fence tip, no PR", got)
	}
	if parent := strings.TrimSpace(f.g.Git("rev-parse", got.Tip+"^")); parent != tip {
		t.Errorf("fence parent = %s, want %s", parent, tip)
	}
	if len(spy.closed) != 1 || spy.closed[0] != 77 {
		t.Errorf("closed PRs = %v, want [77]", spy.closed)
	}
}

func TestAdopter_TheSameEpochKeepsTheRecordAndItsPR(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	tip := f.commitOn(t, "integration/w135-b1", "a.txt")
	rec := sampleRecord()
	rec.Tip = tip
	spy := &closerSpy{}
	got, err := Adopter{Repo: f.repo, Closer: spy}.Adopt(context.Background(), rec, 3)
	if err != nil || got.PR != 77 || got.Epoch != 3 || len(spy.closed) != 0 {
		t.Errorf("Adopt = %+v, %v, closed %v; want the record unchanged", got, err, spy.closed)
	}
}

func TestAdopter_AStaleHolderCannotAdopt(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	tip := f.commitOn(t, "integration/w135-b1", "a.txt")
	rec := sampleRecord()
	rec.Tip = tip
	spy := &closerSpy{}
	if _, err := (Adopter{Repo: f.repo, Closer: spy}).Adopt(context.Background(), rec, 2); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("Adopt error = %v, want ErrStaleEpoch", err)
	}
	if len(spy.closed) != 0 {
		t.Errorf("a stale holder closed %v", spy.closed)
	}
}

func TestAdopter_ABranchThatMovedIsNotAdopted(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	f.commitOn(t, "integration/w135-b1", "a.txt")
	rec := sampleRecord()
	spy := &closerSpy{}
	if _, err := (Adopter{Repo: f.repo, Closer: spy}).Adopt(context.Background(), rec, 4); !errors.Is(err, ErrBranchMoved) {
		t.Fatalf("Adopt error = %v, want ErrBranchMoved", err)
	}
	if len(spy.closed) != 0 {
		t.Errorf("a branch that moved still closed %v", spy.closed)
	}
}

func TestAdopter_AMissingBranchIsNotAdopted(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	if _, err := (Adopter{Repo: f.repo, Closer: &closerSpy{}}).Adopt(context.Background(), sampleRecord(), 4); !errors.Is(err, ErrBranchMoved) {
		t.Fatalf("Adopt error = %v, want ErrBranchMoved", err)
	}
}

func TestAdopter_ARecordWithoutAPRClosesNothing(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	tip := f.commitOn(t, "integration/w135-b1", "a.txt")
	rec := sampleRecord()
	rec.Tip, rec.PR = tip, 0
	spy := &closerSpy{}
	got, err := Adopter{Repo: f.repo, Closer: spy}.Adopt(context.Background(), rec, 4)
	if err != nil || got.Epoch != 4 || len(spy.closed) != 0 {
		t.Errorf("Adopt = %+v, %v, closed %v", got, err, spy.closed)
	}
}

func TestAdopter_AFailedCloseFailsTheAdoption(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	tip := f.commitOn(t, "integration/w135-b1", "a.txt")
	rec := sampleRecord()
	rec.Tip = tip
	boom := errors.New("gh down")
	_, err := Adopter{Repo: f.repo, Closer: &closerSpy{err: boom}}.Adopt(context.Background(), rec, 4)
	if !errors.Is(err, boom) {
		t.Fatalf("Adopt error = %v, want the close error", err)
	}
}

func TestAdopter_RefusesAnInvalidRecordBeforeGit(t *testing.T) {
	t.Parallel()
	calls := 0
	r := Repo{Git: func(context.Context, string, string, ...string) (string, error) { calls++; return "", nil }}
	rec := sampleRecord()
	rec.Tip = "main"
	_, err := Adopter{Repo: r, Closer: &closerSpy{}}.Adopt(context.Background(), rec, 4)
	if err == nil || errors.Is(err, ErrBranchMoved) || calls != 0 {
		t.Errorf("Adopt = %v after %d git calls, want a validation refusal before git", err, calls)
	}
}

func (f originFixture) cutBatch(t *testing.T) (Batch, IntegrationRecord) {
	t.Helper()
	b, err := f.repo.StartBatchFromOrigin(context.Background(), "135", 1)
	if err != nil {
		t.Fatalf("StartBatchFromOrigin: %v", err)
	}
	c := f.commitOn(t, "scratch", "a.txt")
	f.g.Git("update-ref", branchRef(b.Branch), c, b.Tip)
	rec := sampleRecord()
	rec.Base, rec.Tip = b.Base, c
	return b, rec
}

func TestBatchLifecycle_CutPublishRecordAdoptPushWithNoHandPush(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	_, rec := f.cutBatch(t)
	if err := f.repo.PublishBranch(ctx, rec); err != nil {
		t.Fatalf("PublishBranch: %v", err)
	}
	if got := strings.TrimSpace(f.remote.Git("rev-parse", branchRef(rec.Branch))); got != rec.Tip {
		t.Fatalf("remote branch = %s, want %s", got, rec.Tip)
	}
	root := t.TempDir()
	path, err := WriteIntegrationRecord(root, rec)
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadIntegrationRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := (Adopter{Repo: f.repo}).Adopt(ctx, read, read.Epoch); err != nil || got.Tip != rec.Tip {
		t.Fatalf("resume at the same epoch = %+v, %v", got, err)
	}
	f.g.Git("checkout", "-q", "scratch")
	f.g.Git("commit", "-q", "--allow-empty", "-m", "second")
	second := headOf(f.g)
	if err := f.repo.PushBranch(ctx, read, second); err != nil {
		t.Fatalf("PushBranch on the recorded tip: %v", err)
	}
	if got := strings.TrimSpace(f.remote.Git("rev-parse", branchRef(rec.Branch))); got != second {
		t.Errorf("remote branch = %s, want %s", got, second)
	}
	if err := f.repo.PublishBranch(ctx, rec); err == nil {
		t.Error("PublishBranch overwrote a branch that the remote already holds")
	}
}

func TestPublishBranch_RefusesAnInvalidRecordBeforeGit(t *testing.T) {
	t.Parallel()
	calls := 0
	r := Repo{Git: func(context.Context, string, string, ...string) (string, error) { calls++; return "", nil }}
	rec := sampleRecord()
	rec.Tip = strings.Repeat("0", 40)
	if err := r.PublishBranch(context.Background(), rec); err == nil || calls != 0 {
		t.Errorf("PublishBranch = %v after %d git calls, want a refusal before git", err, calls)
	}
}

func TestPushBranch_AStaleTipFails(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	first := f.commitOn(t, "integration/w135-b1", "a.txt")
	f.g.Git("push", "-q", "origin", first+":refs/heads/integration/w135-b1")
	f.g.Git("commit", "-q", "--allow-empty", "-m", "other writer")
	other := headOf(f.g)
	f.g.Git("push", "-q", "origin", other+":refs/heads/integration/w135-b1")

	f.g.Git("commit", "-q", "--allow-empty", "-m", "mine")
	mine := headOf(f.g)
	stale := sampleRecord()
	stale.Tip = first
	err := f.repo.PushBranch(context.Background(), stale, mine)
	if err == nil {
		t.Fatal("a write with a stale tip succeeded")
	}
	if got := strings.TrimSpace(f.remote.Git("rev-parse", "refs/heads/integration/w135-b1")); got != other {
		t.Errorf("remote branch = %s, want the other writer's %s", got, other)
	}
}

func TestPushBranch_RefusesUnsafeInputBeforeGit(t *testing.T) {
	t.Parallel()
	calls := 0
	r := Repo{Dir: t.TempDir(), Git: func(context.Context, string, string, ...string) (string, error) {
		calls++
		return "", nil
	}}
	good := strings.Repeat("b", 40)
	withTip := func(tip string) IntegrationRecord { r := sampleRecord(); r.Tip = tip; return r }
	withBranch := func(b string) IntegrationRecord { r := sampleRecord(); r.Branch = b; return r }
	cases := map[string]struct {
		rec IntegrationRecord
		tip string
	}{
		"recorded tip":      {withTip("x"), good},
		"zero recorded tip": {withTip(strings.Repeat("0", 40)), good},
		"new tip":           {withTip(good), "HEAD"},
		"zero new tip":      {withTip(good), strings.Repeat("0", 40)},
		"branch":            {withBranch("x:y"), good},
		"branch of a wave":  {withBranch("integration/w999-b1"), good},
	}
	for name, c := range cases {
		if err := r.PushBranch(context.Background(), c.rec, c.tip); err == nil {
			t.Errorf("%s: PushBranch accepted it", name)
		}
	}
	if calls != 0 {
		t.Errorf("git ran %d times for unsafe input", calls)
	}
}

func TestAdopter_AdoptionFencesTheBranchSoAStaleHolderCannotPush(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	_, rec := f.cutBatch(t)
	if err := f.repo.PublishBranch(ctx, rec); err != nil {
		t.Fatal(err)
	}
	spy := &closerSpy{}
	adopted, err := Adopter{Repo: f.repo, Closer: spy}.Adopt(ctx, rec, 4)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if adopted.Tip == rec.Tip || adopted.Epoch != 4 || adopted.PR != 0 {
		t.Fatalf("adopted = %+v, want a new tip, epoch 4 and no PR", adopted)
	}
	if got := strings.TrimSpace(f.g.Git("rev-parse", adopted.Tip+"^")); got != rec.Tip {
		t.Errorf("the fence commit has parent %s, want the old tip %s", got, rec.Tip)
	}
	if got := strings.TrimSpace(f.remote.Git("rev-parse", branchRef(rec.Branch))); got != adopted.Tip {
		t.Errorf("remote branch = %s, want the fence commit %s", got, adopted.Tip)
	}
	if got := strings.TrimSpace(f.g.Git("rev-parse", branchRef(rec.Branch))); got != adopted.Tip {
		t.Errorf("local branch = %s, want the fence commit %s", got, adopted.Tip)
	}
	if len(spy.branches) != 1 || spy.branches[0] != rec.Branch {
		t.Errorf("closer saw branches %v, want [%s]", spy.branches, rec.Branch)
	}
	f.g.Git("checkout", "-q", "scratch")
	f.g.Git("commit", "-q", "--allow-empty", "-m", "stale holder")
	if err := f.repo.PushBranch(ctx, rec, headOf(f.g)); err == nil {
		t.Error("the stale holder pushed after the adoption")
	}
	if err := f.repo.PushBranch(ctx, adopted, headOf(f.g)); err != nil {
		t.Errorf("the new holder cannot push: %v", err)
	}
}

func TestAdopter_ABranchThatWasNeverPublishedIsFencedOnTheRemoteToo(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	_, rec := f.cutBatch(t)
	adopted, err := Adopter{Repo: f.repo, Closer: &closerSpy{}}.Adopt(ctx, rec, 4)
	if err != nil || adopted.Tip == rec.Tip {
		t.Fatalf("Adopt = %+v, %v; want a fence commit", adopted, err)
	}
	if got := strings.TrimSpace(f.remote.Git("rev-parse", branchRef(rec.Branch))); got != adopted.Tip {
		t.Fatalf("remote branch = %s, want the fence commit %s", got, adopted.Tip)
	}
	if err := f.repo.PublishBranch(ctx, rec); err == nil {
		t.Error("the stale holder published a branch that the adoption fenced")
	}
	f.g.Git("checkout", "-q", "scratch")
	f.g.Git("commit", "-q", "--allow-empty", "-m", "new holder")
	if err := f.repo.PushBranch(ctx, adopted, headOf(f.g)); err != nil {
		t.Errorf("the new holder cannot push: %v", err)
	}
}

func scriptedGit(tip, failOn string) GitFunc {
	return func(_ context.Context, _, _ string, args ...string) (string, error) {
		key := args[0]
		if key == "rev-parse" && strings.HasSuffix(args[len(args)-1], "^{tree}") {
			key = "tree"
		}
		if key == failOn {
			return "", errors.New("boom " + failOn)
		}
		switch key {
		case "rev-parse":
			return tip + "\n", nil
		case "tree", "commit-tree":
			return strings.Repeat("c", 40) + "\n", nil
		case "ls-remote":
			return tip + "\trefs/heads/integration/w135-b1\n", nil
		}
		return "", nil
	}
}

func TestAdopter_EachGitFailureOfTheAdoptionIsReported(t *testing.T) {
	t.Parallel()
	rec := sampleRecord()
	for _, failOn := range []string{"rev-parse", "ls-remote", "tree", "commit-tree", "push", "update-ref"} {
		r := Repo{Git: scriptedGit(rec.Tip, failOn)}
		_, err := Adopter{Repo: r, Closer: &closerSpy{}}.Adopt(context.Background(), rec, 4)
		if err == nil || !strings.Contains(err.Error(), "boom "+failOn) {
			t.Errorf("failure of %s = %v, want it reported", failOn, err)
		}
	}
	r := Repo{Git: scriptedGit(rec.Tip, "")}
	if got, err := (Adopter{Repo: r, Closer: &closerSpy{}}).Adopt(context.Background(), rec, 4); err != nil || got.Tip != strings.Repeat("c", 40) {
		t.Errorf("scripted adoption = %+v, %v", got, err)
	}
}

func recordingGit(inner GitFunc, seen *[]string) GitFunc {
	return func(ctx context.Context, dir, in string, args ...string) (string, error) {
		*seen = append(*seen, args[0])
		return inner(ctx, dir, in, args...)
	}
}

func TestAdopter_AFailedCloseLeavesBothBranchesAtTheRecordedTip(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	_, rec := f.cutBatch(t)
	if err := f.repo.PublishBranch(ctx, rec); err != nil {
		t.Fatal(err)
	}
	_, err := Adopter{Repo: f.repo, Closer: &closerSpy{err: errors.New("gh down")}}.Adopt(ctx, rec, 4)
	if err == nil {
		t.Fatal("Adopt hid the close failure")
	}
	if got := strings.TrimSpace(f.remote.Git("rev-parse", branchRef(rec.Branch))); got != rec.Tip {
		t.Errorf("remote branch = %s, want the recorded tip %s", got, rec.Tip)
	}
	if got := strings.TrimSpace(f.g.Git("rev-parse", branchRef(rec.Branch))); got != rec.Tip {
		t.Errorf("local branch = %s, want the recorded tip %s", got, rec.Tip)
	}
}

func TestAdopter_ARefusedPushDoesNotMoveTheLocalBranch(t *testing.T) {
	t.Parallel()
	rec := sampleRecord()
	var seen []string
	r := Repo{Git: recordingGit(scriptedGit(rec.Tip, "push"), &seen)}
	spy := &closerSpy{}
	if _, err := (Adopter{Repo: r, Closer: spy}).Adopt(context.Background(), rec, 4); err == nil {
		t.Fatal("Adopt hid the push failure")
	}
	if slices.Contains(seen, "update-ref") {
		t.Errorf("git ran %v: the local branch moved after a refused push", seen)
	}
	if len(spy.closed) != 1 {
		t.Errorf("closed PRs = %v; the close comes before the fence and stays done", spy.closed)
	}
}

func TestAdopter_AFailedLocalMoveAfterThePushLeavesTheRemoteFenced(t *testing.T) {
	t.Parallel()
	rec := sampleRecord()
	var seen []string
	r := Repo{Git: recordingGit(scriptedGit(rec.Tip, "update-ref"), &seen)}
	if _, err := (Adopter{Repo: r, Closer: &closerSpy{}}).Adopt(context.Background(), rec, 4); err == nil {
		t.Fatal("Adopt hid the update-ref failure")
	}
	if !slices.Contains(seen, "push") {
		t.Errorf("git ran %v: the fence never reached the remote before the local move", seen)
	}
}

func TestAdopter_ARemoteBranchThatTheListingDoesNotNameCountsAsUnpublished(t *testing.T) {
	t.Parallel()
	rec := sampleRecord()
	r := Repo{Git: func(_ context.Context, _, _ string, args ...string) (string, error) {
		if args[0] == "ls-remote" {
			return rec.Tip + "\trefs/heads/integration/w135-b1-other\nnoise\n", nil
		}
		return scriptedGit(rec.Tip, "")(context.Background(), "", "", args...)
	}}
	remote, err := r.verifyTips(context.Background(), rec)
	if err != nil || remote != "" {
		t.Errorf("verifyTips = %q, %v; want an empty remote tip", remote, err)
	}
}

func TestIntegrationRecord_BatchIsTheProjectionThatEveryVerbUses(t *testing.T) {
	t.Parallel()
	rec := sampleRecord()
	want := Batch{Wave: "135", Index: 1, Base: rec.Base, Branch: rec.Branch, Tip: rec.Tip}
	if rec.Batch() != want {
		t.Errorf("Batch() = %+v, want %+v", rec.Batch(), want)
	}
	if IntegrationRecordPath("/r", "135") != WaveDir("/r", "135")+".json" {
		t.Errorf("record path %q does not derive from WaveDir", IntegrationRecordPath("/r", "135"))
	}
}

func TestStartBatch_RefusesTheZeroBase(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	if _, err := f.repo.StartBatch(context.Background(), Batch{Wave: "135", Index: 1, Base: strings.Repeat("0", 40)}); err == nil {
		t.Error("StartBatch cut a batch at the zero object id")
	}
}

func TestStartBatchFromOrigin_CutsEachBatchOnItsOwnBranchFromOriginMain(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	f.g.Git("checkout", "-q", "main")
	f.g.Git("commit", "-q", "--allow-empty", "-m", "local only")
	ctx := context.Background()

	b1, err := f.repo.StartBatchFromOrigin(ctx, "135", 1)
	if err != nil {
		t.Fatalf("StartBatchFromOrigin: %v", err)
	}
	b2, err := f.repo.StartBatchFromOrigin(ctx, "135", 2)
	if err != nil {
		t.Fatalf("StartBatchFromOrigin: %v", err)
	}
	if b1.Branch != "integration/w135-b1" || b2.Branch != "integration/w135-b2" {
		t.Errorf("branches = %q, %q", b1.Branch, b2.Branch)
	}
	if b1.Base != f.base || b1.Tip != f.base {
		t.Errorf("base %s tip %s, want origin/main %s and not the local-only commit", b1.Base, b1.Tip, f.base)
	}
	if got := strings.TrimSpace(f.g.Git("rev-parse", branchRef(b2.Branch))); got != f.base {
		t.Errorf("branch ref = %s, want %s", got, f.base)
	}
}

func TestStartBatchFromOrigin_FailsWithoutOriginMainAndForABadName(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	if _, err := f.repo.StartBatchFromOrigin(context.Background(), "135", 1); err == nil {
		t.Error("a repository with no origin/main cut a batch")
	}
	o := newOriginFixture(t)
	if _, err := o.repo.StartBatchFromOrigin(context.Background(), "../x", 1); err == nil {
		t.Error("a bad wave name cut a batch")
	}
}

func TestDeleteBatch_DeletesTheBranchOnlyAtTheBatchTip(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	b, err := f.repo.StartBatchFromOrigin(ctx, "135", 1)
	if err != nil {
		t.Fatal(err)
	}
	stale := b
	stale.Tip = f.commitOn(t, "scratch", "z.txt")
	if err := f.repo.DeleteBatch(ctx, stale); err == nil {
		t.Fatal("DeleteBatch removed a branch whose tip is not the batch tip")
	}
	if err := f.repo.DeleteBatch(ctx, b); err != nil {
		t.Fatalf("DeleteBatch: %v", err)
	}
	if _, err := f.repo.Git(ctx, f.repo.Dir, "", "rev-parse", "--verify", "--quiet", branchRef(b.Branch)); err == nil {
		t.Error("the batch branch still exists after the delete")
	}
}

func TestDeleteBatch_RefusesABatchThatHasNoName(t *testing.T) {
	t.Parallel()
	if err := (Repo{Git: SysGit()}).DeleteBatch(context.Background(), Batch{}); err == nil {
		t.Fatal("DeleteBatch accepted an unnamed batch")
	}
}

func TestWriteIntegrationRecord_ReportsAnEncodeAndAStagingFailure(t *testing.T) {
	boom := errors.New("seam failure")
	origMarshal, origCreate := marshalRecord, createRecordTemp
	t.Cleanup(func() { marshalRecord, createRecordTemp = origMarshal, origCreate })

	marshalRecord = func(any, string, string) ([]byte, error) { return nil, boom }
	if _, err := WriteIntegrationRecord(t.TempDir(), sampleRecord()); !errors.Is(err, boom) {
		t.Errorf("encode failure = %v, want the seam error", err)
	}

	marshalRecord = origMarshal
	pattern := ""
	createRecordTemp = func(_, p string) (*os.File, error) { pattern = p; return nil, boom }
	if _, err := WriteIntegrationRecord(t.TempDir(), sampleRecord()); !errors.Is(err, boom) {
		t.Errorf("staging failure = %v, want the seam error", err)
	}
	if !strings.HasPrefix(pattern, integrationRecordPrefix) {
		t.Errorf("staging pattern %q does not use %q", pattern, integrationRecordPrefix)
	}
}

func gitRaising(err error) GitFunc {
	return func(context.Context, string, string, ...string) (string, error) { return "", err }
}

func TestIntegrationRecord_RefusesABranchThatIsNotTheBranchOfItsWaveAndIndex(t *testing.T) {
	t.Parallel()
	for _, branch := range []string{"integration/w999-b1", "integration/w135-b1x", "integration/w135-b0", "integration/w135-b2", "main"} {
		r := sampleRecord()
		r.Branch = branch
		if err := r.Validate(); err == nil {
			t.Errorf("Validate accepted the branch %q for wave 135 batch 1", branch)
		}
	}
}

func TestDeleteBatch_RefusesTheZeroObjectID(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	b, err := f.repo.StartBatchFromOrigin(ctx, "135", 1)
	if err != nil {
		t.Fatal(err)
	}
	b.Tip = strings.Repeat("0", 40)
	if err := f.repo.DeleteBatch(ctx, b); err == nil {
		t.Fatal("DeleteBatch accepted the zero object id, which git reads as an unconditional delete")
	}
	if _, err := f.repo.Git(ctx, f.repo.Dir, "", "rev-parse", "--verify", "--quiet", branchRef(b.Branch)); err != nil {
		t.Errorf("the batch branch is gone: %v", err)
	}
}

func TestAtTip_AGitFailureIsNotAMovedBranch(t *testing.T) {
	t.Parallel()
	boom := errors.New("deadline")
	err := (Repo{Git: gitRaising(boom)}).atTip(context.Background(), sampleRecord())
	if errors.Is(err, ErrBranchMoved) || !errors.Is(err, boom) {
		t.Errorf("atTip = %v, want the git failure and not ErrBranchMoved", err)
	}
	crash := &GitError{Code: 128}
	if err := (Repo{Git: gitRaising(crash)}).atTip(context.Background(), sampleRecord()); errors.Is(err, ErrBranchMoved) {
		t.Errorf("a git exit of 128 read as a moved branch: %v", err)
	}
	failedStart := &GitError{Code: 1, Err: errors.New("start failed")}
	if err := (Repo{Git: gitRaising(failedStart)}).atTip(context.Background(), sampleRecord()); errors.Is(err, ErrBranchMoved) {
		t.Errorf("a runner failure with exit 1 read as a moved branch: %v", err)
	}
	miss := &GitError{Code: 1}
	if err := (Repo{Git: gitRaising(miss)}).atTip(context.Background(), sampleRecord()); !errors.Is(err, ErrBranchMoved) {
		t.Errorf("a missing branch = %v, want ErrBranchMoved", err)
	}
}

func TestAdopter_ANilCloserFailsBeforeAnySideEffect(t *testing.T) {
	t.Parallel()
	calls := 0
	r := Repo{Git: func(context.Context, string, string, ...string) (string, error) {
		calls++
		return strings.Repeat("a", 40), nil
	}}
	if _, err := (Adopter{Repo: r}).Adopt(context.Background(), sampleRecord(), 4); err == nil {
		t.Fatal("Adopt with a PR and no closer must fail")
	}
	if calls != 0 {
		t.Errorf("git ran %d times before the closer was checked", calls)
	}
}

func TestAdopter_ARecordWithoutAPRNeedsNoCloser(t *testing.T) {
	t.Parallel()
	rec := sampleRecord()
	rec.PR = 0
	r := Repo{Git: scriptedGit(rec.Tip, "")}
	if _, err := (Adopter{Repo: r}).Adopt(context.Background(), rec, 4); err != nil {
		t.Errorf("Adopt demanded a closer for a record with no PR: %v", err)
	}
}

func TestReadIntegrationRecord_NamesTheSchemaItRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "integration-135.json")
	body := `{"schema":"other/1","wave":"135","branch":"integration/w135-b1","tip":"` + strings.Repeat("a", 40) + `","epoch":3}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIntegrationRecord(path); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Errorf("ReadIntegrationRecord = %v, want an error that names the schema", err)
	}
}

func TestReadIntegrationRecord_RefusesARecordInTheFileOfAnotherWave(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path, err := WriteIntegrationRecord(root, sampleRecord())
	if err != nil {
		t.Fatal(err)
	}
	other := IntegrationRecordPath(root, "777")
	if err := os.Rename(path, other); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIntegrationRecord(other); err == nil {
		t.Error("ReadIntegrationRecord read the record of wave 135 from the file of wave 777")
	}
}

func TestWriteIntegrationRecord_RefusesToRollTheEpochBack(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	high := sampleRecord()
	high.Epoch = 5
	if _, err := WriteIntegrationRecord(root, high); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteIntegrationRecord(root, sampleRecord()); !errors.Is(err, ErrStaleEpoch) {
		t.Fatalf("a write of epoch 3 over epoch 5 = %v, want ErrStaleEpoch", err)
	}
	got, err := ReadIntegrationRecord(IntegrationRecordPath(root, "135"))
	if err != nil || got.Epoch != 5 {
		t.Errorf("record after the refused write = %+v, %v; want epoch 5", got, err)
	}
	same := high
	same.PR = 0
	if _, err := WriteIntegrationRecord(root, same); err != nil {
		t.Errorf("a write at the same epoch failed: %v", err)
	}
}

func TestAdopter_ABranchThatMovedOnTheRemoteIsNotAdopted(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	tip := f.commitOn(t, "integration/w135-b1", "a.txt")
	f.g.Git("push", "-q", "origin", tip+":refs/heads/integration/w135-b1")
	f.g.Git("checkout", "-q", "-B", "elsewhere", tip)
	f.g.Git("commit", "-q", "--allow-empty", "-m", "moved")
	f.g.Git("push", "-q", "origin", headOf(f.g)+":refs/heads/integration/w135-b1")
	rec := sampleRecord()
	rec.Tip = tip
	spy := &closerSpy{}
	if _, err := (Adopter{Repo: f.repo, Closer: spy}).Adopt(context.Background(), rec, 4); !errors.Is(err, ErrBranchMoved) {
		t.Fatalf("Adopt error = %v, want ErrBranchMoved", err)
	}
	if len(spy.closed) != 0 {
		t.Errorf("a branch that moved on the remote still closed %v", spy.closed)
	}
}

func TestJourney_ANewLeaseHolderTakesAnUnpublishedBatchAndTheOldHolderIsShutOut(t *testing.T) {
	t.Parallel()
	f := newOriginFixture(t)
	ctx := context.Background()
	root := t.TempDir()
	_, rec := f.cutBatch(t)
	rec.Epoch, rec.PR = 3, 0
	path, err := WriteIntegrationRecord(root, rec)
	if err != nil {
		t.Fatal(err)
	}
	read, err := ReadIntegrationRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	adopted, err := Adopter{Repo: f.repo}.Adopt(ctx, read, 4)
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if _, err := WriteIntegrationRecord(root, adopted); err != nil {
		t.Fatalf("the new holder cannot write its record: %v", err)
	}
	if err := f.repo.PublishBranch(ctx, read); err == nil {
		t.Error("the old holder published the batch after the adoption")
	}
	if _, err := WriteIntegrationRecord(root, read); !errors.Is(err, ErrStaleEpoch) {
		t.Errorf("the old holder wrote its record: %v", err)
	}
	f.g.Git("checkout", "-q", "scratch")
	f.g.Git("commit", "-q", "--allow-empty", "-m", "work of the new holder")
	if err := f.repo.PushBranch(ctx, adopted, headOf(f.g)); err != nil {
		t.Errorf("the new holder cannot push: %v", err)
	}
}
