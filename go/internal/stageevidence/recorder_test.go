package stageevidence_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	keyA        = "workflow.retry_ladder.stage"
	keyB        = "held_proposals.stage"
	versionOfV1 = "v1-digest"
)

type faultLog struct {
	mu   sync.Mutex
	errs []error
}

func (f *faultLog) add(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, err)
}

func (f *faultLog) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.errs)
}

type rig struct {
	evolveDir string
	scope     stageevidence.ScopeKey
	faults    *faultLog
	mu        sync.Mutex
	versions  map[string]int
}

func newRig(t *testing.T, scope stageevidence.ScopeKey) *rig {
	t.Helper()
	return &rig{evolveDir: t.TempDir(), scope: scope, faults: &faultLog{}, versions: map[string]int{}}
}

func (r *rig) recorder(session string, lock func(string) (func(), error)) *stageevidence.Recorder {
	if lock == nil {
		lock = flock.Lock
	}
	return stageevidence.NewRecorder(stageevidence.RecorderConfig{
		EvolveDir: r.evolveDir,
		Scope:     r.scope,
		Session:   session,
		Build:     "rev-abc",
		Lock:      lock,
		Version: func(key string) (string, error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.versions[key]++
			return versionOfV1, nil
		},
		OnFault: r.faults.add,
	})
}

func (r *rig) working(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(stageevidence.WorkingPath(r.evolveDir, r.scope))
	if err != nil {
		t.Fatalf("read working file: %v", err)
	}
	return raw
}

func lines(raw []byte) [][]byte {
	parts := bytes.Split(bytes.TrimSuffix(raw, []byte("\n")), []byte("\n"))
	return parts
}

func join(parts [][]byte) []byte {
	return append(bytes.Join(parts, []byte("\n")), '\n')
}

func sealedRig(t *testing.T) (*rig, []byte) {
	t.Helper()
	r := newRig(t, stageevidence.CycleScope(7))
	rec := r.recorder("s1", nil)
	rec.Record(keyA, map[string]string{"would_stop": "false"})
	rec.Record(keyB, map[string]string{"would_hold": "true"})
	rec.Record(keyA, map[string]string{"would_stop": "true"})
	rec.Close()
	return r, r.working(t)
}

func TestRecorderWritesOpenDecisionsAndCloseInOneVerifiedChain(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	seal, records, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{Shipped: true, Exit: "normal"})
	if err != nil {
		t.Fatalf("BuildSeal: %v", err)
	}
	if seal.Incomplete || seal.Count != 5 || seal.Counters[keyA] != 2 || seal.Counters[keyB] != 1 || !seal.Shipped {
		t.Fatalf("seal = %+v, want 5 records, counters A=2 B=1, complete, shipped", seal)
	}
	kinds := []stageevidence.Kind{stageevidence.KindOpen, stageevidence.KindDecision, stageevidence.KindDecision, stageevidence.KindDecision, stageevidence.KindClose}
	for i, rec := range records {
		if rec.Kind != kinds[i] || rec.Seq != i+1 || rec.Session != "s1" || rec.ScopeKey() != r.scope {
			t.Errorf("record %d = kind %s seq %d session %s scope %v", i, rec.Kind, rec.Seq, rec.Session, rec.ScopeKey())
		}
	}
	if seal.Last != records[4].Hash {
		t.Errorf("seal commits to %q, want the hash of the last record %q", seal.Last, records[4].Hash)
	}
}

func TestRecorderStampsVersionOncePerKeyAndBuildOnEachDecision(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	if r.versions[keyA] != 1 || r.versions[keyB] != 1 {
		t.Errorf("version lookups = %v, want one for each key", r.versions)
	}
	_, records, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range records {
		if rec.Kind == stageevidence.KindDecision && (rec.Version != versionOfV1 || rec.Fields["build"] != "rev-abc") {
			t.Errorf("decision %+v lacks the version or the build revision", rec)
		}
	}
}

func TestRecorderDoesNotMutateTheCallersFields(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(1))
	fields := map[string]string{"would_stop": "true"}
	r.recorder("s", nil).Record(keyA, fields)
	if len(fields) != 1 {
		t.Fatalf("caller map was changed: %v", fields)
	}
}

