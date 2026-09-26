package inboxmover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func readItemFailureCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return doc.FailureCount
}

func TestClaim_ConsoleRouteCheckPrecedesCycleValidation(t *testing.T) {
	repo := makeRepo(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	writeItemAt(t, filepath.Join(inbox, "c.json"), `{"id":"c","route":"console-ops"}`)
	writeItemAt(t, filepath.Join(inbox, "d.json"), `{"id":"d"}`)
	opts := Options{ProjectRoot: repo}
	if _, err := Claim(opts, "c", "x"); !errors.Is(err, ErrConsoleRouted) {
		t.Errorf("console-routed + bad cycle: err = %v, want ErrConsoleRouted (the route check comes first)", err)
	}
	if _, err := Claim(opts, "d", "x"); !errors.Is(err, ErrBadArgs) {
		t.Errorf("dispatchable + bad cycle: err = %v, want ErrBadArgs", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "d.json")); err != nil {
		t.Errorf("a refused claim must leave the item at the root: %v", err)
	}
}

// The locked root fails only the rename: the reset's tmp write lives in quarantine/, which stays writable.
func TestReleaseFromQuarantine_CounterResetPrecedesRename(t *testing.T) {
	repo := makeRepo(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	src := filepath.Join(inbox, "quarantine", "q.json")
	writeItemAt(t, src, `{"id":"q","failure_count":3,"last_failure_reason":"boom"}`)
	lockDir(t, inbox)
	_, err := ReleaseFromQuarantine(Options{ProjectRoot: repo, Stderr: &bytes.Buffer{}}, "q")
	unlockDir(t, inbox)
	if !errors.Is(err, ErrMvFailed) {
		t.Fatalf("the rename into a read-only root must fail with ErrMvFailed (the fault did not happen): err = %v", err)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Fatalf("the item must still be quarantined: %v", statErr)
	}
	if got := readItemFailureCount(t, src); got != 0 {
		t.Errorf("failure_count = %d after a failed release, want 0 (the reset precedes the rename)", got)
	}
}

type recordingAppender struct {
	records  []ledger.LifecycleRecord
	onAppend func()
}

func (r *recordingAppender) AppendLifecycle(_ context.Context, rec ledger.LifecycleRecord) error {
	r.records = append(r.records, rec)
	if r.onAppend != nil {
		r.onAppend()
	}
	return nil
}

func TestPromote_RenameFails_NoOpNil_LedgersPromoteWarnMvFailed(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	mustMkdirAll(t, filepath.Join(repo, ".evolve", "inbox", "processed", "cycle-5", "deadbeef-task-1.json"))
	rec := &recordingAppender{}
	res, err := Promote(Options{ProjectRoot: repo, Ledger: rec, IsLandedFn: func(string) (bool, error) { return true, nil }},
		"task-1", "processed", PromoteOpts{Cycle: "5", CommitSHA: "deadbeef00"})
	if err != nil || !res.NoOp {
		t.Fatalf("res = %+v, err = %v; want (NoOp=true, nil)", res, err)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "promote-warn" || !strings.HasSuffix(rec.records[0].Message, ": mv-failed") || rec.records[0].GitHead != "deadbeef00" {
		t.Errorf("ledger records = %+v; want one promote-warn / mv-failed with the sha", rec.records)
	}
}

func TestPromote_LedgerFromPath_PreservesInboxInboxQuirk(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "r.json", "r")
	dropProcessingFile(t, repo, "5", "p.json", "p")
	rec := &recordingAppender{}
	opts := Options{ProjectRoot: repo, Ledger: rec}
	if _, err := Promote(opts, "r", "rejected", PromoteOpts{Cycle: "5"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Promote(opts, "p", "rejected", PromoteOpts{Cycle: "5"}); err != nil {
		t.Fatal(err)
	}
	if len(rec.records) != 2 {
		t.Fatalf("records = %+v", rec.records)
	}
	if !strings.HasPrefix(rec.records[0].Message, ".evolve/inbox/inbox/r.json → ") {
		t.Errorf("root item From = %q, want the .evolve/inbox/inbox/<base> quirk", rec.records[0].Message)
	}
	if !strings.HasPrefix(rec.records[1].Message, ".evolve/inbox/processing/p.json → ") {
		t.Errorf("processing item From = %q, want the cycle-less .evolve/inbox/processing/<base> quirk", rec.records[1].Message)
	}
}

func TestRecoverOrphans_ClobbersExistingRootCopy(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "dup.json", "dup-original")
	dropProcessingFile(t, repo, "3", "dup.json", "dup-claimed")
	res, err := RecoverOrphans(Options{ProjectRoot: repo, ActiveCycleFn: func() (string, error) { return "99", nil }})
	if err != nil || res.Recovered != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	body, _ := os.ReadFile(filepath.Join(repo, ".evolve", "inbox", "dup.json"))
	if !strings.Contains(string(body), "dup-claimed") {
		t.Errorf("the processing copy must overwrite the root twin (the preserved quirk): %s", body)
	}
}

func TestRecoverOrphans_UnreadableCycleState_RecoversTheLiveDir(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "1", "a.json", "a")
	dropProcessingFile(t, repo, "2", "b.json", "b")
	res, err := RecoverOrphans(Options{ProjectRoot: repo, ActiveCycleFn: func() (string, error) { return "", errors.New("unreadable") }})
	if err != nil || res.Recovered != 2 {
		t.Fatalf("res = %+v, err = %v; want both dirs recovered", res, err)
	}
}

// A directory at the rewrite's tmp path fails the bump while the item's id still resolves.
func TestDrain_BumpFailure_FallsOpenToRelease(t *testing.T) {
	repo := makeRepo(t)
	inbox := filepath.Join(repo, ".evolve", "inbox")
	item := filepath.Join(inbox, "processing", "cycle-4", "poison.json")
	writeItemAt(t, item, `{"id":"poison"}`)
	mustMkdirAll(t, item+".tmp."+strconv.Itoa(os.Getpid()))
	rec := &recordingAppender{}
	res, err := ApplyCycleOutcome(Options{ProjectRoot: repo, Ledger: rec}, CycleOutcome{Cycle: 4, CommittedIDs: []string{"poison"}, Ceiling: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Released) != 1 || len(res.Quarantined) != 0 {
		t.Fatalf("res = %+v; want the item released, not quarantined", res)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, "poison.json")); statErr != nil {
		t.Errorf("the item must be back at the root: %v", statErr)
	}
	for _, r := range rec.records {
		if r.Action == "promote" {
			t.Errorf("a failed bump must not reach the quarantine promote: %+v", r)
		}
	}
}

func TestPromote_BadState_MessageStillOmitsQuarantine_AndQuarantineIsValid(t *testing.T) {
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	var stderr bytes.Buffer
	if _, err := Promote(Options{ProjectRoot: repo, Stderr: &stderr}, "task-1", "bogus", PromoteOpts{}); !errors.Is(err, ErrBadState) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stderr.String(), "must be processed|rejected|retry") {
		t.Errorf("the usage text is preserved verbatim: %q", stderr.String())
	}
	res, err := Promote(Options{ProjectRoot: repo, Stderr: &stderr}, "task-1", "quarantine", PromoteOpts{})
	if err != nil || res.NoOp || !strings.HasSuffix(res.DestPath, filepath.Join("quarantine", "task-1.json")) {
		t.Errorf("quarantine is a valid state: res = %+v, err = %v", res, err)
	}
}

func TestPromote_InfoThenRetireThenLedger_Order(t *testing.T) {
	root := t.TempDir()
	seedRootItem(t, root, "task-r")
	seedRegistryOnly(t, root, "task-r", 1507)
	var stderr bytes.Buffer
	rec := &recordingAppender{onAppend: func() { stderr.WriteString("<ledger>\n") }}
	opts := Options{ProjectRoot: root, Stderr: &stderr, Ledger: rec, IsLandedFn: func(string) (bool, error) { return false, nil }}
	res, err := Promote(opts, "task-r", "processed", PromoteOpts{Cycle: "1507", CommitSHA: "unlanded00"})
	if err != nil || res.NoOp {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	out := stderr.String()
	promoted, retired, ledgered := strings.Index(out, "promoted: "), strings.Index(out, "retire 'task-r': released continuation binding"), strings.Index(out, "<ledger>")
	if promoted < 0 || retired < 0 || ledgered < 0 || !(promoted < retired && retired < ledgered) {
		t.Errorf("order must be INFO → retire → ledger; got:\n%s", out)
	}
	if len(rec.records) != 1 || !strings.HasSuffix(rec.records[0].Message, ": ship-promote-retry-unlanded-sha") {
		t.Errorf("ledger reason: %+v", rec.records)
	}
	raw, _ := os.ReadFile(res.DestPath)
	var doc struct {
		Released []struct {
			Reason string `json:"reason"`
		} `json:"released_continuations"`
	}
	if json.Unmarshal(raw, &doc) != nil || len(doc.Released) != 1 || doc.Released[0].Reason != "ship-promote-retry-unlanded-sha" {
		t.Errorf("the preserved pointer must carry the overridden reason: %s", raw)
	}
	stderr.Reset()
	if res, err := Promote(opts, "ghost", "processed", PromoteOpts{}); err != nil || !res.NoOp || strings.Contains(stderr.String(), "retire ") {
		t.Errorf("a NoOp promote must not retire: res=%+v err=%v stderr=%q", res, err, stderr.String())
	}
	if _, ok, _ := continuation.ReadRegistryEntry(root, "task-r"); ok {
		t.Error("the binding must be released by the promote")
	}
}

func TestReadFailureCount_ProjectRootlessOptionsNeverWriteUnderCwd(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	if err := os.Chdir(scratch); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	inbox := t.TempDir()
	writeItemAt(t, filepath.Join(inbox, "a.json"), `{"id":"a","failure_count":2}`)
	opts := Options{InboxDir: inbox, Stderr: io.Discard}
	if n, ok := ReadFailureCount(opts, "a"); !ok || n != 2 {
		t.Errorf("ReadFailureCount = %d, %v", n, ok)
	}
	if st := ResolveDispatchState(opts, "a"); st.State != StatePending {
		t.Errorf("state = %+v", st)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a read-only probe must create nothing under cwd; found %v", entries)
	}
}
