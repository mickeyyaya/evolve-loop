package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerartifacts"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	evKeyA    = "workflow.retry_ladder.stage"
	evKeyB    = "held_proposals.stage"
	evVersion = "v-seal-test"
)

func recordScope(t *testing.T, evolveDir string, scope stageevidence.ScopeKey, session string, closeIt bool, decisions map[string]map[string]string) {
	t.Helper()
	rec := stageevidence.NewRecorder(stageevidence.RecorderConfig{
		EvolveDir: evolveDir, Scope: scope, Session: session, Build: "rev", Lock: flock.Lock,
		Version: func(string) (string, error) { return evVersion, nil },
		OnFault: func(err error) { t.Errorf("recorder fault: %v", err) },
	})
	for key, fields := range decisions {
		rec.Record(key, fields)
	}
	if closeIt {
		rec.Close()
	}
}

func sealedLedger(t *testing.T) (*FileLedger, string) {
	t.Helper()
	dir := t.TempDir()
	l := New(dir)
	recordScope(t, dir, stageevidence.CycleScope(1), "s", true, map[string]map[string]string{evKeyA: {"would_stop": "false"}})
	recordScope(t, dir, stageevidence.CycleScope(2), "s", true, map[string]map[string]string{evKeyA: {"would_stop": "true"}, evKeyB: {"would_hold": "false"}})
	for _, id := range []int{1, 2} {
		if _, err := l.SealStageEvidence(context.Background(), stageevidence.CycleScope(id), stageevidence.SealInput{Shipped: true, Exit: "normal"}); err != nil {
			t.Fatalf("seal cycle %d: %v", id, err)
		}
	}
	return l, dir
}

func readEntries(t *testing.T, dir string) []core.LedgerEntry {
	t.Helper()
	it, err := New(dir).Iter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []core.LedgerEntry
	for {
		e, ok, err := it.Next()
		if err != nil || !ok {
			if err != nil {
				t.Fatal(err)
			}
			return out
		}
		out = append(out, e)
	}
}

func TestSealPinsStoredCopyByDigest(t *testing.T) {
	t.Parallel()
	_, dir := sealedLedger(t)
	entries := readEntries(t, dir)
	if len(entries) != 2 {
		t.Fatalf("ledger has %d entries, want one seal for each of 2 scopes", len(entries))
	}
	e := entries[1]
	working, err := os.ReadFile(stageevidence.WorkingPath(dir, stageevidence.CycleScope(2)))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := ledgerartifacts.Open(dir).Get(e.ArtifactSHA256)
	if err != nil || string(stored) != string(working) {
		t.Fatalf("stored copy differs from the working file (err %v)", err)
	}
	wantPath := filepath.ToSlash(filepath.Join(ledgerartifacts.DirName, "sha256", e.ArtifactSHA256[:2], e.ArtifactSHA256[2:]))
	if e.Kind != StageEvidenceSealKind || e.Role != StageEvidenceRole || e.CycleLabel != "cycle-2" || e.Cycle != 2 || e.ArtifactPath != wantPath {
		t.Fatalf("seal entry = %+v, want kind, role, label cycle-2, cycle 2 and path %s", e, wantPath)
	}
	seal, err := stageevidence.ParseSeal(e.Message)
	if err != nil || seal.Count != 4 || !seal.Shipped || seal.Exit != "normal" || seal.Incomplete {
		t.Fatalf("seal message = %+v err %v", seal, err)
	}
}

func TestPoolScopeSealCarriesNoCycleNumber(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scope := stageevidence.ScopeKey{Kind: stageevidence.ScopePool, ID: 4}
	recordScope(t, dir, scope, "s", true, map[string]map[string]string{evKeyA: nil})
	if _, err := New(dir).SealStageEvidence(context.Background(), scope, stageevidence.SealInput{}); err != nil {
		t.Fatal(err)
	}
	e := readEntries(t, dir)[0]
	if e.CycleLabel != "pool-4" || e.Cycle != 0 {
		t.Fatalf("pool seal entry label %q cycle %d, want pool-4 and no cycle", e.CycleLabel, e.Cycle)
	}
}