func TestEvidenceEditFailsVerify(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	edited := bytes.Replace(raw, []byte(`"would_stop":"true"`), []byte(`"would_stop":"false"`), 1)
	assertBroken(t, r.scope, edited, 4)
}

func TestEvidenceDeleteFailsVerify(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	parts := lines(raw)
	assertBroken(t, r.scope, join(append(append([][]byte{}, parts[:2]...), parts[3:]...)), 4)
}

func TestEvidenceReorderFailsVerify(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	parts := lines(raw)
	parts[1], parts[2] = parts[2], parts[1]
	assertBroken(t, r.scope, join(parts), 3)
}

func TestDeletedTailFailsAgainstTheSeal(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	seal, _, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	parts := lines(raw)
	_, err = stageevidence.VerifyStored(r.scope, join(parts[:4]), seal)
	var broken *stageevidence.BrokenChainError
	if !errors.As(err, &broken) || broken.Seq != 4 {
		t.Fatalf("VerifyStored of a truncated copy = %v, want a broken chain at seq 4", err)
	}
	if _, err := stageevidence.VerifyStored(r.scope, raw, seal); err != nil {
		t.Fatalf("VerifyStored of the intact copy: %v", err)
	}
}

func assertBroken(t *testing.T, scope stageevidence.ScopeKey, raw []byte, wantSeq int) {
	t.Helper()
	_, _, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{})
	var broken *stageevidence.BrokenChainError
	if !errors.Is(err, stageevidence.ErrBrokenChain) || !errors.As(err, &broken) {
		t.Fatalf("BuildSeal = %v, want a typed broken-chain error", err)
	}
	if broken.Scope != scope || broken.Seq != wantSeq {
		t.Errorf("broken at %v seq %d, want %v seq %d", broken.Scope, broken.Seq, scope, wantSeq)
	}
}

func TestEvidenceWriteFailureMarksCycleIncomplete(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(3))
	calls := 0
	lock := func(path string) (func(), error) {
		calls++
		if calls == 1 {
			return nil, errors.New("lock refused")
		}
		return flock.Lock(path)
	}
	rec := r.recorder("s", lock)
	rec.Record(keyA, map[string]string{"would_stop": "false"})
	rec.Close()
	if r.faults.count() != 1 {
		t.Fatalf("faults = %d, want one loud fault for the failed write", r.faults.count())
	}
	seal, _, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !seal.Incomplete || seal.Cause != stageevidence.CauseWriteFailed || seal.Counters[keyA] != 0 {
		t.Fatalf("seal = %+v, want incomplete write-failed with no decision stored", seal)
	}
}

func TestVersionLookupFailureIsAFaultNotACrash(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(3))
	rec := stageevidence.NewRecorder(stageevidence.RecorderConfig{
		EvolveDir: r.evolveDir, Scope: r.scope, Session: "s", Lock: flock.Lock,
		Version: func(string) (string, error) { return "", errors.New("no source") },
		OnFault: r.faults.add,
	})
	rec.Record(keyA, nil)
	rec.Close()
	seal, _, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{})
	if err != nil || r.faults.count() != 1 || seal.Cause != stageevidence.CauseWriteFailed {
		t.Fatalf("seal=%+v err=%v faults=%d, want one fault and cause write-failed", seal, err, r.faults.count())
	}
}

func craft(t *testing.T, scope stageevidence.ScopeKey, recs ...stageevidence.Record) []byte {
	t.Helper()
	var out []byte
	prev := ""
	for i, rec := range recs {
		rec.Scope, rec.ID, rec.Seq = scope.Kind, scope.ID, i+1
		rec = stageevidence.Link(prev, rec)
		prev = rec.Hash
		line := bytes.TrimSuffix(mustJSON(t, rec), []byte("\n"))
		out = append(append(out, line...), '\n')
	}
	return out
}

