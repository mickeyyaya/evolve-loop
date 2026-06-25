package selfsha

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestOf(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "blob")
	content := []byte("evolve-binary-bytes\x00\x01\x02")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])

	got, err := Of(p)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if got != want {
		t.Errorf("Of=%q want %q", got, want)
	}
}

func TestOf_MissingFile(t *testing.T) {
	if _, err := Of(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestRunning(t *testing.T) {
	got, err := Running()
	if err != nil {
		t.Fatalf("Running: %v", err)
	}
	if len(got) != 64 {
		t.Errorf("Running()=%q len=%d, want a 64-char hex sha256", got, len(got))
	}
	if _, err := hex.DecodeString(got); err != nil {
		t.Errorf("Running()=%q is not valid hex: %v", got, err)
	}
}

// Of is a read-only hasher with no shared state, so concurrent calls against
// the same file must return the same digest with no race (the per-phase
// integrity capture hashes the running binary from each phase's goroutine).
// Run with -race to prove it.
func TestOf_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "blob")
	content := []byte("concurrent-hashing-bytes\x00\xff")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])

	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			got, err := Of(p)
			if err != nil {
				t.Errorf("concurrent Of error: %v", err)
				return
			}
			if got != want {
				t.Errorf("concurrent Of=%q want %q", got, want)
			}
		}()
	}
	wg.Wait()
}