func TestSealOfAScopeWithNoWorkingFileIsAnEmptyChain(t *testing.T) {
	t.Parallel()
	if _, err := New(t.TempDir()).SealStageEvidence(context.Background(), stageevidence.CycleScope(3), stageevidence.SealInput{}); !errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Fatalf("seal with no working file = %v, want an empty chain", err)
	}
}

func TestReaderBuildsChainFromSealsAndScoresIt(t *testing.T) {
	t.Parallel()
	l, _ := sealedLedger(t)
	reader := l.StageChain()
	chain, err := reader.ReadChain(context.Background(), evKeyA)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain.Records) != 2 || chain.Records[0].ID != 1 || chain.Records[1].ID != 2 || len(chain.Outcomes) != 2 || chain.Digest == "" {
		t.Fatalf("chain = %+v, want one record from each cycle, two outcomes and a digest", chain)
	}
	res, err := stageevidence.Verify(context.Background(), reader, evKeyA, evVersion, gatecontract.Promotion{MinDecisions: 2, MaxFalseStops: 0}, nil)
	if err != nil || res.Decisions != 2 || res.FalseStops != 1 || res.Ready || res.FirstCycle != 1 || res.LastCycle != 2 {
		t.Fatalf("res=%+v err=%v, want 2 decisions, 1 false stop (cycle 2 shipped), not ready", res, err)
	}
	other, err := reader.ReadChain(context.Background(), evKeyB)
	if err != nil || len(other.Records) != 1 || len(other.Outcomes) != 1 || other.Outcomes[0].ID != 2 {
		t.Fatalf("key B chain = %+v err=%v, want only the record and outcome of cycle 2", other, err)
	}
	if _, err := reader.ReadChain(context.Background(), "no.such.stage"); !errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Fatalf("a key with no record gave %v, want an empty chain", err)
	}
}

func TestChainDigestIsTheHashOverTheSealDigestsInLedgerOrder(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	chain, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, e := range readEntries(t, dir) {
		h.Write([]byte(e.ArtifactSHA256))
	}
	if want := hex.EncodeToString(h.Sum(nil)); chain.Digest != want {
		t.Fatalf("digest %q, want %q", chain.Digest, want)
	}
}

func TestStoredCopyEditIsCorruptStore(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	entries := readEntries(t, dir)
	object := filepath.Join(dir, filepath.FromSlash(entries[1].ArtifactPath))
	if err := os.Chmod(object, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if !errors.Is(err, stageevidence.ErrCorruptStore) || stageevidence.ErrorCode(err) != "corrupt-store" {
		t.Fatalf("edited stored copy gave %v, want corrupt-store", err)
	}
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrCorruptStore) {
		t.Fatalf("a removed stored copy gave %v, want corrupt-store", err)
	}
}

func TestStoredCopyThatDisagreesWithItsSealIsABrokenChain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scope := stageevidence.CycleScope(1)
	recordScope(t, dir, scope, "s", true, map[string]map[string]string{evKeyA: {"would_stop": "false"}})
	l := New(dir)
	seal, err := l.SealStageEvidence(context.Background(), scope, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	lying := seal
	lying.Count++
	message := lying.Encode()
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: scope.String(), ArtifactSHA256: readEntries(t, dir)[0].ArtifactSHA256, Message: message}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("a seal that claims more records than its copy holds gave %v, want a broken chain", err)
	}
}

