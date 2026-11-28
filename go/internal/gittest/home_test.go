package gittest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsolateHome_PointsHomeAtAnEmptyScratchDirAndRestores(t *testing.T) {
	real := t.TempDir()
	t.Setenv("HOME", real)
	t.Setenv("CODEX_HOME", filepath.Join(real, ".codex"))
	t.Setenv("GOCACHE", "")
	t.Setenv("GOPATH", "")

	restore, err := IsolateHome()
	if err != nil {
		t.Fatalf("IsolateHome: %v", err)
	}
	scratch := os.Getenv("HOME")
	if scratch == real || scratch == "" {
		t.Fatalf("HOME = %q, want a scratch dir other than %q", scratch, real)
	}
	if _, err := os.Stat(filepath.Join(scratch, ".codex", "auth.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("scratch home holds an auth file: %v", err)
	}
	if got := os.Getenv("CODEX_HOME"); !strings.HasPrefix(got, scratch) {
		t.Fatalf("CODEX_HOME = %q, want it under %q", got, scratch)
	}
	if os.Getenv("GOCACHE") == "" || os.Getenv("GOPATH") == "" {
		t.Fatalf("GOCACHE=%q GOPATH=%q, want both pinned to the real home before the swap", os.Getenv("GOCACHE"), os.Getenv("GOPATH"))
	}
	if err := restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := os.Getenv("HOME"); got != real {
		t.Fatalf("HOME after restore = %q, want %q", got, real)
	}
	if _, err := os.Stat(scratch); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("scratch home survives restore: %v", err)
	}
}

func TestIsolateHome_KeepsAnExplicitGoCacheAndFailsWhenNoScratchDirExists(t *testing.T) {
	t.Setenv("GOCACHE", "/keep/gocache")
	t.Setenv("GOPATH", "/keep/gopath")
	restore, err := IsolateHome()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GOCACHE") != "/keep/gocache" || os.Getenv("GOPATH") != "/keep/gopath" {
		t.Fatalf("GOCACHE=%q GOPATH=%q, want the explicit values kept", os.Getenv("GOCACHE"), os.Getenv("GOPATH"))
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}

	old := mkdirTemp
	t.Cleanup(func() { mkdirTemp = old })
	boom := errors.New("no tmp")
	mkdirTemp = func(string, string) (string, error) { return "", boom }
	if _, err := IsolateHome(); !errors.Is(err, boom) {
		t.Fatalf("IsolateHome error = %v, want %v", err, boom)
	}
	mkdirTemp = old

	oldMk := mkdirAll
	t.Cleanup(func() { mkdirAll = oldMk })
	mkdirAll = func(string, os.FileMode) error { return boom }
	if _, err := IsolateHome(); !errors.Is(err, boom) {
		t.Fatalf("IsolateHome mkdir error = %v, want %v", err, boom)
	}
}

func TestIsolateHome_RestoreReportsARemoveFailure(t *testing.T) {
	restore, err := IsolateHome()
	if err != nil {
		t.Fatal(err)
	}
	scratch := os.Getenv("HOME")
	oldRemove := removeAll
	t.Cleanup(func() { removeAll = oldRemove })
	boom := errors.New("busy")
	removeAll = func(string) error { return boom }
	if err := restore(); !errors.Is(err, boom) {
		t.Fatalf("restore error = %v, want %v", err, boom)
	}
	removeAll = oldRemove
	_ = os.RemoveAll(scratch)
}

func TestHermeticHomeError_NamesAHomeThatIsNotAnIsolatedScratchDir(t *testing.T) {
	restore, err := IsolateHome()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := restore(); err != nil {
			t.Error(err)
		}
	})
	if err := HermeticHomeError(); err != nil {
		t.Fatalf("isolated home: %v", err)
	}

	for name, mutate := range map[string]func(*testing.T){
		"home outside the scratch prefix": func(t *testing.T) { t.Setenv("HOME", t.TempDir()) },
		"codex home outside home":         func(t *testing.T) { t.Setenv("CODEX_HOME", t.TempDir()) },
		"auth file under home": func(t *testing.T) {
			dir := filepath.Join(os.Getenv("HOME"), ".codex")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			mutate(t)
			if err := HermeticHomeError(); !errors.Is(err, ErrHomeNotHermetic) {
				t.Fatalf("HermeticHomeError = %v, want ErrHomeNotHermetic", err)
			}
		})
	}
}

func TestIsolateHome_RestoreReturnsEveryKeyAndPinsTheToolchainBeforeTheSwap(t *testing.T) {
	real := t.TempDir()
	t.Setenv("HOME", real)
	t.Setenv("GOCACHE", "")
	t.Setenv("GOPATH", "")
	t.Setenv("CODEX_HOME", "")
	if err := os.Unsetenv("CODEX_HOME"); err != nil {
		t.Fatal(err)
	}
	restore, err := IsolateHome()
	if err != nil {
		t.Fatal(err)
	}
	scratch := os.Getenv("HOME")
	if got, want := os.Getenv("GOPATH"), filepath.Join(real, "go"); got != want {
		t.Errorf("GOPATH = %q, want %q: the toolchain dirs are pinned before HOME moves", got, want)
	}
	if got := os.Getenv("GOCACHE"); got == "" || strings.HasPrefix(got, scratch) {
		t.Errorf("GOCACHE = %q, want a cache outside the scratch home %q", got, scratch)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	if v, set := os.LookupEnv("CODEX_HOME"); set {
		t.Errorf("CODEX_HOME = %q after restore, want unset as before", v)
	}
	for key, want := range map[string]string{"HOME": real, "GOCACHE": "", "GOPATH": ""} {
		if got := os.Getenv(key); got != want {
			t.Errorf("%s = %q after restore, want %q", key, got, want)
		}
	}
}
