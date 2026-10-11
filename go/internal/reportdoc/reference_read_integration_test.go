//go:build integration

package reportdoc

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func swap[T any](t *testing.T, seam *T, fake T) {
	t.Helper()
	saved := *seam
	*seam = fake
	t.Cleanup(func() { *seam = saved })
}

func nthCall[T any](n int, nth T, otherwise T) func() T {
	calls := 0
	return func() T {
		calls++
		if calls == n {
			return nth
		}
		return otherwise
	}
}

func referenceRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	return root
}

func writeRef(t *testing.T, root, rel, body string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func wrapGit(t *testing.T, caseArms string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \"$3:$4\" in\n" + caseArms + "\nesac\nexec '" + real + "' \"$@\"\n"
	fakeclitest.Install(t, filepath.Join(bin, "git"), script)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestReadCurrentReference_ReadsFilesLinksAndGitlinks(t *testing.T) {
	root := referenceRoot(t)
	writeRef(t, root, "docs/a.md", "line one\nline two\n")
	if err := os.Symlink("docs/a.md", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	sub := gittest.Fixture(t)
	sub.Git("commit", "-q", "--allow-empty", "-m", "nested")
	if err := os.Rename(sub.Dir, filepath.Join(root, "vendor-sub")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(filepath.Join(root, "vendor-sub"), sub.Dir) })
	commit := strings.TrimSpace(gitOut(t, filepath.Join(root, "vendor-sub"), "rev-parse", "HEAD"))
	writeRef(t, root, "exact.bin", "")
	if err := os.Truncate(filepath.Join(root, "exact.bin"), maxCitationBytes); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ref, body string
		oneLine   bool
		size      int
	}{
		{"docs/a.md", "line one\nline two\n", false, -1},
		{"link", "docs/a.md", true, -1},
		{"vendor-sub", commit, true, -1},
		{"exact.bin", "", false, maxCitationBytes},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			body, oneLine, err := readCurrentReference(context.Background(), root, tc.ref)

			if err != nil || oneLine != tc.oneLine {
				t.Fatalf("readCurrentReference(%q) oneLine=%v err=%v; want %v, nil", tc.ref, oneLine, err, tc.oneLine)
			}
			if tc.size >= 0 && len(body) != tc.size || tc.size < 0 && string(body) != tc.body {
				t.Fatalf("readCurrentReference(%q) body len %d, want %q / size %d", tc.ref, len(body), tc.body, tc.size)
			}
		})
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func TestReadCurrentReference_RefusesUnsafeOrUnreadableTargets(t *testing.T) {
	root := referenceRoot(t)
	outside := t.TempDir()
	writeRef(t, outside, "secret.txt", "secret")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plain-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeRef(t, root, "huge.bin", "")
	if err := os.Truncate(filepath.Join(root, "huge.bin"), maxCitationBytes+1); err != nil {
		t.Fatal(err)
	}
	locked := writeRef(t, root, "locked.txt", "x")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	for _, tc := range []struct {
		name, root, ref, want string
		is                    error
	}{
		{"missing root", filepath.Join(root, "absent"), "x", "", fs.ErrNotExist},
		{"missing reference", root, "nope.md", "", fs.ErrNotExist},
		{"file used as a directory", root, "locked.txt/child", "not a directory", nil},
		{"parent escapes", root, "escape/secret.txt", "citation target escapes the worktree", nil},
		{"plain directory", root, "plain-dir", "citation directory is not a readable gitlink", nil},
		{"fifo", root, "pipe", "citation target has unsupported file type", nil},
		{"one byte over the cap", root, "huge.bin", "citation target exceeds 16 MiB", nil},
		{"unreadable file", root, "locked.txt", "permission denied", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, oneLine, err := readCurrentReference(context.Background(), tc.root, tc.ref)

			if body != nil || oneLine || err == nil || tc.is != nil && !errors.Is(err, tc.is) || tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("readCurrentReference(%q) = %q, %v, %v; want only an error (%q / %v)", tc.ref, body, oneLine, err, tc.want, tc.is)
			}
		})
	}
}

