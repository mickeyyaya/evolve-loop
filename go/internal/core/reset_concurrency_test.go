package core_test

// reset_concurrency_test.go lives in the EXTERNAL core_test package (not
// package core) specifically to import adapters/storage — storage imports
// core, so an internal (package core) test file importing storage forms an
// import cycle (see reset_test.go, which stays package core and cannot use
// storage.New directly).

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// noopLedger satisfies the ledgerAppender method set structurally (the
// interface itself is unexported in package core, but Go interface
// satisfaction is structural, so this external type works as an argument).
type noopLedger struct{}

func (noopLedger) Append(context.Context, core.LedgerEntry) error { return nil }

// concurrencySealFixture seeds a minimal .evolve/{cycle-state.json,state.json}
// pair for cycle cycleID, mirroring reset_test.go's unexported sealFixture
// (duplicated here since that helper is unreachable from core_test).
func concurrencySealFixture(t *testing.T, evolveDir string, cycleID int) {
	t.Helper()
	workspace := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycleID))
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	cs := map[string]any{
		"cycle_id":       cycleID,
		"phase":          "scout",
		"active_agent":   "scout",
		"workspace_path": workspace,
	}
	writeJSON(t, filepath.Join(evolveDir, "cycle-state.json"), cs)
	st := map[string]any{
		"lastCycleNumber": cycleID - 1,
		"version":         18,
		"currentBatch":    map[string]any{"cycleAccruedCostUSD": 239.2},
	}
	writeJSON(t, filepath.Join(evolveDir, "state.json"), st)
}

func writeJSON(t *testing.T, path string, body any) {
	t.Helper()
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func intField(m map[string]any, key string) int {
	v, _ := m[key].(float64)
	return int(v)
}

// TestSealCycle_ConcurrentUpdateStateNoLostUpdate is the cycle-616 regression
// for the fable5_deep_scan finding "statefile-rmw-flock-consolidation":
// SealCycle's state.json read-modify-write (reset.go's
// readJSONMapFile/writeJSONMapFileAtomic, step 4) holds NO flock, while
// storage.UpdateState (the documented single-writer contract in
// adapters/storage/updatestate.go) holds "<state.json>.lock" for its whole
// RMW. In fleet mode (2+ concurrent lanes are the live operating mode — see
// fleet_concurrency_respect_architecture memory) SealCycle can race a
// concurrent UpdateState caller and lose one side's write.
//
// The interleaving is forced deterministically via channels (no sleep-based
// race gambling), mirroring the existing
// TestWithPathLock_SerializesConcurrentRMW pattern in
// adapters/flock/withpath_test.go:
//  1. A goroutine starts storage.UpdateState, which locks state.json, reads
//     it (lastCycleNumber=41, version=18), then blocks mid-mutate on a
//     channel — the lock stays held the whole time (release is deferred
//     until mutate returns AND the merged write completes).
//  2. While UpdateState is paused holding the lock, a second goroutine runs
//     SealCycle (which today does its own unlocked read+write of the SAME
//     state.json) and is given a bounded window to run to completion.
//  3. UpdateState is then released to finish its own write.
//
// Without a shared lock, SealCycle's unlocked write completes first and is
// then clobbered by UpdateState's write, because UpdateState's in-memory
// state was read BEFORE SealCycle ran and does not reflect SealCycle's
// lastCycleNumber bump — the classic lost update. Once reset.go's RMW
// acquires the same flock.PathLock(statePath) storage.UpdateState uses,
// SealCycle blocks until UpdateState releases, then reads the
// POST-UpdateState state and its own write lands cleanly on top — both
// writers' changes survive.
func TestSealCycle_ConcurrentUpdateStateNoLostUpdate(t *testing.T) {
	ev := t.TempDir()
	concurrencySealFixture(t, ev, 42) // state.json{lastCycleNumber:41,version:18,...}
	statePath := filepath.Join(ev, "state.json")
	fs := storage.New(ev)

	mutateEntered := make(chan struct{})
	releaseUpdate := make(chan struct{})
	updateErr := make(chan error, 1)
	go func() {
		_, err := fs.UpdateState(context.Background(), func(st *core.State) {
			close(mutateEntered)
			<-releaseUpdate
			st.Version++ // an UNRELATED field SealCycle never touches
		})
		updateErr <- err
	}()

	<-mutateEntered // UpdateState holds state.json.lock, has read stale state, is paused

	sealErr := make(chan error, 1)
	go func() {
		_, err := core.SealCycle(context.Background(), noopLedger{}, core.SealOptions{
			EvolveDir:   ev,
			ProjectRoot: ev,
			Reason:      "concurrency regression test",
			Now:         func() time.Time { return time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC) },
			GitHead:     func(string) (string, error) { return "testhead123", nil },
		})
		sealErr <- err
	}()

	// Bounded window for SealCycle to run its (today unlocked) RMW while
	// UpdateState deliberately holds the lock. If SealCycle is fixed to lock
	// too, it blocks here and the loop just times out harmlessly — no
	// assertion is made until after releaseUpdate below, in either case.
	cycleStatePath := filepath.Join(ev, "cycle-state.json")
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cycleStatePath); errors.Is(err, os.ErrNotExist) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	close(releaseUpdate)
	if err := <-updateErr; err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	if err := <-sealErr; err != nil {
		t.Fatalf("SealCycle: %v", err)
	}

	final := readJSON(t, statePath)
	if got := intField(final, "lastCycleNumber"); got != 42 {
		t.Errorf("lost update: lastCycleNumber=%d want 42 — SealCycle's RMW must hold the same "+
			"flock.PathLock(statePath) storage.UpdateState holds, so a concurrent locked writer's "+
			"stale in-memory read cannot clobber SealCycle's write", got)
	}
	if got := intField(final, "version"); got != 19 {
		t.Errorf("lost update: version=%d want 19 — the concurrent UpdateState write must also survive", got)
	}
}

