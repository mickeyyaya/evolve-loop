package core

// dossier_producer_lock_test.go — fleet-ship-git-index-lock-serialization.
//
// The dossier closeout commit ("dossier: cycle-N closeout") mutates the SHARED
// main-repo .git/index. Under a fleet (one subprocess per lane) it races sibling
// lanes' ship + dossier commits on .git/index.lock. These tests pin that
// writeCycleDossier serializes that commit on the integrator's .evolve/ship.lock
// (the SAME file ship holds), fails OPEN on a lock error (a dossier is
// best-effort), and that two concurrent real commits both land.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

// mutexSpyLocker is a deterministic gitMutationLocker for TDD: a REAL mutex (so
// acquire/release model a real blocking lock) plus counters proving the dossier
// commit acquired the lock (acquired) and released it (released). It is a WIRING
// spy — it proves writeCycleDossier routes through the injected locker under
// concurrency. It deliberately does NOT assert serialization: the spy's own
// mutex would make any "no two holders at once" check tautological (a premature
// release would still pass). The genuine index-serialization proof — that the
// lock actually prevents a .git/index.lock race — lives in
// TestWriteCycleDossier_ConcurrentRealRepo_BothLand (real flock, real shared
// repo). When failErr is set, acquire records the attempt then returns the error.
//
// Only `acquired` is atomic (bumped BEFORE the mutex, and on the failErr path
// that never locks); `released` is mutated only while the mutex is held.
type mutexSpyLocker struct {
	mu       sync.Mutex
	acquired int32 // atomic — incremented before mu is taken
	released int   // mu-guarded
	failErr  error
}

func (s *mutexSpyLocker) acquire(projectRoot string) (func(), error) {
	atomic.AddInt32(&s.acquired, 1)
	if s.failErr != nil {
		return nil, s.failErr
	}
	s.mu.Lock()
	return func() {
		s.released++
		s.mu.Unlock()
	}, nil
}

// TestWriteCycleDossier_AcquiresGitMutationLock — the WIRING PROOF: the dossier
// commit must acquire (and release) the git-mutation lock exactly once. RED until
// writeCycleDossier actually calls the injected locker.
func TestWriteCycleDossier_AcquiresGitMutationLock(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	spy := &mutexSpyLocker{}

	if err := writeCycleDossier(spy.acquire, root, t.TempDir(), 11, "wire the lock", "run", CycleOutcomeShippedViaBuild, nil); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	if got := atomic.LoadInt32(&spy.acquired); got != 1 {
		t.Errorf("git-mutation lock acquired %d times, want 1 (the dossier commit must serialize on the shared lock)", got)
	}
	if spy.released != 1 {
		t.Errorf("git-mutation lock released %d times, want 1 (must release after the commit)", spy.released)
	}
}

// TestWriteCycleDossier_ConcurrentLanesEachAcquireAndRelease — under concurrency,
// EVERY lane's dossier commit must route through the injected locker: acquire and
// release each fire once per lane. This is the wiring-at-scale proof; the real
// index-serialization proof is TestWriteCycleDossier_ConcurrentRealRepo_BothLand
// (real flock, real shared repo). RED until writeCycleDossier acquires the lock.
func TestWriteCycleDossier_ConcurrentLanesEachAcquireAndRelease(t *testing.T) {
	const lanes = 4
	spy := &mutexSpyLocker{}
	var wg sync.WaitGroup
	errs := make([]error, lanes)
	for i := 0; i < lanes; i++ {
		root := t.TempDir()
		initDossierRepo(t, root)
		wg.Add(1)
		go func(i int, root string) {
			defer wg.Done()
			errs[i] = writeCycleDossier(spy.acquire, root, t.TempDir(), 100+i, "lane", "run", CycleOutcomeShippedViaBuild, nil)
		}(i, root)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("lane %d: writeCycleDossier: %v", i, e)
		}
	}
	if got := atomic.LoadInt32(&spy.acquired); got != lanes {
		t.Errorf("lock acquired %d times, want %d (every lane's dossier commit must acquire the shared locker)", got, lanes)
	}
	if spy.released != lanes {
		t.Errorf("lock released %d times, want %d (every lane must release after its commit)", spy.released, lanes)
	}
}

