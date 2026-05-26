package bridge

import (
	"context"
	"errors"
	"io/fs"
	"testing"
)

// coverage_batch5_test.go — defensive branches reachable only via seams:
// the challenge-token entropy error and the manifest-FS read errors.

// fakeManifestFS is a swappable manifestSource for error injection.
type fakeManifestFS struct {
	entries    []fs.DirEntry
	readDirErr error
	files      map[string][]byte
}

func (f fakeManifestFS) ReadFile(name string) ([]byte, error) {
	if b, ok := f.files[name]; ok {
		return b, nil
	}
	return nil, errors.New("fake: not found: " + name)
}

func (f fakeManifestFS) ReadDir(string) ([]fs.DirEntry, error) {
	if f.readDirErr != nil {
		return nil, f.readDirErr
	}
	return f.entries, nil
}

type fakeDirEntry struct{ n string }

func (e fakeDirEntry) Name() string             { return e.n }
func (fakeDirEntry) IsDir() bool                { return false }
func (fakeDirEntry) Type() fs.FileMode          { return 0 }
func (fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func swapManifestFS(t *testing.T, fake manifestSource) {
	t.Helper()
	orig := manifestFS
	manifestFS = fake
	t.Cleanup(func() { manifestFS = orig })
}

func TestDefaultChallengeToken_RandError(t *testing.T) {
	orig := randRead
	randRead = func([]byte) (int, error) { return 0, errors.New("no entropy") }
	defer func() { randRead = orig }()
	if _, err := defaultChallengeToken(); err == nil {
		t.Fatal("expected error when entropy source fails")
	}
}

func TestManifestNames_ReadDirError(t *testing.T) {
	swapManifestFS(t, fakeManifestFS{readDirErr: errors.New("readdir boom")})
	if names := ManifestNames(); names != nil {
		t.Fatalf("ManifestNames on ReadDir error = %v, want nil", names)
	}
}

func TestProbe_LoadManifestFailsForListedCLI(t *testing.T) {
	// ReadDir lists x.json but ReadFile fails → Probe records tier "none".
	swapManifestFS(t, fakeManifestFS{entries: []fs.DirEntry{fakeDirEntry{"x.json"}}})
	eng := NewEngine(Deps{LookPath: func(string) (string, error) { return "", errNoBin }})
	p, err := eng.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe err: %v", err)
	}
	if p.CLIs["x"] != "none" {
		t.Fatalf("x tier = %q, want none (manifest load failed)", p.CLIs["x"])
	}
}
