package flock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTryLock_AParentThatIsAFileFailsAtTheMkdir(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	release, held, err := TryLock(filepath.Join(file, "run.lock"))

	if err == nil || held || release != nil || !strings.Contains(err.Error(), "flock mkdir") {
		t.Fatalf("TryLock under a file = held %v, err %v, want a mkdir error and no hold", held, err)
	}
}

func TestTryLock_AnAbsPathErrorFallsBackToThePathAsGiven(t *testing.T) {
	old := absFn
	t.Cleanup(func() { absFn = old })
	absFn = func(string) (string, error) { return "", errors.New("no working directory") }
	path := filepath.Join(t.TempDir(), "run.lock")

	release, held, err := TryLock(path)
	if err != nil || held {
		t.Fatalf("TryLock(%s) with an abs error = held %v, err %v, want the lock on the path as given", path, held, err)
	}
	defer release()
	_, again, err := TryLock(path)

	if err != nil || !again {
		t.Fatalf("a second TryLock(%s) = held %v, err %v, want held under the same key", path, again, err)
	}
}