func TestDeciderReturnWithoutWriteMarksIncomplete(t *testing.T) {
	t.Parallel()
	scope := stageevidence.CycleScope(9)
	raw := craft(t, scope,
		stageevidence.Record{Session: "s", Kind: stageevidence.KindOpen},
		stageevidence.Record{Session: "s", Key: keyA, Version: versionOfV1, Kind: stageevidence.KindDecision},
		stageevidence.Record{Session: "s", Kind: stageevidence.KindClose, Fields: map[string]string{"counter." + keyA: "2"}},
	)
	seal, _, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{})
	if err != nil || !seal.Incomplete || seal.Cause != stageevidence.CauseGap {
		t.Fatalf("seal=%+v err=%v, want incomplete with cause gap (counter 2, one record)", seal, err)
	}
	over := craft(t, scope,
		stageevidence.Record{Session: "s", Kind: stageevidence.KindOpen},
		stageevidence.Record{Session: "s", Key: keyA, Version: versionOfV1, Kind: stageevidence.KindDecision},
		stageevidence.Record{Session: "s", Kind: stageevidence.KindClose, Fields: map[string]string{"counter." + keyA: "0"}},
	)
	if s, _, _ := stageevidence.BuildSeal(scope, over, stageevidence.SealInput{}); s.Cause != stageevidence.CauseGap {
		t.Fatalf("a record above its counter gave cause %q, want gap", s.Cause)
	}
}

func TestTornTailMarksIncomplete(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	torn := append(append([]byte{}, raw...), []byte(`{"scope":"cycle","id":7,"ke`)...)
	seal, records, err := stageevidence.BuildSeal(r.scope, torn, stageevidence.SealInput{})
	if err != nil || len(records) != 5 || !seal.Incomplete || seal.Cause != stageevidence.CauseTornTail {
		t.Fatalf("seal=%+v records=%d err=%v, want the 5 whole records kept and cause torn-tail", seal, len(records), err)
	}
}

func TestRecorderKeepsTheChainAfterATornTail(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(7))
	first := r.recorder("s1", nil)
	first.Record(keyA, nil)
	first.Close()
	path := stageevidence.WorkingPath(r.evolveDir, r.scope)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"scope":"cy`); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	second := r.recorder("s2", nil)
	second.Record(keyA, nil)
	second.Close()
	seal, records, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{})
	if err != nil || seal.Cause != stageevidence.CauseTornTail || records[len(records)-1].Kind != stageevidence.KindClose {
		t.Fatalf("seal=%+v err=%v, want a verified chain across the torn line with cause torn-tail", seal, err)
	}
}

func TestOpenSessionWithoutCloseMarksIncomplete(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(7))
	r.recorder("s1", nil).Record(keyA, nil)
	seal, _, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{})
	if err != nil || !seal.Incomplete || seal.Cause != stageevidence.CauseOpenSession {
		t.Fatalf("seal=%+v err=%v, want incomplete open-session", seal, err)
	}
}

func TestLateDecisionIsRecordedAndNotCounted(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(7))
	rec := r.recorder("s1", nil)
	rec.Record(keyA, map[string]string{"would_stop": "false"})
	rec.Close()
	rec.Record(keyA, map[string]string{"would_stop": "true"})
	seal, records, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{Shipped: true})
	if err != nil || seal.Incomplete || seal.Counters[keyA] != 1 {
		t.Fatalf("seal=%+v err=%v, want complete with one counted decision", seal, err)
	}
	if last := records[len(records)-1]; last.Kind != stageevidence.KindLate {
		t.Fatalf("last record kind %s, want late", last.Kind)
	}
	chain := stageevidence.PreverifiedChain(records, []stageevidence.Outcome{seal.Outcome(r.scope)}, "")
	res := stageevidence.Score(chain, versionOfV1, gatecontract.Promotion{MinDecisions: 1}, nil)
	if res.Decisions != 1 || len(res.Late) != 1 {
		t.Fatalf("score decisions=%d late=%d, want 1 and 1", res.Decisions, len(res.Late))
	}
}

func TestSecondSealContainsFirstAsPrefix(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	first, _, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	resumed := r.recorder("s2", nil)
	resumed.Record(keyB, nil)
	resumed.Close()
	second, _, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{Prior: &first})
	if err != nil || second.Count != first.Count+3 {
		t.Fatalf("second seal count=%d err=%v, want %d", second.Count, err, first.Count+3)
	}
	wrong := first
	wrong.Last = "0000"
	_, _, err = stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{Prior: &wrong})
	var broken *stageevidence.BrokenChainError
	if !errors.As(err, &broken) || broken.Seq != first.Count {
		t.Fatalf("a prior whose hash is not in the file gave %v, want broken chain at seq %d", err, first.Count)
	}
	shrunk := first
	shrunk.Count = len(lines(r.working(t))) + 1
	if _, _, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{Prior: &shrunk}); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("a prior longer than the file gave %v, want broken chain", err)
	}
	empty := first
	empty.Count = 0
	if _, _, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{Prior: &empty}); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("a prior of count 0 gave %v, want broken chain", err)
	}
}

