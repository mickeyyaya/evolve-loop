//go:build integration

package explanationdocs

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

const zeroSHA = "0000000000000000000000000000000000000000"

func failGit(t *testing.T, pattern string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \"$3:$4\" in\n" + pattern + ") echo 'injected git failure' >&2; exit 1;;\nesac\nexec '" + real + "' \"$@\"\n"
	fakeclitest.Install(t, filepath.Join(bin, "git"), script)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func stubLstat(t *testing.T, fn func(string) (os.FileInfo, error)) {
	t.Helper()
	saved := lstat
	lstat = fn
	t.Cleanup(func() { lstat = saved })
}

func notADirectoryRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	return dir
}

func TestChangedSince_RejectsAMissingOrNonCommitBase(t *testing.T) {
	f := newFixture(t)
	blob := f.git(t, "rev-parse", "HEAD:config/app.yaml")
	for _, tc := range []struct {
		name, worktree, base, want string
	}{
		{"no worktree", "", f.base, "worktree and base SHA are required"},
		{"no base", f.worktree, "", "worktree and base SHA are required"},
		{"symbolic base", f.worktree, "HEAD", "base revision must be a full commit SHA"},
		{"short base", f.worktree, f.base[:39], "base revision must be a full commit SHA"},
		{"unknown sha1", f.worktree, zeroSHA, "base SHA is not a commit"},
		{"unknown sha256", f.worktree, strings.Repeat("a", 64), "base SHA is not a commit"},
		{"blob not commit", f.worktree, blob, "base SHA is not a commit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths, err := changedSince(context.Background(), tc.worktree, tc.base)

			if err == nil || !strings.Contains(err.Error(), tc.want) || paths != nil {
				t.Fatalf("changedSince = %q, %v; want %q", paths, err, tc.want)
			}
		})
	}
}

func TestChangedSince_ListsModifiedDeletedAndUntrackedOnceInOrder(t *testing.T) {
	f := newFixture(t)
	f.write(t, "z/kept.txt", "kept\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "second")
	base := f.git(t, "rev-parse", "HEAD")
	f.write(t, "config/app.yaml", "enabled: true\n")
	if err := os.Remove(filepath.Join(f.worktree, "z", "kept.txt")); err != nil {
		t.Fatal(err)
	}
	f.write(t, "b/new file.md", "new\n")
	f.write(t, "a/ünï.md", "unicode\n")

	got, err := changedSince(context.Background(), f.worktree, base)

	want := []string{"a/ünï.md", "b/new file.md", "config/app.yaml", "z/kept.txt"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("changedSince = %q, %v; want %q", got, err, want)
	}
}

func TestChangedSince_GitFailuresAreWrappedWithTheirOutput(t *testing.T) {
	for _, tc := range []struct{ pattern, want string }{
		{"diff:*", "--name-only: exit status 1: injected git failure"},
		{"ls-files:*", "git ls-files --others: exit status 1: injected git failure"},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			f := newFixture(t)
			failGit(t, tc.pattern)

			paths, err := changedSince(context.Background(), f.worktree, f.base)

			if err == nil || !strings.Contains(err.Error(), tc.want) || paths != nil {
				t.Fatalf("changedSince = %q, %v; want %q", paths, err, tc.want)
			}
		})
	}
}

