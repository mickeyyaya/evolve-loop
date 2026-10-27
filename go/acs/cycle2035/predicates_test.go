//go:build acs

package cycle2035

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	redCommit       = "deadbeefcafe0123"
	redCommitShort  = "ci-red-deadbeefcafe"
	floorCycle      = 1279
	bumpLockName    = ".bump.lock"
	lockHeldProbe   = 600 * time.Millisecond
	concurrentMints = 8
)

var floorDefects = []string{"the ledger row lacks evidence", "the manifest row lacks a path"}

type noLedger struct{}

func (noLedger) AppendLifecycle(context.Context, ledger.LifecycleRecord) error { return nil }

func clockAt(second int) func() time.Time {
	return func() time.Time { return time.Date(2026, 10, 20, 9, 0, second, 0, time.UTC) }
}

func failingRun(failingTest string) ciwatch.RunStatus {
	return ciwatch.RunStatus{
		Status:      ciwatch.StatusCompleted,
		Conclusion:  "failure",
		RunURL:      "https://github.com/example/evolve-loop/actions/runs/42",
		FailingTest: failingTest,
		LogExcerpt:  "--- FAIL: FixtureFailingCase",
	}
}

func watchRed(inbox, workspace string, now func() time.Time, run ciwatch.RunStatus) (ciwatch.Options, error) {
	opts := ciwatch.Options{
		SHA:          redCommit,
		Cycle:        2004,
		InboxDir:     inbox,
		WorkspaceDir: workspace,
		Fetch:        func(context.Context, string) (ciwatch.RunStatus, error) { return run, nil },
		Now:          now,
		Sleep:        func(time.Duration) {},
	}
	_, err := ciwatch.Watch(context.Background(), opts)
	return opts, err
}

func floorFailure(t *testing.T, root string) failurelearning.Failure {
	t.Helper()
	ws := t.TempDir()
	return failurelearning.Failure{Cycle: floorCycle, Phase: cyclestate.PhaseAudit, ProjectRoot: root, Workspace: ws}
}

func floorLearned() failurelearning.Learned {
	return failurelearning.Learned{
		Summary: "audit phase exited 1 after 3 attempts",
		Structured: &phasecontract.FailureBlock{
			Class:   "deliverable-rejected",
			Defects: floorDefects,
		},
	}
}

func writeFloor(f failurelearning.Failure) {
	failurelearning.New(clockAt(0), carryover.New()).WriteFloor(f, floorLearned())
}

func floorInbox(root string) string { return filepath.Join(root, ".evolve", "inbox") }

type minted struct {
	name string
	doc  map[string]any
	raw  []byte
}

func inboxItems(t *testing.T, inbox string) []minted {
	t.Helper()
	ents, err := os.ReadDir(inbox)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read inbox %s: %v", inbox, err)
	}
	var out []minted
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(inbox, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s is not a JSON object: %v", e.Name(), err)
		}
		out = append(out, minted{name: e.Name(), doc: doc, raw: raw})
	}
	return out
}

func withID(items []minted, id string) []minted {
	var out []minted
	for _, it := range items {
		if it.doc["id"] == id {
			out = append(out, it)
		}
	}
	return out
}

func requireOne(t *testing.T, inbox, id string) minted {
	t.Helper()
	got := withID(inboxItems(t, inbox), id)
	if len(got) != 1 {
		names := make([]string, 0, len(got))
		for _, g := range got {
			names = append(names, g.name)
		}
		t.Fatalf("RED: %d inbox files carry id %q (%v), want exactly 1", len(got), id, names)
	}
	return got[0]
}

func nonBlankText(doc map[string]any, key string) bool {
	text, ok := doc[key].(string)
	return ok && strings.TrimSpace(text) != ""
}

func nonBlankList(doc map[string]any, key string) bool {
	list, ok := doc[key].([]any)
	if !ok || len(list) == 0 {
		return false
	}
	for _, entry := range list {
		text, ok := entry.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return false
		}
	}
	return true
}

