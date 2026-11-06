package integration

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestEpochHolder_ClaimIncrementsOncePerHolderAndKeepsItsValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := NewEpochHolder(dir)

	a, err := first.Claim()
	if err != nil || a != 1 {
		t.Fatalf("first claim = %d, %v, want 1", a, err)
	}
	again, err := first.Claim()
	if err != nil || again != 1 {
		t.Fatalf("second claim of the same holder = %d, %v, want the held 1", again, err)
	}
	b, err := NewEpochHolder(dir).Claim()
	if err != nil || b != 2 {
		t.Fatalf("a new holder = %d, %v, want 2", b, err)
	}
}

func TestEpochHolder_CurrentReadsTheFileSoAStaleHolderSeesTheNewOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := NewEpochHolder(dir)
	held, _ := old.Claim()
	if cur, err := old.Current(); err != nil || cur != held {
		t.Fatalf("current = %d, %v, want %d", cur, err, held)
	}
	if _, err := NewEpochHolder(dir).Claim(); err != nil {
		t.Fatal(err)
	}
	if cur, err := old.Current(); err != nil || cur == held {
		t.Fatalf("current = %d, %v, want a number above the held %d", cur, err, held)
	}
}

func TestEpochHolder_ConcurrentHoldersGetDistinctNumbers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const n = 8
	got := make([]uint64, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], _ = NewEpochHolder(dir).Claim()
		}()
	}
	wg.Wait()
	seen := map[uint64]bool{}
	for _, v := range got {
		if v == 0 || seen[v] {
			t.Fatalf("claims = %v, want %d distinct numbers above 0", got, n)
		}
		seen[v] = true
	}
}

func TestEpochHolder_RefusesAFileThatIsNotANumber(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(EpochPath(dir), []byte("many\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewEpochHolder(dir)
	if _, err := h.Claim(); err == nil {
		t.Error("Claim accepted a corrupt epoch file")
	}
	if _, err := h.Current(); err == nil {
		t.Error("Current accepted a corrupt epoch file")
	}
}

func TestEpochHolder_ReportsAnUnwritableDirectory(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEpochHolder(filepath.Join(blocker, "sub")).Claim(); err == nil {
		t.Error("Claim under a file succeeded")
	}
}

func TestEpochHolder_AnAbsentFileReadsAsZero(t *testing.T) {
	t.Parallel()
	if cur, err := NewEpochHolder(t.TempDir()).Current(); err != nil || cur != 0 {
		t.Fatalf("current = %d, %v, want 0 and no error", cur, err)
	}
}

func TestEpochHolder_ReportsALockThatCannotBeTaken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, epochLockName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEpochHolder(dir).Claim(); err == nil {
		t.Fatal("Claim took a lock that is a directory")
	}
}

func TestEpochHolder_ReportsAFileThatCannotBeRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(EpochPath(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEpochHolder(dir).Current(); err == nil {
		t.Fatal("Current read a directory as the epoch")
	}
}

func TestEpochReader_TheHolderSatisfiesItAndReadsTheFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	holder := NewEpochHolder(dir)
	if _, err := holder.Claim(); err != nil {
		t.Fatal(err)
	}
	var reader EpochReader = holder
	got, err := reader.Current()
	if err != nil || got != 1 {
		t.Fatalf("Current = %d, %v, want 1", got, err)
	}
}