func TestSealEntryRemovedFailsLedgerVerify(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	path := filepath.Join(dir, "ledger.jsonl")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitAfter(string(raw), "\n")
	if err := os.WriteFile(path, []byte(parts[1]), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = l.StageChain().ReadChain(context.Background(), evKeyA)
	if !errors.Is(err, stageevidence.ErrLedgerBroken) || stageevidence.ErrorCode(err) != "ledger-broken" {
		t.Fatalf("a ledger missing its first seal gave %v, want ledger-broken", err)
	}
}

func TestMalformedSealEntryBreaksTheLedgerReading(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := New(dir)
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: "cycle-1", Message: "not a seal"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("a malformed seal gave %v, want ledger-broken", err)
	}
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: "weird", Message: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("a seal with an unknown scope label gave %v, want ledger-broken", err)
	}
}

func TestUnsealedWorkingFileIsIncomplete(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	recordScope(t, dir, stageevidence.CycleScope(3), "s", false, map[string]map[string]string{evKeyA: {"would_stop": "true"}})
	chain, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil {
		t.Fatal(err)
	}
	var incomplete []int
	for _, o := range chain.Outcomes {
		if o.Incomplete {
			incomplete = append(incomplete, o.ID)
		}
	}
	if len(incomplete) != 1 || incomplete[0] != 3 || len(chain.Records) != 2 {
		t.Fatalf("incomplete=%v records=%d, want cycle 3 incomplete and only the 2 sealed records", incomplete, len(chain.Records))
	}
	res := stageevidence.Score(chain, evVersion, gatecontract.Promotion{MinDecisions: 1, MaxFalseStops: 5}, nil)
	if len(res.IncompleteCycles) != 1 || res.Decisions != 2 {
		t.Fatalf("score incomplete=%v decisions=%d, want the unsealed cycle reported and not counted", res.IncompleteCycles, res.Decisions)
	}
}

func TestOnlyUnsealedEvidenceIsAnEmptyChain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	recordScope(t, dir, stageevidence.CycleScope(3), "s", false, map[string]map[string]string{evKeyA: nil})
	if _, err := New(dir).StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Fatalf("evidence that was never sealed gave %v, want an empty chain", err)
	}
}

func TestWorkingFileEditedAfterTheSealMarksTheScopeIncompleteForEveryKey(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	path := stageevidence.WorkingPath(dir, stageevidence.CycleScope(2))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{}\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{evKeyA, evKeyB} {
		chain, err := l.StageChain().ReadChain(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		got := 0
		for _, o := range chain.Outcomes {
			if o.ID == 2 && o.Incomplete {
				got++
			}
		}
		if got != 1 {
			t.Errorf("key %s: cycle 2 appears incomplete %d times, want once", key, got)
		}
	}
}

func TestRemovedWorkingFileAfterSealKeepsTheSameChain(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	before, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, stageevidence.WorkingDirName)); err != nil {
		t.Fatal(err)
	}
	after, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil || after.Digest != before.Digest || len(after.Outcomes) != 2 {
		t.Fatalf("after cleanup digest %q (was %q) outcomes %d err %v, want the same chain", after.Digest, before.Digest, len(after.Outcomes), err)
	}
}

func TestReaderScansLedgerOnce(t *testing.T) {
	l, dir := sealedLedger(t)
	reads := atomic.Int32{}
	withHooks(ledgerHooks{readF: func(path string) ([]byte, error) {
		if filepath.Base(path) == "ledger.jsonl" {
			reads.Add(1)
		}
		return os.ReadFile(path)
	}}, func() {
		reader := l.StageChain()
		for _, key := range []string{evKeyA, evKeyB, "a.b", "c.d"} {
			_, _ = reader.ReadChain(context.Background(), key)
		}
	})
	if reads.Load() != 1 {
		t.Fatalf("ledger.jsonl was read %d times for 4 keys, want once (%s)", reads.Load(), dir)
	}
}

