package core

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

// mutexSpyLocker is a WIRING spy, not a serialization proof: it deliberately
// never asserts "no two holders at once", since its own mutex would make that
// check tautological. The real index-serialization proof lives in
// TestWriteCycleDossier_ConcurrentRealRepo_BothLand. `acquired` is atomic
// (bumped before the mutex, including on the failErr path); `released` is
// mutated only while the mutex is held.
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

func TestWriteCycleDossier_AcquiresGitMutationLock(t *testing.T) {
	root := initDossierRepo(t)
	spy := &mutexSpyLocker{}

	if err := writeCycleDossier(spy.acquire, cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 11, Goal: "wire the lock", RunID: "run", Outcome: CycleOutcomeShippedViaBuild}); err != nil {
		t.Fatalf("writeCycleDossier: %v", err)
	}
	if got := atomic.LoadInt32(&spy.acquired); got != 1 {
		t.Errorf("git-mutation lock acquired %d times, want 1 (the dossier commit must serialize on the shared lock)", got)
	}
	if spy.released != 1 {
		t.Errorf("git-mutation lock released %d times, want 1 (must release after the commit)", spy.released)
	}
}

func TestWriteCycleDossier_ConcurrentLanesEachAcquireAndRelease(t *testing.T) {
	const lanes = 4
	spy := &mutexSpyLocker{}
	var wg sync.WaitGroup
	errs := make([]error, lanes)
	for i := 0; i < lanes; i++ {
		root := initDossierRepo(t)
		wg.Add(1)
		go func(i int, root string) {
			defer wg.Done()
			errs[i] = writeCycleDossier(spy.acquire, cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 100 + i, Goal: "lane", RunID: "run", Outcome: CycleOutcomeShippedViaBuild})
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

func TestWriteCycleDossier_LockError_FailsOpen(t *testing.T) {
	root := initDossierRepo(t)
	spy := &mutexSpyLocker{failErr: errors.New("flock unavailable")}

	if err := writeCycleDossier(spy.acquire, cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 12, Goal: "fail open", RunID: "run", Outcome: CycleOutcomeShippedViaBuild}); err != nil {
		t.Fatalf("a lock error must fail-open, not fail the dossier write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-12.json")); err != nil {
		t.Errorf("dossier must still be written despite a lock error (fail-open): %v", err)
	}
	if spy.released != 0 {
		t.Errorf("release must NOT be called when acquire failed; got %d", spy.released)
	}
}

func TestNewOrchestrator_WiresGitMutationLock(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if o.gitMutationLock == nil {
		t.Fatal("NewOrchestrator must wire gitMutationLock; a nil locker makes the dossier commit silently race the shared .git/index (fail-open masks it)")
	}
}

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

func TestWriteCycleDossier_ConcurrentRealRepo_BothLand(t *testing.T) {
	root := initDossierRepo(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = writeCycleDossier(defaultGitMutationLock, cycleDossierParams{ProjectRoot: root, WorkspacePath: t.TempDir(), Cycle: 200 + i, Goal: "concurrent", RunID: "run", Outcome: CycleOutcomeShippedViaBuild})
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