func TestRequireLandedCommit_DemandsTheProjectHeadExactly(t *testing.T) {
	f := newFixture(t)
	f.write(t, "next.txt", "next\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "next")
	head := f.git(t, "rev-parse", "HEAD")
	for _, tc := range []struct {
		name, root, landed, want string
	}{
		{"head", f.worktree, head, ""},
		{"uppercase is not the resolved head", f.worktree, strings.ToUpper(head), "landed commit does not match ProjectRoot HEAD"},
		{"older commit", f.worktree, f.base, "landed commit does not match ProjectRoot HEAD"},
		{"abbreviated", f.worktree, head[:12], "landed commit must be a full commit SHA"},
		{"empty", f.worktree, "", "landed commit must be a full commit SHA"},
		{"not a repository", notADirectoryRepo(t), head, "resolve landed HEAD: exit status 128"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := requireLandedCommit(context.Background(), tc.root, tc.landed)

			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("requireLandedCommit = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExistsAtBase_AnswersForExactTreeEntriesOnly(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"config/app.yaml", true},
		{"config", true},
		{"config/app", false},
		{"config/APP.yaml", false},
		{"missing.txt", false},
	} {
		got, err := existsAtBase(context.Background(), f.worktree, f.base, tc.path)
		if err != nil || got != tc.want {
			t.Fatalf("existsAtBase(%q) = %v, %v; want %v", tc.path, got, err, tc.want)
		}
	}
}

func TestExistsAtBase_InvalidPathOrBaseIsAnError(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct{ path, base, want string }{
		{"../escape", f.base, `invalid base path "../escape"`},
		{"/abs/path", f.base, `invalid base path "/abs/path"`},
		{"", f.base, `invalid base path ""`},
		{"config/app.yaml", zeroSHA, "inspect config/app.yaml at base: exit status 128"},
	} {
		got, err := existsAtBase(context.Background(), f.worktree, tc.base, tc.path)
		if got || err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("existsAtBase(%q, %q) = %v, %v; want %q", tc.path, tc.base, got, err, tc.want)
		}
	}
}

func TestLineageHolds_SameCommitIsNeverALineage(t *testing.T) {
	f := newFixture(t)
	failGit(t, "*")

	ok, err := lineageHolds(context.Background(), f.worktree, f.base, f.base, []string{"config/app.yaml"})

	if ok || err != nil {
		t.Fatalf("lineageHolds(base, base) = %v, %v; want false, nil without any git call", ok, err)
	}
}

func TestLineageHolds_JudgesThePeerDiffAgainstTheLane(t *testing.T) {
	f := newFixture(t)
	f.write(t, "peer/only.txt", "peer\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "peer")
	peer := f.git(t, "rev-parse", "HEAD")
	for _, tc := range []struct {
		lane []string
		want bool
	}{
		{[]string{"config/app.yaml"}, true},
		{[]string{"peer/only.txt"}, false},
		{[]string{"PEER/ONLY.TXT"}, false},
	} {
		ok, err := lineageHolds(context.Background(), f.worktree, f.base, peer, tc.lane)
		if err != nil || ok != tc.want {
			t.Fatalf("lineageHolds(lane %q) = %v, %v; want %v", tc.lane, ok, err, tc.want)
		}
	}
}

func TestLineageHolds_PeerDiffFailureIsAWrappedError(t *testing.T) {
	f := newFixture(t)
	f.write(t, "peer/only.txt", "peer\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "peer")
	peer := f.git(t, "rev-parse", "HEAD")
	failGit(t, "diff:*")

	ok, err := lineageHolds(context.Background(), f.worktree, f.base, peer, []string{"config/app.yaml"})

	if ok || err == nil || !strings.Contains(err.Error(), "git diff "+f.base+" "+peer+" --name-only: exit status 1: injected git failure") {
		t.Fatalf("lineageHolds = %v, %v; want the wrapped diff failure", ok, err)
	}
}

func TestArchiveSupersededPredicatePackages_EveryFailureIsReturned(t *testing.T) {
	t.Run("bad base", func(t *testing.T) {
		f := newFixture(t)
		got, err := ArchiveSupersededPredicatePackages(context.Background(), f.worktree, "HEAD", ancestorsOf(42))
		if got != nil || err == nil || !strings.Contains(err.Error(), "full commit SHA") {
			t.Fatalf("got %q, %v; want the base error", got, err)
		}
	})
	t.Run("invalid ancestor", func(t *testing.T) {
		f := newFixture(t)
		escape := func([]string) []string { return []string{"../outside"} }
		got, err := ArchiveSupersededPredicatePackages(context.Background(), f.worktree, f.base, escape)
		if got != nil || err == nil || !strings.Contains(err.Error(), `invalid base path "../outside"`) {
			t.Fatalf("got %q, %v; want the invalid path error", got, err)
		}
	})
	t.Run("staging fails", func(t *testing.T) {
		f := newFixture(t)
		f.write(t, "go/acs/cycle41/predicates_test.go", "//go:build acs\n\npackage cycle41\n")
		failGit(t, "add:*")
		got, err := ArchiveSupersededPredicatePackages(context.Background(), f.worktree, f.base, ancestorsOf(42))
		if got != nil || err == nil || !strings.Contains(err.Error(), "stage superseded predicate archive: exit status 1: injected git failure") {
			t.Fatalf("got %q, %v; want the staging error", got, err)
		}
	})
}

const unpublishedRecord = "docs/explain/builds/cycle-41-old-run.md"

func archivedRecordPath() string {
	return filepath.ToSlash(filepath.Join(archiveDateDir(), "unshipped-build-explanations", filepath.Base(unpublishedRecord)))
}

func TestArchiveUnpublishedContinuationRecords_RecordsOnBaseOrUnchangedAreKept(t *testing.T) {
	f := newFixture(t)
	f.write(t, unpublishedRecord, "published\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "published record")
	base := f.git(t, "rev-parse", "HEAD")
	f.write(t, unpublishedRecord, "edited published\n")
	f.write(t, "docs/explain/builds/not-a-record.md", "overview\n")

	got, err := ArchiveUnpublishedContinuationRecords(context.Background(), f.worktree, base)

	if got != nil || err != nil {
		t.Fatalf("ArchiveUnpublishedContinuationRecords = %q, %v; want nil, nil", got, err)
	}
	if body, _ := os.ReadFile(filepath.Join(f.worktree, unpublishedRecord)); string(body) != "edited published\n" {
		t.Fatalf("a record on base was moved: %q", body)
	}
	if staged := f.git(t, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("nothing to archive still staged %q", staged)
	}
}

func TestArchiveUnpublishedContinuationRecords_EachRefusalIsALoudError(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		arrange    func(t *testing.T, f fixture)
	}{
		{"bad base", "full commit SHA", nil},
		{"archive root blocked by a file", "archive directory must be a real directory", func(t *testing.T, f fixture) {
			f.write(t, "docs/private", "a file where the archive directory goes\n")
		}},
		{"archive root not creatable", "create archive directory", func(t *testing.T, f fixture) {
			docs := filepath.Join(f.worktree, "docs")
			if err := os.Chmod(docs, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(docs, 0o755) })
		}},
		{"base inspection fails", "inspect " + unpublishedRecord + " at base: exit status 1: injected git failure", func(t *testing.T, f fixture) {
			failGit(t, "ls-tree:*")
		}},
		{"record is a symlink", "unpublished explanation " + unpublishedRecord + " must be a regular non-symlink file", func(t *testing.T, f fixture) {
			if err := os.Remove(filepath.Join(f.worktree, unpublishedRecord)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../../../config/app.yaml", filepath.Join(f.worktree, unpublishedRecord)); err != nil {
				t.Fatal(err)
			}
		}},
		{"record too large", "verify unpublished explanation " + unpublishedRecord, func(t *testing.T, f fixture) {
			if err := os.Truncate(filepath.Join(f.worktree, unpublishedRecord), maxArtifactBytes+1); err != nil {
				t.Fatal(err)
			}
		}},
		{"archive slot taken", "archive destination " + archivedRecordPath() + " already exists", func(t *testing.T, f fixture) {
			f.write(t, archivedRecordPath(), "earlier archive\n")
		}},
		{"archive slot unreadable", "inspect archive destination " + archivedRecordPath(), func(t *testing.T, f fixture) {
			stubLstat(t, func(p string) (os.FileInfo, error) {
				if strings.HasSuffix(filepath.ToSlash(p), archivedRecordPath()) {
					return nil, &fs.PathError{Op: "lstat", Path: p, Err: syscall.EACCES}
				}
				return os.Lstat(p)
			})
		}},
		{"archive directory read-only", "archive unpublished explanation " + unpublishedRecord, func(t *testing.T, f fixture) {
			dir := filepath.Join(f.worktree, filepath.FromSlash(filepath.Dir(archivedRecordPath())))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		}},
		{"record unreadable", "inspect unpublished explanation " + unpublishedRecord, func(t *testing.T, f fixture) {
			stubLstat(t, func(p string) (os.FileInfo, error) {
				if strings.HasSuffix(filepath.ToSlash(p), "/"+unpublishedRecord) {
					return nil, &fs.PathError{Op: "lstat", Path: p, Err: syscall.EACCES}
				}
				return os.Lstat(p)
			})
		}},
		{"staging fails", "stage unpublished explanation archive: exit status 1: injected git failure", func(t *testing.T, f fixture) {
			failGit(t, "add:*")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.write(t, unpublishedRecord, "unshipped explanation\n")
			f.git(t, "add", "-A")
			f.git(t, "commit", "-q", "-m", "failed build snapshot")
			base := f.base
			if tc.arrange == nil {
				base = "HEAD"
			} else {
				tc.arrange(t, f)
			}

			got, err := ArchiveUnpublishedContinuationRecords(context.Background(), f.worktree, base)

			if got != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ArchiveUnpublishedContinuationRecords = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestArchiveUnpublishedContinuationRecords_ARecordVanishingMidSweepIsSkipped(t *testing.T) {
	f := newFixture(t)
	f.write(t, unpublishedRecord, "unshipped explanation\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "failed build snapshot")
	stubLstat(t, func(p string) (os.FileInfo, error) {
		if strings.HasSuffix(filepath.ToSlash(p), "/"+unpublishedRecord) {
			return nil, &fs.PathError{Op: "lstat", Path: p, Err: fs.ErrNotExist}
		}
		return os.Lstat(p)
	})

	got, err := ArchiveUnpublishedContinuationRecords(context.Background(), f.worktree, f.base)

	if got != nil || err != nil {
		t.Fatalf("ArchiveUnpublishedContinuationRecords = %q, %v; want nil, nil", got, err)
	}
	if body, readErr := os.ReadFile(filepath.Join(f.worktree, unpublishedRecord)); readErr != nil || string(body) != "unshipped explanation\n" {
		t.Fatalf("vanished record was touched: %q, %v", body, readErr)
	}
	if staged := f.git(t, "diff", "--cached", "--name-only", "HEAD"); staged != "" {
		t.Fatalf("index changed: %q", staged)
	}
}

func TestArchiveUnpublishedContinuationRecords_ArchivesEveryNewRecordAndStagesTheMove(t *testing.T) {
	f := newFixture(t)
	second := "docs/explain/builds/cycle-40-other-run.md"
	f.write(t, unpublishedRecord, "one\n")
	f.write(t, second, "two\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "failed build snapshot")

	got, err := ArchiveUnpublishedContinuationRecords(context.Background(), f.worktree, f.base)

	dir := filepath.ToSlash(filepath.Join(archiveDateDir(), "unshipped-build-explanations"))
	want := []string{dir + "/cycle-40-other-run.md", dir + "/cycle-41-old-run.md"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("ArchiveUnpublishedContinuationRecords = %q, %v; want %q", got, err, want)
	}
	staged := f.git(t, "diff", "--cached", "--no-renames", "--name-status", "HEAD")
	wantStaged := "D\t" + second + "\nD\t" + unpublishedRecord + "\nA\t" + want[0] + "\nA\t" + want[1]
	if staged != wantStaged {
		t.Fatalf("staged = %q, want %q", staged, wantStaged)
	}
	if _, statErr := os.Stat(filepath.Join(f.worktree, unpublishedRecord)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("canonical record still present: %v", statErr)
	}
}

func TestLineageHolds_ANonAncestorOrUnknownAuthoredCommitNeverHolds(t *testing.T) {
	f := newFixture(t)
	f.write(t, "peer/only.txt", "peer\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "peer")
	peer := f.git(t, "rev-parse", "HEAD")

	reversed, revErr := lineageHolds(context.Background(), f.worktree, peer, f.base, nil)
	unknown, unkErr := lineageHolds(context.Background(), f.worktree, zeroSHA, peer, nil)

	if reversed || revErr != nil {
		t.Fatalf("lineageHolds(peer→base) = %v, %v; want false, nil", reversed, revErr)
	}
	if unknown || unkErr == nil || !strings.Contains(unkErr.Error(), "git merge-base --is-ancestor "+zeroSHA) {
		t.Fatalf("lineageHolds(unknown) = %v, %v; want false and the merge-base error", unknown, unkErr)
	}
}