func requireFilingShape(t *testing.T, it minted) {
	t.Helper()
	for _, key := range []string{"id", "title", "kind", "summary", "fix"} {
		if !nonBlankText(it.doc, key) {
			t.Errorf("RED: %s has no non-empty %q — the minter did not write the full filing shape", it.name, key)
		}
	}
	if !nonBlankList(it.doc, "acceptance") {
		t.Errorf("RED: %s has no non-empty acceptance list — the minter did not write the full filing shape", it.name)
	}
}

func mintFloorItems(t *testing.T) (root string, items []minted) {
	t.Helper()
	root = t.TempDir()
	writeFloor(floorFailure(t, root))
	items = inboxItems(t, floorInbox(root))
	if len(items) != len(floorDefects) {
		t.Fatalf("WriteFloor filed %d inbox items, want %d", len(items), len(floorDefects))
	}
	return root, items
}

func mintCIRed(t *testing.T, failingTest string) (inbox string, item minted) {
	t.Helper()
	inbox = filepath.Join(t.TempDir(), "inbox")
	if _, err := watchRed(inbox, "", clockAt(1), failingRun(failingTest)); err != nil {
		t.Fatalf("Watch on a red run: %v", err)
	}
	return inbox, requireOne(t, inbox, redCommitShort)
}

func holdBumpLock(t *testing.T, inbox string) (release func()) {
	t.Helper()
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	release, err := verifylock.AcquireAt(context.Background(), filepath.Join(inbox, bumpLockName), io.Discard)
	if err != nil {
		t.Fatalf("take the inbox bump lock: %v", err)
	}
	return release
}

func baseRedReport(rerunGreen ciwatch.Fact) ciwatch.ClassifyReport {
	return ciwatch.ClassifyReport{
		Target: "pr:7", RunID: 9, RunURL: "https://ci/runs/9", BaseSHA: "fedcba9876543210",
		Failures: []ciwatch.ClassifiedFailure{{
			FailingTest: ciwatch.FailingTest{Package: "example.com/m/p", Test: "FixtureBaseRedCase"},
			Evidence:    ciwatch.Evidence{BaseRed: ciwatch.FactYes, RerunGreen: rerunGreen},
		}},
	}
}

func mintBaseRed(inbox string, tick int) ([]string, error) {
	return ciwatch.FileBaseRedItems(inbox, baseRedReport(ciwatch.FactNo), clockAt(tick)())
}

func mintBaseRedOnce(t *testing.T) (inbox string, item minted) {
	t.Helper()
	inbox = filepath.Join(t.TempDir(), "inbox")
	filed, err := mintBaseRed(inbox, 1)
	if err != nil || len(filed) != 1 {
		t.Fatalf("FileBaseRedItems filed %v, err %v; want exactly one id", filed, err)
	}
	return inbox, requireOne(t, inbox, filed[0])
}

func TestC2035_001_FloorMintedItemsCarrySummaryFixAndAcceptance(t *testing.T) {
	_, items := mintFloorItems(t)
	for _, it := range items {
		requireFilingShape(t, it)
	}
}

func TestC2035_002_CIRedMintedItemCarriesFixAndAcceptance(t *testing.T) {
	for name, failing := range map[string]string{
		"named failing test":   "FixtureFailingCase",
		"unidentified failure": "",
	} {
		t.Run(name, func(t *testing.T) {
			_, item := mintCIRed(t, failing)
			requireFilingShape(t, item)
		})
	}
}

func TestC2035_003_BaseRedMintedItemCarriesFixAndAcceptance(t *testing.T) {
	_, item := mintBaseRedOnce(t)
	requireFilingShape(t, item)
}

