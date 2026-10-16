package mintregistry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func blockedParent(t *testing.T) string {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(parent, []byte("a file where a directory must be"), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, "active-mints.json")
}

func writeCorruptRegistry(t *testing.T) string {
	t.Helper()
	path := Path(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAppendResetsACorruptRegistryAndKeepsTheNewMint(t *testing.T) {
	t.Parallel()
	path := writeCorruptRegistry(t)
	now := time.Now()
	if err := Append(path, "fresh", now); err != nil {
		t.Fatalf("a corrupt registry must not brick the next mint: %v", err)
	}
	names, err := ActiveNames(path, now)
	if err != nil || len(names) != 1 || !names["fresh"] {
		t.Errorf("want only the new mint after the reset, got %v, %v", names, err)
	}
}

func TestAppendFailsWhenTheRegistryCannotBeLocked(t *testing.T) {
	t.Parallel()
	if err := Append(blockedParent(t), "name", time.Now()); err == nil {
		t.Fatal("an unlockable registry must fail the append")
	}
}

func TestQuarantineCorruptFailsWhenTheRegistryCannotBeLocked(t *testing.T) {
	t.Parallel()
	quarantined, err := QuarantineCorrupt(blockedParent(t))
	if err == nil || quarantined {
		t.Fatalf("an unlockable registry cannot be quarantined: %v, %v", quarantined, err)
	}
}

func TestActiveNamesReportsAnUnreadableRegistryAsAnErrorWithAnEmptySet(t *testing.T) {
	t.Parallel()
	path := Path(t.TempDir())
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := ActiveNames(path, time.Now())
	if err == nil || len(names) != 0 {
		t.Fatalf("a registry that cannot be read fails safe: %v, %v", names, err)
	}
}

func TestQuarantineCorruptFailsWhenTheRegistryCannotBeRenamedAway(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	path := writeCorruptRegistry(t)
	lock, err := os.Create(path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	quarantined, err := QuarantineCorrupt(path)
	if err == nil || quarantined {
		t.Fatalf("a failed rename must be reported, not hidden: %v, %v", quarantined, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("the registry must stay in place when the rename fails: %v", statErr)
	}
}
