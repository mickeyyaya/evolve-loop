package swarm

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRegisterTestSocketDir_ARegisteredDirIsListedAndAGoneDirIsPruned(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := filepath.Join(t.TempDir(), "tmux-0")
	if err := RegisterTestSocketDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := RegisterTestSocketDir(dir); err != nil {
		t.Fatal(err)
	}
	if got := registeredTestSocketDirs(); !slices.Equal(got, []string{dir}) {
		t.Fatalf("registered dirs = %v; want %s once", got, dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if got := registeredTestSocketDirs(); len(got) != 0 {
		t.Errorf("registered dirs = %v; a dir that is gone must be pruned", got)
	}
}

func registryEntryFor(dir string) string {
	sum := sha256.Sum256([]byte(dir))
	return filepath.Join(testSocketRegistry(), hex.EncodeToString(sum[:8]))
}

func TestRegisterTestSocketDir_ReportsADirItCannotCreate(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RegisterTestSocketDir(filepath.Join(file, "tmux-0")); err == nil {
		t.Fatal("a dir under a regular file must be refused")
	}
}

func TestRegisterTestSocketDir_ReportsARegistryItCannotCreate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", file)
	if err := RegisterTestSocketDir(filepath.Join(t.TempDir(), "tmux-0")); err == nil {
		t.Fatal("a registry under a regular file must be refused")
	}
}

func TestRegisterTestSocketDir_ReportsAnEntryItCannotWrite(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := filepath.Join(t.TempDir(), "tmux-0")
	if err := os.MkdirAll(registryEntryFor(dir)+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RegisterTestSocketDir(dir); err == nil {
		t.Fatal("an entry whose temporary file cannot be written must be refused")
	}
}

func TestRegisteredTestSocketDirs_SkipsTemporaryAndUnreadableEntries(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := filepath.Join(t.TempDir(), "tmux-0")
	if err := RegisterTestSocketDir(dir); err != nil {
		t.Fatal(err)
	}
	registry := testSocketRegistry()
	if err := os.WriteFile(filepath.Join(registry, "half.tmp"), []byte(dir+"-half"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(registry, "unreadable"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := registeredTestSocketDirs(); !slices.Equal(got, []string{dir}) {
		t.Fatalf("registered dirs = %v; want only %s", got, dir)
	}
}