func TestC2035_004_EveryMintedItemPassesTheFilingWritersRules(t *testing.T) {
	_, floorItems := mintFloorItems(t)
	_, ciItem := mintCIRed(t, "FixtureFailingCase")
	_, baseItem := mintBaseRedOnce(t)
	all := append(slices.Clone(floorItems), ciItem, baseItem)
	classes := []string{inboxbatch.ClassCorrectness, inboxbatch.ClassStability, inboxbatch.ClassDebuggability}
	for _, it := range all {
		t.Run(fmt.Sprint(it.doc["id"]), func(t *testing.T) {
			mover := lifecycle.New(t.TempDir(), noLedger{}, lifecycle.WithNow(clockAt(5)), lifecycle.WithPriorityClasses(classes))
			if _, err := mover.File(it.raw); err != nil {
				t.Errorf("RED: the filing writer refuses the minted item %s: %v", it.name, err)
			}
		})
	}
}

func TestC2035_005_MintedFilesUseTheFilingWritersNameAndLeaveNoTemporaries(t *testing.T) {
	floorRoot, floorItems := mintFloorItems(t)
	ciInbox, ciItem := mintCIRed(t, "FixtureFailingCase")
	baseInbox, baseItem := mintBaseRedOnce(t)
	cases := []struct {
		inbox string
		items []minted
	}{{floorInbox(floorRoot), floorItems}, {ciInbox, []minted{ciItem}}, {baseInbox, []minted{baseItem}}}
	for _, c := range cases {
		for _, it := range c.items {
			want := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z-` + regexp.QuoteMeta(fmt.Sprint(it.doc["id"])) + `\.json$`)
			if !want.MatchString(it.name) {
				t.Errorf("RED: %s is not named <stamp>-<id>.json as the filing writer names a filed item", it.name)
			}
		}
		ents, err := os.ReadDir(c.inbox)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if !strings.HasSuffix(e.Name(), ".json") && e.Name() != bumpLockName {
				t.Errorf("unexpected leftover %q in %s", e.Name(), c.inbox)
			}
		}
	}
}

func TestC2035_006_CIRedRepeatedIdAcrossClockTicksIsAnIdempotentSkip(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	for _, second := range []int{1, 2, 3} {
		if _, err := watchRed(inbox, "", clockAt(second), failingRun("FixtureFailingCase")); err != nil {
			t.Fatalf("Watch at tick %d: a repeated filed id must be a skip, not a failure: %v", second, err)
		}
	}
	requireOne(t, inbox, redCommitShort)
}

func TestC2035_007_BaseRedRepeatedIdAcrossClockTicksIsAnIdempotentSkip(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	first, err := mintBaseRed(inbox, 1)
	if err != nil || len(first) != 1 {
		t.Fatalf("first mint filed %v, err %v", first, err)
	}
	for _, tick := range []int{2, 3} {
		again, err := mintBaseRed(inbox, tick)
		if err != nil {
			t.Fatalf("mint at tick %d: a repeated filed id must be a skip, not a failure: %v", tick, err)
		}
		if len(again) != 0 {
			t.Errorf("mint at tick %d reported %v as filed; a skipped id is not filed", tick, again)
		}
	}
	requireOne(t, inbox, first[0])
}

func TestC2035_008_BaseRedItemSittingInRetryIsNotFiledAgain(t *testing.T) {
	inbox, item := mintBaseRedOnce(t)
	retry := filepath.Join(inbox, "retry")
	if err := os.MkdirAll(retry, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(inbox, item.name), filepath.Join(retry, item.name)); err != nil {
		t.Fatal(err)
	}

	again, err := mintBaseRed(inbox, 2)

	if err != nil {
		t.Fatalf("a base-red id parked in retry/ must be a skip: %v", err)
	}
	if len(again) != 0 || len(inboxItems(t, inbox)) != 0 {
		t.Errorf("a base-red id parked in retry/ was filed again: reported %v, top level holds %d item(s)", again, len(inboxItems(t, inbox)))
	}
}

func TestC2035_009_FloorItemFiledUnderAnotherStampIsNotFiledTwice(t *testing.T) {
	root, items := mintFloorItems(t)
	inbox := floorInbox(root)
	for _, it := range items {
		moved := "2020-01-01T00-00-00Z-" + fmt.Sprint(it.doc["id"]) + ".json"
		if err := os.Rename(filepath.Join(inbox, it.name), filepath.Join(inbox, moved)); err != nil {
			t.Fatal(err)
		}
	}

	again := floorFailure(t, root)
	writeFloor(again)

	if _, err := os.Stat(filepath.Join(again.Workspace, "retrospective-unqueued.md")); err == nil {
		t.Errorf("RED: a re-mint of an already filed id degraded to an UNQUEUED retrospective — a repeated id must be a skip")
	}
	for _, it := range items {
		requireOne(t, inbox, fmt.Sprint(it.doc["id"]))
	}
}

func runConcurrently(n int, work func(i int)) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			work(i)
		}()
	}
	close(start)
	wg.Wait()
}

func TestC2035_010_ConcurrentCIRedMintsFileExactlyOneItem(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	errs := make([]error, concurrentMints)
	runConcurrently(concurrentMints, func(i int) {
		_, errs[i] = watchRed(inbox, "", clockAt(i), failingRun("FixtureFailingCase"))
	})
	for i, err := range errs {
		if err != nil {
			t.Errorf("mint %d failed instead of skipping a filed id: %v", i, err)
		}
	}
	requireOne(t, inbox, redCommitShort)
}

func TestC2035_011_ConcurrentBaseRedMintsFileExactlyOneItem(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	errs := make([]error, concurrentMints)
	filedCounts := make([]int, concurrentMints)
	runConcurrently(concurrentMints, func(i int) {
		var filed []string
		filed, errs[i] = mintBaseRed(inbox, i)
		filedCounts[i] = len(filed)
	})
	for i, err := range errs {
		if err != nil {
			t.Errorf("mint %d failed instead of skipping a filed id: %v", i, err)
		}
	}
	items := inboxItems(t, inbox)
	if len(items) != 1 {
		t.Fatalf("RED: %d inbox files after %d concurrent mints of one id, want 1", len(items), concurrentMints)
	}
	if total := sumInts(filedCounts); total != 1 {
		t.Errorf("RED: %d mints reported filing the id, want exactly 1", total)
	}
}

func sumInts(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

func TestC2035_012_ConcurrentFloorMintsFileEachItemExactlyOnce(t *testing.T) {
	root := t.TempDir()
	failures := make([]failurelearning.Failure, concurrentMints)
	for i := range failures {
		failures[i] = floorFailure(t, root)
	}
	runConcurrently(concurrentMints, func(i int) { writeFloor(failures[i]) })
	items := inboxItems(t, floorInbox(root))
	if len(items) != len(floorDefects) {
		t.Fatalf("RED: %d inbox files after %d concurrent mints of %d defects, want %d", len(items), concurrentMints, len(floorDefects), len(floorDefects))
	}
	for _, it := range items {
		requireOne(t, floorInbox(root), fmt.Sprint(it.doc["id"]))
	}
}

func assertMintWaitsForLock(t *testing.T, inbox string, mint func()) {
	t.Helper()
	release := holdBumpLock(t, inbox)
	done := make(chan struct{})
	go func() {
		defer close(done)
		mint()
	}()
	time.Sleep(lockHeldProbe)
	if during := inboxItems(t, inbox); len(during) != 0 {
		t.Errorf("RED: the minter filed %d item(s) while another writer held %s", len(during), bumpLockName)
	}
	release()
	<-done
}

func TestC2035_013_CIRedMintWaitsForTheInboxBumpLock(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	var mintErr error
	assertMintWaitsForLock(t, inbox, func() { _, mintErr = watchRed(inbox, "", clockAt(1), failingRun("FixtureFailingCase")) })
	if mintErr != nil {
		t.Fatalf("Watch after the lock was released: %v", mintErr)
	}
	requireOne(t, inbox, redCommitShort)
}

func TestC2035_014_BaseRedMintWaitsForTheInboxBumpLock(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	var filed []string
	var mintErr error
	assertMintWaitsForLock(t, inbox, func() { filed, mintErr = mintBaseRed(inbox, 1) })
	if mintErr != nil || len(filed) != 1 {
		t.Fatalf("mint after the lock was released filed %v, err %v", filed, mintErr)
	}
	requireOne(t, inbox, filed[0])
}

func TestC2035_015_FloorMintWaitsForTheInboxBumpLock(t *testing.T) {
	root := t.TempDir()
	inbox := floorInbox(root)
	f := floorFailure(t, root)
	assertMintWaitsForLock(t, inbox, func() { writeFloor(f) })
	if after := inboxItems(t, inbox); len(after) != len(floorDefects) {
		t.Fatalf("after the lock was released the floor filed %d item(s), want %d", len(after), len(floorDefects))
	}
}

func occupiedPath(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestC2035_016_CIRedDegradedPathIsUnchangedWhenTheInboxIsUnwritable(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "inbox")
	occupiedPath(t, notADir)
	workspace := t.TempDir()

	_, err := watchRed(notADir, workspace, clockAt(1), failingRun("FixtureFailingCase"))

	if err == nil || !strings.Contains(err.Error(), "ciwatch:") {
		t.Fatalf("a writer fault must still surface as a ciwatch-prefixed error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(workspace, "ci-watch-verdict.json")); statErr != nil {
		entries, _ := os.ReadDir(workspace)
		t.Errorf("the CI verdict must be written before the filing is attempted; workspace holds %v", entries)
	}
}

func TestC2035_017_BaseRedDegradedPathIsUnchangedWhenTheInboxIsUnwritable(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "inbox")
	occupiedPath(t, notADir)

	filed, err := mintBaseRed(notADir, 1)

	if err == nil {
		t.Fatalf("a writer fault must surface as an error, filed %v", filed)
	}
	if len(filed) != 0 {
		t.Errorf("nothing reached the inbox yet %v was reported filed", filed)
	}
}

func TestC2035_018_FloorDegradedPathIsUnchangedWhenTheInboxIsUnwritable(t *testing.T) {
	root := t.TempDir()
	occupiedPath(t, floorInbox(root))
	f := floorFailure(t, root)

	writeFloor(f)

	if _, err := os.Stat(filepath.Join(f.Workspace, "retrospective-report.md")); err == nil {
		t.Errorf("retrospective-report.md was written while the remediation items reached no queue")
	}
	raw, err := os.ReadFile(filepath.Join(f.Workspace, "retrospective-unqueued.md"))
	if err != nil {
		t.Fatalf("a writer fault must leave the UNQUEUED retrospective: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "UNQUEUED") {
		t.Errorf("retrospective-unqueued.md lacks the UNQUEUED marker:\n%s", body)
	}
	for _, defect := range floorDefects {
		if !strings.Contains(body, strings.Split(defect, " ")[1]) {
			t.Errorf("retrospective-unqueued.md does not carry the unqueued defect %q:\n%s", defect, body)
		}
	}
}

func TestC2035_019_FloorRefusedItemKeepsTheDiagnosisAndFilesNothing(t *testing.T) {
	root := t.TempDir()
	f := floorFailure(t, root)
	learned := floorLearned()
	learned.Structured.Defects = []string{"   "}

	failurelearning.New(clockAt(0), carryover.New()).WriteFloor(f, learned)

	if got := inboxItems(t, floorInbox(root)); len(got) != 0 {
		t.Errorf("a blank defect filed %d inbox item(s), want none", len(got))
	}
	if _, err := os.Stat(filepath.Join(f.Workspace, "retrospective-report.md")); err != nil {
		t.Errorf("the retrospective is always written when no item is minted: %v", err)
	}
}

func TestC2035_020_CIRedFilesNothingForGreenRunsOrWhenEscalationIsOff(t *testing.T) {
	for name, tc := range map[string]struct {
		conclusion   string
		noEscalation bool
	}{
		"green run":      {conclusion: ciwatch.ConclusionSuccess},
		"escalation off": {conclusion: "failure", noEscalation: true},
	} {
		t.Run(name, func(t *testing.T) {
			inbox := filepath.Join(t.TempDir(), "inbox")
			run := failingRun("FixtureFailingCase")
			run.Conclusion = tc.conclusion
			_, err := ciwatch.Watch(context.Background(), ciwatch.Options{
				SHA: redCommit, Cycle: 2035, InboxDir: inbox, NoEscalation: tc.noEscalation,
				Fetch: func(context.Context, string) (ciwatch.RunStatus, error) { return run, nil },
				Now:   clockAt(1), Sleep: func(time.Duration) {},
			})
			if err != nil {
				t.Fatalf("Watch: %v", err)
			}
			if got := inboxItems(t, inbox); len(got) != 0 {
				t.Errorf("filed %d item(s), want none", len(got))
			}
		})
	}
}

func TestC2035_021_BaseRedFilesNothingWhenTheRerunTurnedGreen(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")

	filed, err := ciwatch.FileBaseRedItems(inbox, baseRedReport(ciwatch.FactYes), clockAt(1)())

	if err != nil || len(filed) != 0 {
		t.Fatalf("a flaky failure filed %v, err %v; want nothing", filed, err)
	}
	if got := inboxItems(t, inbox); len(got) != 0 {
		t.Errorf("filed %d item(s), want none", len(got))
	}
}

func TestC2035_022_TheFilingWriterPackageIsEnrolledInTheApicoverGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforce, err := os.ReadFile(filepath.Join(root, "go", ".apicover-enforce"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(strings.Split(string(enforce), "\n"), "./internal/inboxfile") {
		t.Errorf("RED: ./internal/inboxfile is not listed in go/.apicover-enforce")
	}
	if _, err := os.Stat(filepath.Join(root, "go", "internal", "inboxfile", "apicover_named_test.go")); err != nil {
		t.Errorf("RED: ./internal/inboxfile has no apicover_named_test.go naming its exported symbols")
	}
}

func TestC2035_023_TheInboxWritersContractEnrollsTheFilingWriterAndHolds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	contract := filepath.Join(root, "go", "cmd", "evolve", "inbox_writers_contract_test.go")
	if !acsassert.FileContains(t, contract, "internal/inboxfile") {
		t.Errorf("RED: the inbox writers contract does not enroll internal/inboxfile")
	}
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./cmd/evolve",
		"-run", "^TestInboxWriters_EveryAutofiledItemCarriesAKnownClass$")
	if err != nil || code != 0 {
		t.Errorf("RED: the inbox writers contract fails (exit %d): %v\n%s%s", code, err, out, errOut)
	}
}

func TestC2035_024_CoreRetroRemediationTestsHoldWithTheFilingWritersInboxLayout(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "./internal/core",
		"-run", "^(TestWriteDeterministicLearning_|TestRepro1285_)")
	if err != nil || code != 0 {
		t.Errorf("RED: the core retrospective remediation tests fail (exit %d): %v\n%s%s", code, err, out, errOut)
	}
	if strings.Contains(out, "no tests to run") {
		t.Errorf("RED: the run matched no core remediation test, so it proves nothing:\n%s", out)
	}
}

func TestC2035_025_TheWaveProcessStartTimeTestIsUntouchedByTheLane(t *testing.T) {
	const baseSHA = "8dbfc8d92a174d203bf9b39de8b6e39eb626806e"
	const outOfScopeTest = "go/cmd/evolve/cmd_wave_pid_integration_test.go"
	root := acsassert.RepoRoot(t)
	if _, errOut, code, err := acsassert.SubprocessOutput("git", "-C", root, "cat-file", "-e", baseSHA+"^{commit}"); err != nil || code != 0 {
		t.Fatalf("base commit %s is not in the object store (exit %d): %v %s", baseSHA, code, err, errOut)
	}
	diff, errOut, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--stat", baseSHA, "--", outOfScopeTest)
	if err != nil || code != 0 {
		t.Fatalf("git diff failed (exit %d): %v %s", code, err, errOut)
	}
	if strings.TrimSpace(diff) != "" {
		t.Errorf("RED: %s differs from the base the lane started at; the ps sandbox skip is out of scope for the inbox minting lane and belongs in its own item:\n%s", outOfScopeTest, diff)
	}
}
