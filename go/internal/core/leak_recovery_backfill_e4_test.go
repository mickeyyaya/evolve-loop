package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func committedRepoE4(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	for rel, body := range files {
		writeFile(t, filepath.Join(r.Dir, rel), body)
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", "base")
	return r
}

func TestReadMainTreeStatus_AnUnlaunchableGitReturnsItsError(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gone")

	out, err := readMainTreeStatus(context.Background(), missing)

	if err == nil || out != "" || strings.Contains(err.Error(), "git status exit") {
		t.Fatalf("readMainTreeStatus = (%q, %v), want the launch error itself", out, err)
	}
}

func TestRecoverPath_AStagedAddOrDeleteIsDiscardedAndAnUnknownStatusAborts(t *testing.T) {
	cases := []struct {
		name          string
		xy            string
		arrange       func(r *gittest.Repo)
		wantRecovered bool
		wantLog       string
		check         func(t *testing.T, r *gittest.Repo)
	}{
		{"a staged add over a HEAD path is reset to HEAD", "A ", func(r *gittest.Repo) {
			writeFile(t, filepath.Join(r.Dir, "tracked.txt"), "leaked edit\n")
		}, true, "discarded leaked main-tree change tracked.txt", func(t *testing.T, r *gittest.Repo) {
			if body, err := os.ReadFile(filepath.Join(r.Dir, "tracked.txt")); err != nil || string(body) != "base\n" {
				t.Fatalf("the leaked content must be reset to HEAD: %q %v", body, err)
			}
		}},
		{"a staged new file has no HEAD copy and is not recovered", "A ", func(r *gittest.Repo) {
			r.Git("rm", "-q", "--cached", "tracked.txt")
			r.Git("commit", "-q", "-m", "untrack")
			r.Git("add", "tracked.txt")
		}, false, "WARN build-leak-recover: git checkout HEAD -- tracked.txt: exit ", func(t *testing.T, r *gittest.Repo) {
			if got := r.Git("status", "--porcelain"); got != "A  tracked.txt" {
				t.Fatalf("an unrecovered add must stay staged for the abort: %q", got)
			}
		}},
		{"a staged delete", "D ", func(r *gittest.Repo) {
			r.Git("rm", "-q", "tracked.txt")
		}, true, "discarded leaked main-tree change tracked.txt", func(t *testing.T, r *gittest.Repo) {
			if body, err := os.ReadFile(filepath.Join(r.Dir, "tracked.txt")); err != nil || string(body) != "base\n" {
				t.Fatalf("the deleted file must be restored from HEAD: %q %v", body, err)
			}
		}},
		{"a type change", "T ", func(*gittest.Repo) {}, false, `unrecoverable leak status "T " for tracked.txt`, func(*testing.T, *gittest.Repo) {}},
		{"an unmerged path", "UU", func(*gittest.Repo) {}, false, `unrecoverable leak status "UU" for tracked.txt`, func(*testing.T, *gittest.Repo) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := committedRepoE4(t, map[string]string{"tracked.txt": "base\n"})
			tc.arrange(r)
			rec := leakRecovery{projectRoot: r.Dir, worktree: t.TempDir(), cycle: 4, authority: sourceWriterAuthority}

			var stage, recovered bool
			log := captureStderr(t, func() { stage, recovered = rec.recoverPath(context.Background(), tc.xy, "tracked.txt") })

			if stage || recovered != tc.wantRecovered {
				t.Fatalf("recoverPath = (%v, %v), want (false, %v)", stage, recovered, tc.wantRecovered)
			}
			if !strings.Contains(log, tc.wantLog) {
				t.Fatalf("stderr lacks %q:\n%s", tc.wantLog, log)
			}
			tc.check(t, r)
		})
	}
}

func TestRecoverUntracked_AVanishedLeakCannotBeRelocated(t *testing.T) {
	rec := leakRecovery{projectRoot: t.TempDir(), worktree: t.TempDir(), cycle: 4, authority: sourceWriterAuthority}

	var stage, recovered bool
	log := captureStderr(t, func() { stage, recovered = rec.recoverUntracked("go/p/gone.go") })

	if stage || recovered {
		t.Fatalf("recoverUntracked = (%v, %v), want (false, false)", stage, recovered)
	}
	if !strings.Contains(log, "WARN build-leak-recover: relocate go/p/gone.go: ") {
		t.Fatalf("stderr must name the failed relocation:\n%s", log)
	}
}