func TestScopeKeysWithEqualIdDoNotCollide(t *testing.T) {
	t.Parallel()
	cycle := newRig(t, stageevidence.CycleScope(5))
	pool := newRig(t, stageevidence.ScopeKey{Kind: stageevidence.ScopePool, ID: 5})
	for _, rg := range []*rig{cycle, pool} {
		rec := rg.recorder("s", nil)
		rec.Record(keyA, map[string]string{"would_stop": "true"})
		rec.Close()
	}
	cycleSeal, cycleRecs, err := stageevidence.BuildSeal(cycle.scope, cycle.working(t), stageevidence.SealInput{Shipped: true})
	if err != nil {
		t.Fatal(err)
	}
	poolSeal, poolRecs, err := stageevidence.BuildSeal(pool.scope, pool.working(t), stageevidence.SealInput{Shipped: false})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := stageevidence.BuildSeal(pool.scope, cycle.working(t), stageevidence.SealInput{}); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("sealing cycle bytes as a pool scope gave %v, want broken chain", err)
	}
	both := stageevidence.Chain{Records: append(append([]stageevidence.Record{}, cycleRecs...), poolRecs...)}
	if err := stageevidence.VerifyChain(both); err != nil {
		t.Fatalf("two scopes with one id and separate chains: %v", err)
	}
	outcomes := []stageevidence.Outcome{cycleSeal.Outcome(cycle.scope), poolSeal.Outcome(pool.scope)}
	outcomes[1].Incomplete = true
	res := stageevidence.Score(stageevidence.PreverifiedChain(both.Records, outcomes, ""), versionOfV1, gatecontract.Promotion{MinDecisions: 1}, nil)
	if res.Decisions != 1 || res.FalseStops != 1 {
		t.Fatalf("decisions=%d false stops=%d: the incomplete pool scope must not hide or alter the cycle scope", res.Decisions, res.FalseStops)
	}
	if cycle.scope.String() != "cycle-5" || pool.scope.String() != "pool-5" {
		t.Fatalf("scope labels %q and %q", cycle.scope, pool.scope)
	}
}

func TestTwoSessionsShareOneCycleChainUnderTheFileLock(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(11))
	const each = 20
	var wg sync.WaitGroup
	for _, session := range []string{"parent", "ship-child"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := r.recorder(session, nil)
			for i := 0; i < each; i++ {
				rec.Record(keyA, map[string]string{"n": strconv.Itoa(i)})
			}
			rec.Close()
		}()
	}
	wg.Wait()
	seal, records, err := stageevidence.BuildSeal(r.scope, r.working(t), stageevidence.SealInput{})
	if err != nil {
		t.Fatalf("interleaved sessions broke the chain: %v", err)
	}
	if seal.Incomplete || seal.Counters[keyA] != 2*each || len(records) != 2*each+4 {
		t.Fatalf("seal=%+v records=%d, want %d decisions across 2 complete sessions", seal, len(records), 2*each)
	}
}

func TestSealEncodeParseRoundTripAndRefuseOtherVersions(t *testing.T) {
	t.Parallel()
	in := stageevidence.Seal{V: stageevidence.SealVersion, Count: 4, Last: "abc", Incomplete: true, Cause: stageevidence.CauseGap, Exit: "abort", Counters: map[string]int{keyA: 2}}
	msg := in.Encode()
	out, err := stageevidence.ParseSeal(msg)
	if err != nil || out.Count != 4 || out.Cause != stageevidence.CauseGap || out.Counters[keyA] != 2 || out.Exit != "abort" {
		t.Fatalf("round trip = %+v err=%v", out, err)
	}
	if _, err := stageevidence.ParseSeal(`{"v":2,"count":1}`); err == nil {
		t.Fatal("a seal of another version was accepted")
	}
	if _, err := stageevidence.ParseSeal(`not json`); err == nil {
		t.Fatal("a malformed seal was accepted")
	}
}

