package selfsha

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPath_ResolvesTheRunningExecutableThroughSymlinks(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(self)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Path()
	if err != nil || got != real {
		t.Errorf("Path() = %q, %v; want %q", got, err, real)
	}
}

func TestRunning_HashesTheResolvedPathAndReportsAResolverFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bin")
	if err := os.WriteFile(file, []byte("abc"), 0o755); err != nil {
		t.Fatal(err)
	}
	const sha = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got, err := hashExecutable(func() (string, error) { return file, nil }); err != nil || got != sha {
		t.Errorf("hashExecutable = %q, %v; want %q", got, err, sha)
	}
	if got, err := hashExecutable(func() (string, error) { return "", os.ErrNotExist }); err == nil || got != "" {
		t.Errorf("a resolver failure must give no digest, got %q, %v", got, err)
	}
}

func TestResolveExecutable_FollowsASymlinkAndFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-evolve")
	if err := os.WriteFile(target, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "evolve")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	wantTarget, _ := filepath.EvalSymlinks(target)
	got, err := resolveExecutable(func() (string, error) { return link, nil })
	if err != nil || got != wantTarget {
		t.Errorf("resolveExecutable(link) = %q, %v; want %q", got, err, wantTarget)
	}
	if got, err := resolveExecutable(func() (string, error) { return filepath.Join(dir, "gone"), nil }); err == nil || got != "" {
		t.Errorf("a dangling path must fail with no entry, got %q, %v", got, err)
	}
	if got, err := resolveExecutable(func() (string, error) { return "", os.ErrNotExist }); err == nil || got != "" {
		t.Errorf("a resolver error must fail with no entry, got %q, %v", got, err)
	}
}