// TestWriteCycleDossier_LockError_FailsOpen — a lock-acquire error must NOT
// orphan the dossier: the commit's own bounded index.lock retry is the backstop,
// and a dossier is best-effort (losing it is worse than a rare unserialized
// commit). release must not be called when acquire failed.
func TestWriteCycleDossier_LockError_FailsOpen(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	spy := &mutexSpyLocker{failErr: errors.New("flock unavailable")}

	if err := writeCycleDossier(spy.acquire, root, t.TempDir(), 12, "fail open", "run", CycleOutcomeShippedViaBuild, nil); err != nil {
		t.Fatalf("a lock error must fail-open, not fail the dossier write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-12.json")); err != nil {
		t.Errorf("dossier must still be written despite a lock error (fail-open): %v", err)
	}
	if spy.released != 0 {
		t.Errorf("release must NOT be called when acquire failed; got %d", spy.released)
	}
}

// TestNewOrchestrator_WiresGitMutationLock guards a SILENT-regression hole:
// writeCycleDossier fails OPEN on a nil locker, so if NewOrchestrator's struct
// literal ever drops `gitMutationLock: defaultGitMutationLock`, the dossier
// closeout commit silently stops serializing on the shared index and NO other
// test would notice (the dossier is still written, just unserialized). This
// asserts the seam is wired, failing LOUDLY on that regression.
func TestNewOrchestrator_WiresGitMutationLock(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if o.gitMutationLock == nil {
		t.Fatal("NewOrchestrator must wire gitMutationLock; a nil locker makes the dossier commit silently race the shared .git/index (fail-open masks it)")
	}
}

// TestDefaultGitMutationLock_LocksShipIntegratorFile is the parity WIRING PROOF:
// the production locker must acquire the SAME file the ship integrator locks.
// Both now resolve it through flock.ShipLockPath (ship's acquireShipLock and this
// package's defaultGitMutationLock), so drift is structurally impossible — this
// asserts defaultGitMutationLock actually locks flock.ShipLockPath(root) (flock
// O_CREATEs the file on Lock), i.e. the dossier commit and a sibling ship commit
// contend on ONE file rather than racing on .git/index.lock.
func TestDefaultGitMutationLock_LocksShipIntegratorFile(t *testing.T) {
	root := t.TempDir()
	release, err := defaultGitMutationLock(root)
	if err != nil {
		t.Fatalf("defaultGitMutationLock: %v", err)
	}
	defer release()
	if _, err := os.Stat(flock.ShipLockPath(root)); err != nil {
		t.Errorf("defaultGitMutationLock did not lock the shared ship integrator file %q: %v", flock.ShipLockPath(root), err)
	}
}

// TestWriteCycleDossier_ConcurrentRealRepo_BothLand — acceptance
// (SerializesIndexMutations): two concurrent dossier commits to ONE shared repo
// under the REAL flock both land, with no 'index.lock: File exists' error.
func TestWriteCycleDossier_ConcurrentRealRepo_BothLand(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = writeCycleDossier(defaultGitMutationLock, root, t.TempDir(), 200+i, "concurrent", "run", CycleOutcomeShippedViaBuild, nil)
		}(i)
	}
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Fatalf("lane %d landed with error (index.lock race not serialized): %v", i, e)
		}
	}
	out, err := exec.Command("git", "-C", root, "log", "--oneline", "--all").CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	for _, cyc := range []string{"cycle-200", "cycle-201"} {
		if !strings.Contains(string(out), cyc+" closeout") {
			t.Errorf("dossier commit for %s did not land; git log:\n%s", cyc, out)
		}
	}
}