func TestSetAsideShadowedLeak_AnUnwritableQuarantineKeepsTheLeakInPlace(t *testing.T) {
	root, wt := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve"), "a file where the quarantine home belongs")
	src, dst := filepath.Join(root, "a.go"), filepath.Join(wt, "a.go")
	writeFile(t, src, "main-tree copy\n")
	writeFile(t, dst, "worktree copy\n")
	rec := leakRecovery{projectRoot: root, worktree: wt, cycle: 9}

	var ok bool
	log := captureStderr(t, func() { ok = rec.setAsideShadowedLeak(src, dst, "a.go") })

	if ok {
		t.Fatal("a leak that could not be quarantined must not count as recovered")
	}
	if !strings.Contains(log, "WARN build-leak-recover: quarantine leaked a.go: ") {
		t.Fatalf("stderr must name the failed quarantine:\n%s", log)
	}
	if body, err := os.ReadFile(src); err != nil || string(body) != "main-tree copy\n" {
		t.Fatalf("the leak must stay in place: %q %v", body, err)
	}
	if body, err := os.ReadFile(dst); err != nil || string(body) != "worktree copy\n" {
		t.Fatalf("the worktree copy must be untouched: %q %v", body, err)
	}
}

func TestRemoveDuplicateLeak_AnUnremovableLeakIsNotRecovered(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.go")

	var ok bool
	log := captureStderr(t, func() { ok = removeDuplicateLeak(missing, "gone.go") })

	if ok || !strings.Contains(log, "WARN build-leak-recover: drop leaked gone.go: ") {
		t.Fatalf("removeDuplicateLeak = %v, stderr %q; want a loud refusal", ok, log)
	}
}

func TestSameBytes_ComparesContentAndReportsEitherUnreadableSide(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	writeFile(t, a, "ünïcødé\n")
	writeFile(t, b, "ünïcødé\n")
	writeFile(t, c, "ünïcødé \n")
	missing := filepath.Join(dir, "missing")
	cases := []struct {
		name     string
		left     string
		right    string
		wantSame bool
		wantErr  bool
	}{
		{"equal bytes", a, b, true, false},
		{"one byte differs", a, c, false, false},
		{"left unreadable", missing, a, false, true},
		{"right unreadable", a, missing, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			same, err := sameBytes(tc.left, tc.right)
			if same != tc.wantSame || (err != nil) != tc.wantErr {
				t.Fatalf("sameBytes = (%v, %v), want (%v, err=%v)", same, err, tc.wantSame, tc.wantErr)
			}
			if tc.wantErr && !os.IsNotExist(err) {
				t.Fatalf("err = %v, want the read's not-exist error", err)
			}
		})
	}
}

func TestRecoverTrackedEdit_AnEditThatVanishedFromTheMainTreeAborts(t *testing.T) {
	main := committedRepoE4(t, map[string]string{"tracked.txt": "base\n"})
	wt := committedRepoE4(t, map[string]string{"tracked.txt": "base\n"})
	if err := os.Remove(filepath.Join(main.Dir, "tracked.txt")); err != nil {
		t.Fatal(err)
	}
	rec := leakRecovery{projectRoot: main.Dir, worktree: wt.Dir, cycle: 4, authority: sourceWriterAuthority}

	var stage, recovered bool
	log := captureStderr(t, func() { stage, recovered = rec.recoverTrackedEdit(context.Background(), "tracked.txt") })

	if stage || recovered {
		t.Fatalf("recoverTrackedEdit = (%v, %v), want (false, false)", stage, recovered)
	}
	if !strings.Contains(log, "WARN build-leak-recover: relocate tracked edit tracked.txt: relocate content of tracked.txt: ") {
		t.Fatalf("stderr must name the failed relocation:\n%s", log)
	}
	if got := wt.Git("status", "--porcelain"); got != "" {
		t.Fatalf("the worktree must be untouched: %q", got)
	}
}

func TestDiscard_ANonRepositoryMainTreeIsNotRecovered(t *testing.T) {
	rec := leakRecovery{projectRoot: t.TempDir(), cycle: 4}

	var ok bool
	log := captureStderr(t, func() { ok = rec.discard(context.Background(), "x.go", "main-tree change") })

	if ok || !strings.Contains(log, "WARN build-leak-recover: git checkout HEAD -- x.go: exit ") {
		t.Fatalf("discard = %v, stderr %q; want a loud refusal", ok, log)
	}
}

func TestStageRelocated_NothingToStageIsTrueAndAFailedAddIsFalse(t *testing.T) {
	wt := committedRepoE4(t, map[string]string{"a.txt": "a\n"})
	if !stageRelocated(context.Background(), t.TempDir(), nil) {
		t.Fatal("no relocated paths must succeed without running git")
	}
	writeFile(t, filepath.Join(wt.Dir, "moved.txt"), "relocated\n")
	if !stageRelocated(context.Background(), wt.Dir, []string{"moved.txt"}) {
		t.Fatal("control: a relocated path must stage")
	}
	if got := wt.Git("diff", "--cached", "--name-only"); got != "moved.txt" {
		t.Fatalf("staged = %q, want moved.txt", got)
	}

	var ok bool
	log := captureStderr(t, func() { ok = stageRelocated(context.Background(), wt.Dir, []string{"never-written.txt"}) })

	if ok || !strings.Contains(log, "git add of relocated paths failed (rc=128)") {
		t.Fatalf("stageRelocated = %v, stderr %q; want a refusal naming git's exit", ok, log)
	}
}