func TestReadCurrentReference_ConcurrentChangesAreRefused(t *testing.T) {
	root := referenceRoot(t)
	path := writeRef(t, root, "docs/a.md", "stable\n")
	if err := os.Symlink("docs/a.md", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	other := writeRef(t, t.TempDir(), "other.md", "a different, longer body\n")
	otherInfo, err := os.Stat(other)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, ref, want string
		arrange         func(t *testing.T)
	}{
		{"parent unresolvable", "docs/a.md", "parent gone", func(t *testing.T) {
			next := nthCall(2, errors.New("parent gone"), nil)
			swap(t, &evalSymlinks, func(p string) (string, error) {
				if err := next(); err != nil {
					return "", err
				}
				return filepath.EvalSymlinks(p)
			})
		}},
		{"link unreadable", "link", "readlink failed", func(t *testing.T) {
			swap(t, &readlink, func(string) (string, error) { return "", errors.New("readlink failed") })
		}},
		{"link replaced", "link", "citation symlink changed while reading", func(t *testing.T) {
			next := nthCall(2, true, false)
			swap(t, &lstat, func(p string) (os.FileInfo, error) {
				if next() {
					return otherInfo, nil
				}
				return os.Lstat(p)
			})
		}},
		{"link removed", "link", "citation symlink changed while reading", func(t *testing.T) {
			next := nthCall(2, true, false)
			swap(t, &lstat, func(p string) (os.FileInfo, error) {
				if next() {
					return nil, fs.ErrNotExist
				}
				return os.Lstat(p)
			})
		}},
		{"opened handle unstatable", "docs/a.md", "citation target changed type or identity while opening", func(t *testing.T) {
			swap(t, &statFile, func(*os.File) (os.FileInfo, error) { return nil, errors.New("stat failed") })
		}},
		{"opened a different file", "docs/a.md", "citation target changed type or identity while opening", func(t *testing.T) {
			swap(t, &openRegularNoFollow, func(string) (*os.File, error) { return os.Open(other) })
		}},
		{"handle not readable", "docs/a.md", "bad file descriptor", func(t *testing.T) {
			swap(t, &openRegularNoFollow, func(p string) (*os.File, error) { return os.OpenFile(p, os.O_WRONLY, 0) })
		}},
		{"grew past the cap before reading", "docs/a.md", "citation target exceeds 16 MiB", func(t *testing.T) {
			swap(t, &openRegularNoFollow, func(p string) (*os.File, error) {
				t.Cleanup(func() { _ = os.WriteFile(path, []byte("stable\n"), 0o644) })
				if err := os.Truncate(p, maxCitationBytes+1); err != nil {
					return nil, err
				}
				return OpenRegularNoFollow(p)
			})
		}},
		{"touched while reading", "docs/a.md", "citation target changed while reading", func(t *testing.T) {
			next := nthCall(2, true, false)
			swap(t, &statFile, func(f *os.File) (os.FileInfo, error) {
				if next() {
					info, err := f.Stat()
					if err != nil {
						return nil, err
					}
					later := info.ModTime().Add(time.Second)
					if err := os.Chtimes(path, later, later); err != nil {
						return nil, err
					}
				}
				return f.Stat()
			})
		}},
		{"changed while reading", "docs/a.md", "citation target changed while reading", func(t *testing.T) {
			next := nthCall(2, true, false)
			swap(t, &statFile, func(f *os.File) (os.FileInfo, error) {
				if next() {
					return otherInfo, nil
				}
				return f.Stat()
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.arrange(t)

			body, oneLine, err := readCurrentReference(context.Background(), root, tc.ref)

			if body != nil || oneLine || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("readCurrentReference(%q) = %q, %v, %v; want an error naming %q", tc.ref, body, oneLine, err, tc.want)
			}
		})
	}
}

func baseRepo(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeRef(t, repo.Dir, "docs/deleted.md", "one\ntwo\n")
	writeRef(t, repo.Dir, "dir/inner.md", "inner\n")
	if err := os.Symlink("docs/deleted.md", filepath.Join(repo.Dir, "link")); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("ab", 20)+",sub")
	repo.Git("commit", "-q", "-m", "base")
	return repo, repo.Git("rev-parse", "HEAD")
}