func TestReaderFindsSealInCompactedSegment(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	for i := 0; i < 3; i++ {
		if err := l.Append(context.Background(), core.LedgerEntry{Role: "scout", Kind: "phase"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Seal(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	segments, err := os.ReadDir(filepath.Join(dir, segmentsDirName))
	if err != nil || len(segments) == 0 {
		t.Fatalf("no segment was written (err %v)", err)
	}
	chain, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil || len(chain.Records) != 2 {
		t.Fatalf("chain after compaction: records=%d err=%v, want both seals found", len(chain.Records), err)
	}
}

func TestSecondSealOfAResumedScopeExtendsTheFirstAndTheReaderUsesIt(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	recordScope(t, dir, stageevidence.CycleScope(2), "resumed", true, map[string]map[string]string{evKeyA: {"would_stop": "false"}})
	second, err := l.SealStageEvidence(context.Background(), stageevidence.CycleScope(2), stageevidence.SealInput{Shipped: true})
	if err != nil || second.Count != 7 {
		t.Fatalf("second seal count %d err %v, want 7 records (4 + open, decision, close)", second.Count, err)
	}
	chain, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil || len(chain.Records) != 3 {
		t.Fatalf("records=%d err=%v, want the cycle 1 record and both cycle 2 decisions", len(chain.Records), err)
	}
}

func TestSecondSealRefusesHistoryThatWasRewritten(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	scope := stageevidence.CycleScope(2)
	forgedDir := t.TempDir()
	recordScope(t, forgedDir, scope, "forged", true, map[string]map[string]string{evKeyA: nil, evKeyB: nil, "x.y": nil, "z.w": nil})
	forged, err := os.ReadFile(stageevidence.WorkingPath(forgedDir, scope))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stageevidence.WorkingPath(dir, scope), forged, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.SealStageEvidence(context.Background(), scope, stageevidence.SealInput{}); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("a longer chain that does not contain the sealed one gave %v, want a broken chain", err)
	}
}

func TestReaderRefusesAHistoryWhoseLaterSealDropsTheEarlierOne(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	scope := stageevidence.CycleScope(2)
	forgedDir := t.TempDir()
	recordScope(t, forgedDir, scope, "forged", true, map[string]map[string]string{evKeyA: {"would_stop": "false"}})
	working, err := os.ReadFile(stageevidence.WorkingPath(forgedDir, scope))
	if err != nil {
		t.Fatal(err)
	}
	seal, _, err := stageevidence.BuildSeal(scope, working, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	message := seal.Encode()
	digest, err := ledgerartifacts.Open(dir).Put(working)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: scope.String(), ArtifactSHA256: digest, Message: message}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("a later seal that does not contain the earlier one gave %v, want a broken chain", err)
	}
}

func TestSealedScopeKeepsOneEntryPerSeal(t *testing.T) {
	t.Parallel()
	_, dir := sealedLedger(t)
	count := 0
	for _, e := range readEntries(t, dir) {
		if e.Kind == StageEvidenceSealKind {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("seal entries = %d, want 2", count)
	}
}

func TestLedgerThatCannotBeReadIsReportedAsBroken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ledger.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir).StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("an unreadable ledger gave %v, want ledger-broken", err)
	}
	scope := stageevidence.CycleScope(1)
	recordScope(t, dir, scope, "s", true, map[string]map[string]string{evKeyA: nil})
	if _, err := New(dir).SealStageEvidence(context.Background(), scope, stageevidence.SealInput{}); err == nil {
		t.Fatal("a seal over an unreadable ledger succeeded")
	}
}

func TestSealOfAnUnreadableWorkingFileFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scope := stageevidence.CycleScope(1)
	if err := os.MkdirAll(stageevidence.WorkingPath(dir, scope), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := New(dir).SealStageEvidence(context.Background(), scope, stageevidence.SealInput{})
	if err == nil || errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Fatalf("a working path that is a directory gave %v, want a read error that is not an empty chain", err)
	}
}

func TestSealWhoseStoreCannotBeWrittenFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scope := stageevidence.CycleScope(1)
	recordScope(t, dir, scope, "s", true, map[string]map[string]string{evKeyA: nil})
	if err := os.WriteFile(filepath.Join(dir, ledgerartifacts.DirName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir).SealStageEvidence(context.Background(), scope, stageevidence.SealInput{}); err == nil {
		t.Fatal("a seal whose evidence store is blocked succeeded")
	}
	for _, e := range readEntries(t, dir) {
		if e.Kind == StageEvidenceSealKind {
			t.Fatal("a seal entry was appended although the stored copy was not written")
		}
	}
}

func TestSealEntryThatCannotBeAppendedFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	scope := stageevidence.CycleScope(1)
	recordScope(t, dir, scope, "s", true, map[string]map[string]string{evKeyA: nil})
	if err := os.Mkdir(filepath.Join(dir, "ledger.lock"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir).SealStageEvidence(context.Background(), scope, stageevidence.SealInput{}); err == nil {
		t.Fatal("a seal that cannot reach the ledger succeeded")
	}
}

func TestSealsInOrderSkipsOtherEntriesThatNameTheKindAndFlagsGarbage(t *testing.T) {
	t.Parallel()
	mention := []byte(`{"kind":"phase","action":"stage-evidence-seal"}`)
	garbage := []byte(`stage-evidence-seal "stage-evidence-seal" {`)
	seals, err := sealsInOrder([][]byte{mention})
	if err != nil || len(seals) != 0 {
		t.Fatalf("an entry that only mentions the kind gave seals=%d err=%v", len(seals), err)
	}
	if _, err := sealsInOrder([][]byte{garbage}); err == nil {
		t.Fatal("a line that names the kind and is not an entry was accepted")
	}
}

func TestWorkingDirThatCannotBeListedFailsTheReader(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	if err := os.RemoveAll(filepath.Join(dir, stageevidence.WorkingDirName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stageevidence.WorkingDirName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); err == nil {
		t.Fatal("a working dir that is a file was ignored")
	}
}

func TestWorkingFileThatCannotBeReadFailsTheReader(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	if err := os.MkdirAll(stageevidence.WorkingPath(dir, stageevidence.CycleScope(8)), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); err == nil {
		t.Fatal("an unreadable working file was ignored")
	}
}

func TestFilesInTheWorkingDirThatAreNotWorkingFilesAreIgnored(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	for _, name := range []string{"cycle-9", "cycle-9.ndjson.bak", "notes.ndjson"} {
		if err := os.WriteFile(filepath.Join(dir, stageevidence.WorkingDirName, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	chain, err := l.StageChain().ReadChain(context.Background(), evKeyA)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range chain.Outcomes {
		if o.Incomplete {
			t.Fatalf("outcome %+v was marked incomplete by a file that is not a working file", o)
		}
	}
}

func TestSealRefusesToStackOnAMalformedEarlierSeal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	l := New(dir)
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: "cycle-1", Message: "not a seal"}); err != nil {
		t.Fatal(err)
	}
	scope := stageevidence.CycleScope(2)
	recordScope(t, dir, scope, "s", true, map[string]map[string]string{evKeyA: nil})
	if _, err := l.SealStageEvidence(context.Background(), scope, stageevidence.SealInput{}); !errors.Is(err, stageevidence.ErrLedgerBroken) {
		t.Fatalf("sealing beside a malformed seal gave %v, want ledger-broken", err)
	}
}

func TestReaderRefusesAnEarlierSealOfCountZeroWithoutPanicking(t *testing.T) {
	t.Parallel()
	l, dir := sealedLedger(t)
	scope := stageevidence.CycleScope(2)
	zero := stageevidence.Seal{V: stageevidence.SealVersion, Count: 0}
	first := readEntries(t, dir)[1]
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: scope.String(), ArtifactSHA256: first.ArtifactSHA256, Message: zero.Encode()}); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(context.Background(), core.LedgerEntry{Role: StageEvidenceRole, Kind: StageEvidenceSealKind, CycleLabel: scope.String(), ArtifactSHA256: first.ArtifactSHA256, Message: first.Message}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.StageChain().ReadChain(context.Background(), evKeyA); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("an earlier seal of count 0 gave %v, want a broken chain", err)
	}
}