func TestBuildSealOfAnEmptyWorkingFileIsAnEmptyChain(t *testing.T) {
	t.Parallel()
	if _, _, err := stageevidence.BuildSeal(stageevidence.CycleScope(1), nil, stageevidence.SealInput{}); !errors.Is(err, stageevidence.ErrEmptyChain) {
		t.Fatalf("BuildSeal(nil) = %v, want an empty chain", err)
	}
}

func TestRecorderThatNeverRecordedWritesNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t, stageevidence.CycleScope(2))
	rec := r.recorder("s", nil)
	rec.Close()
	rec.Close()
	if _, err := os.Stat(stageevidence.WorkingPath(r.evolveDir, r.scope)); !os.IsNotExist(err) {
		t.Fatalf("a session with no decision left a working file (stat err %v)", err)
	}
}

func TestChainDigestPrefersTheSealDigest(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	_, records, _ := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	chain := stageevidence.PreverifiedChain(records, nil, "sealed-digest")
	if res := stageevidence.Score(chain, versionOfV1, gatecontract.Promotion{}, nil); res.ChainDigest != "sealed-digest" {
		t.Fatalf("chain digest = %q, want the digest the reader supplied", res.ChainDigest)
	}
	plain := stageevidence.PreverifiedChain(records, nil, "")
	if res := stageevidence.Score(plain, versionOfV1, gatecontract.Promotion{}, nil); res.ChainDigest != records[len(records)-1].Hash {
		t.Fatalf("chain digest = %q, want the last record hash", res.ChainDigest)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestWriteFaultsAreLoudAndNeverPanic(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, r *rig){
		"working dir blocked by a file": func(t *testing.T, r *rig) {
			if err := os.WriteFile(filepath.Join(r.evolveDir, stageevidence.WorkingDirName), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"working path is a directory": func(t *testing.T, r *rig) {
			if err := os.MkdirAll(stageevidence.WorkingPath(r.evolveDir, r.scope), 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"working file is read only": func(t *testing.T, r *rig) {
			path := stageevidence.WorkingPath(r.evolveDir, r.scope)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0o444); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, stageevidence.CycleScope(1))
			setup(t, r)
			rec := r.recorder("s", nil)
			rec.Record(keyA, nil)
			if r.faults.count() != 1 {
				t.Fatalf("faults after a failed record = %d, want 1", r.faults.count())
			}
			rec.Close()
			if r.faults.count() != 2 {
				t.Fatalf("faults after a failed close = %d, want 2", r.faults.count())
			}
		})
	}
}

func TestCausePrecedenceNamesTheStrongestFault(t *testing.T) {
	t.Parallel()
	scope := stageevidence.CycleScope(4)
	open := func(s string) stageevidence.Record {
		return stageevidence.Record{Session: s, Kind: stageevidence.KindOpen}
	}
	closeWith := func(s string, f map[string]string) stageevidence.Record {
		return stageevidence.Record{Session: s, Kind: stageevidence.KindClose, Fields: f}
	}
	gap := map[string]string{"counter." + keyA: "3"}
	faulted := map[string]string{"fault": stageevidence.FaultWriteFailed}
	cases := []struct {
		name string
		recs []stageevidence.Record
		torn bool
		want string
	}{
		{"open session beats gap", []stageevidence.Record{open("a"), closeWith("a", gap), open("b")}, false, stageevidence.CauseOpenSession},
		{"gap beats torn tail", []stageevidence.Record{open("a"), closeWith("a", gap)}, true, stageevidence.CauseGap},
		{"write failed beats open session", []stageevidence.Record{open("a"), closeWith("a", faulted), open("b")}, false, stageevidence.CauseWriteFailed},
		{"write failed beats gap in another session", []stageevidence.Record{open("a"), closeWith("a", gap), open("b"), closeWith("b", faulted)}, false, stageevidence.CauseWriteFailed},
		{"torn tail alone", []stageevidence.Record{open("a"), closeWith("a", nil)}, true, stageevidence.CauseTornTail},
		{"counter for a key with no decision", []stageevidence.Record{open("a"), closeWith("a", map[string]string{"counter." + keyB: "1"})}, false, stageevidence.CauseGap},
		{"unreadable counter", []stageevidence.Record{open("a"), closeWith("a", map[string]string{"counter." + keyB: "many"})}, false, stageevidence.CauseGap},
		{"clean", []stageevidence.Record{open("a"), closeWith("a", nil)}, false, ""},
	}
	for _, c := range cases {
		raw := craft(t, scope, c.recs...)
		if c.torn {
			raw = append(raw, []byte(`{"scope"`)...)
		}
		seal, _, err := stageevidence.BuildSeal(scope, raw, stageevidence.SealInput{})
		if err != nil || seal.Cause != c.want || seal.Incomplete != (c.want != "") {
			t.Errorf("%s: cause %q incomplete %v err %v, want cause %q", c.name, seal.Cause, seal.Incomplete, err, c.want)
		}
	}
}

func TestVerifyStoredRefusesBytesThatDoNotChain(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	seal, _, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	flipped := bytes.Replace(raw, []byte(`"would_stop":"true"`), []byte(`"would_stop":"false"`), 1)
	if _, err := stageevidence.VerifyStored(r.scope, flipped, seal); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("VerifyStored of edited bytes = %v, want a broken chain", err)
	}
	changed := seal
	changed.Cause, changed.Incomplete = stageevidence.CauseGap, true
	if _, err := stageevidence.VerifyStored(r.scope, raw, changed); !errors.Is(err, stageevidence.ErrBrokenChain) {
		t.Fatalf("VerifyStored of a seal that claims a fault the copy lacks = %v, want a broken chain", err)
	}
}

func TestWorkingNameRoundTripsEveryScopeAndRefusesOtherNames(t *testing.T) {
	t.Parallel()
	for _, kind := range []stageevidence.ScopeKind{stageevidence.ScopeCycle, stageevidence.ScopePool, stageevidence.ScopeFleet, stageevidence.ScopeCurator} {
		scope := stageevidence.ScopeKey{Kind: kind, ID: 12}
		name := filepath.Base(stageevidence.WorkingPath("/evolve", scope))
		if got, ok := stageevidence.ParseWorkingName(name); !ok || got != scope {
			t.Errorf("ParseWorkingName(%q) = %v %v, want %v", name, got, ok, scope)
		}
	}
	for _, name := range []string{"cycle-12", "cycle-12.ndjson.lock", "cycle-12.json", "notes.ndjson", ".ndjson"} {
		if _, ok := stageevidence.ParseWorkingName(name); ok {
			t.Errorf("ParseWorkingName(%q) accepted a name that is not a working file", name)
		}
	}
}

func TestSealIsPrefixOfNeedsAPositiveCountThatTheRecordsHold(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	seal, records, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !seal.IsPrefixOf(records) || !seal.IsPrefixOf(append(records, records[0])) {
		t.Fatal("a seal is a prefix of its own records and of a longer history")
	}
	zero, tooLong, wrongHash := seal, seal, seal
	zero.Count = 0
	tooLong.Count = len(records) + 1
	wrongHash.Last = "0000"
	for name, s := range map[string]stageevidence.Seal{"count 0": zero, "count beyond the records": tooLong, "other hash": wrongHash} {
		if s.IsPrefixOf(records) {
			t.Errorf("%s: accepted as a prefix", name)
		}
	}
}

func TestEditOfScopeIdOrSessionOfTheFirstRecordFailsVerify(t *testing.T) {
	t.Parallel()
	r, raw := sealedRig(t)
	_, records, err := stageevidence.BuildSeal(r.scope, raw, stageevidence.SealInput{})
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string]func(stageevidence.Record) stageevidence.Record{
		"scope":   func(rec stageevidence.Record) stageevidence.Record { rec.Scope = stageevidence.ScopePool; return rec },
		"id":      func(rec stageevidence.Record) stageevidence.Record { rec.ID++; return rec },
		"session": func(rec stageevidence.Record) stageevidence.Record { rec.Session = "other"; return rec },
	}
	for name, edit := range edits {
		changed := append([]stageevidence.Record{edit(records[0])}, records[1:]...)
		if err := stageevidence.VerifyChain(stageevidence.Chain{Records: changed[:1]}); !errors.Is(err, stageevidence.ErrBrokenChain) {
			t.Errorf("editing the %s of the first record gave %v, want a broken chain", name, err)
		}
	}
}