func TestReadBaseReference_ReadsBlobsLinksAndGitlinksFromTheBase(t *testing.T) {
	repo, base := baseRepo(t)
	for _, tc := range []struct {
		ref, body string
		oneLine   bool
	}{
		{"docs/deleted.md", "one\ntwo\n", false},
		{"link", "docs/deleted.md", true},
		{"sub", strings.Repeat("ab", 20), true},
	} {
		body, oneLine, err := readBaseReference(context.Background(), repo.Dir, base, tc.ref)
		if err != nil || string(body) != tc.body || oneLine != tc.oneLine {
			t.Fatalf("readBaseReference(%q) = %q, %v, %v; want %q, %v", tc.ref, body, oneLine, err, tc.body, tc.oneLine)
		}
	}
}

func TestReadBaseReference_RefusesWhatTheBaseCannotVouchFor(t *testing.T) {
	for _, tc := range []struct {
		name, ref, base, arms, want string
	}{
		{name: "unknown base", ref: "docs/deleted.md", base: strings.Repeat("0", 40), want: "inspect deleted base path"},
		{name: "absent at base", ref: "docs/never.md", want: "deleted path is absent from the base"},
		{name: "prefix only", ref: "docs/deleted", want: "deleted path is absent from the base"},
		{name: "a tree", ref: "dir", want: "deleted base path has unsupported git type tree"},
		{name: "listing names another path", ref: "docs/deleted.md", arms: `ls-tree:-z) printf '100644 blob 0123456789012345678901234567890123456789\tdocs/other.md\0'; exit 0;;`, want: "deleted path is absent from the base"},
		{name: "listing malformed", ref: "docs/deleted.md", arms: `ls-tree:-z) printf '100644 0123456789\tdocs/deleted.md\0'; exit 0;;`, want: "deleted path is absent from the base"},
		{name: "size unreadable", ref: "docs/deleted.md", arms: `cat-file:-s) exit 1;;`, want: "size deleted base blob"},
		{name: "size garbled", ref: "docs/deleted.md", arms: `cat-file:-s) echo lots; exit 0;;`, want: "deleted base blob exceeds 16 MiB or has invalid size"},
		{name: "size negative", ref: "docs/deleted.md", arms: `cat-file:-s) echo -1; exit 0;;`, want: "deleted base blob exceeds 16 MiB or has invalid size"},
		{name: "size over the cap", ref: "docs/deleted.md", arms: `cat-file:-s) echo 16777217; exit 0;;`, want: "deleted base blob exceeds 16 MiB or has invalid size"},
		{name: "blob longer than its size", ref: "docs/deleted.md", arms: `cat-file:blob) printf 'one\ntwo\nthree\n'; exit 0;;`, want: "deleted base blob size changed while reading"},
		{name: "blob unreadable", ref: "docs/deleted.md", arms: `cat-file:blob) exit 1;;`, want: "read deleted base blob"},
		{name: "blob size drifted", ref: "docs/deleted.md", arms: `cat-file:blob) printf 'one\n'; exit 0;;`, want: "deleted base blob size changed while reading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, base := baseRepo(t)
			if tc.base != "" {
				base = tc.base
			}
			if tc.arms != "" {
				wrapGit(t, tc.arms)
			}

			body, oneLine, err := readBaseReference(context.Background(), repo.Dir, base, tc.ref)

			if body != nil || oneLine || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("readBaseReference(%q) = %q, %v, %v; want an error naming %q", tc.ref, body, oneLine, err, tc.want)
			}
		})
	}
}