// --- Test-amplification additions (cycle 616 black-box adversarial pass) ---
//
// The AC-Materialization contract for statefile-rmw-flock-consolidation
// requires "reset.go RMW uses the same flock.PathLock-protected accessor as
// statefile.go; concurrent-writer regression test passes; no duplicate RMW
// implementation remains." The RED test above pins the minimal 2-writer
// interleaving. The tests below amplify that same contract along two
// adversarial axes the RED test does not cover: (1) scaling from a single
// concurrent writer to many (the actual fleet operating mode is 2+ lanes,
// per the fleet_concurrency_respect_architecture memory, and the lock must
// hold under N-way contention, not just N=2), and (2) verifying the lock is
// released promptly on SealCycle's success path — a lock-leak/deadlock class
// of regression the original test never checks for, since it only observes
// state AFTER releasing its own paused writer.

// TestSealCycle_ManyConcurrentUpdateStateWritersNoLostUpdate scales the
// RED test's 2-writer interleaving up to N independent, uncoordinated
// storage.UpdateState callers racing a single SealCycle call. Unlike the RED
// test, these writers are not paused on a channel — they are left to race
// naturally, exercising the shared flock.PathLock sidecar under genuine
// concurrent contention rather than one forced, deterministic interleaving.
// If SealCycle's RMW does not hold the SAME lock, at least one of the N
// increments (or SealCycle's own lastCycleNumber bump) is expected to be
// lost to a lost-update race.
func TestSealCycle_ManyConcurrentUpdateStateWritersNoLostUpdate(t *testing.T) {
	ev := t.TempDir()
	concurrencySealFixture(t, ev, 42) // state.json{lastCycleNumber:41,version:18,...}
	statePath := filepath.Join(ev, "state.json")
	fs := storage.New(ev)

	const writers = 5
	var wg sync.WaitGroup
	errs := make(chan error, writers+1)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := fs.UpdateState(context.Background(), func(st *core.State) {
				st.Version++
			}); err != nil {
				errs <- err
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := core.SealCycle(context.Background(), noopLedger{}, core.SealOptions{
			EvolveDir:   ev,
			ProjectRoot: ev,
			Reason:      "many-writer concurrency regression test",
			Now:         func() time.Time { return time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC) },
			GitHead:     func(string) (string, error) { return "testhead123", nil },
		})
		if err != nil {
			errs <- err
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent writer failed: %v", err)
	}

	final := readJSON(t, statePath)
	if got := intField(final, "lastCycleNumber"); got != 42 {
		t.Errorf("lost update under %d-way concurrency: lastCycleNumber=%d want 42 — SealCycle's write "+
			"was clobbered by a concurrent UpdateState writer", writers, got)
	}
	if got := intField(final, "version"); got != 18+writers {
		t.Errorf("lost update under %d-way concurrency: version=%d want %d — every concurrent "+
			"UpdateState increment must survive alongside SealCycle's own write", writers, got, 18+writers)
	}
}

// TestSealCycle_LockReleasedPromptlyAfterCompletion guards against a
// lock-leak/deadlock class of regression that TestSealCycle_
// ConcurrentUpdateStateNoLostUpdate cannot catch, because that test only
// inspects state.json AFTER its own paused writer has already released the
// lock. A correct fix must release the shared flock.PathLock(statePath) on
// SealCycle's success path; if it instead leaks the lock (e.g. by opening a
// second, never-closed lock handle, or by holding the lock past return), any
// subsequent legitimate writer — like a following cycle's storage.UpdateState
// — would hang indefinitely.
func TestSealCycle_LockReleasedPromptlyAfterCompletion(t *testing.T) {
	ev := t.TempDir()
	concurrencySealFixture(t, ev, 7)
	fs := storage.New(ev)

	if _, err := core.SealCycle(context.Background(), noopLedger{}, core.SealOptions{
		EvolveDir:   ev,
		ProjectRoot: ev,
		Reason:      "lock-release regression test",
		Now:         func() time.Time { return time.Date(2026, 5, 27, 8, 0, 0, 0, time.UTC) },
		GitHead:     func(string) (string, error) { return "testhead123", nil },
	}); err != nil {
		t.Fatalf("SealCycle: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := fs.UpdateState(context.Background(), func(st *core.State) {
			st.Version++
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("post-SealCycle UpdateState: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("UpdateState did not complete within 2s after SealCycle finished — state.json.lock " +
			"appears to be leaked/held, indicating SealCycle's flock acquisition is not released on " +
			"the success path")
	}
}
