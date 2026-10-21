package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func commitPath(t *testing.T, r *gittest.Repo, rel, body string) {
	t.Helper()
	path := filepath.Join(r.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", rel)
	r.Git("commit", "-q", "-m", "add "+rel)
}

func laneOnOrigin(t *testing.T, runtime *gittest.Repo) string {
	t.Helper()
	worktree := filepath.Join(t.TempDir(), "lane")
	runtime.Git("worktree", "add", "-q", "-b", "cycle-7", worktree, "origin/main")
	if err := os.WriteFile(filepath.Join(worktree, "lane.txt"), []byte("lane"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, worktree, "add", "lane.txt")
	gitIn(t, worktree, "commit", "-q", "-m", "lane work")
	return worktree
}

func TestRebaseCycleBranchOntoMain_AnUnpushedCodeCommitOnLocalMainIsNotALandedPeer(t *testing.T) {
	_, runtime := startRefFixture(t)
	writeCommit(t, runtime, "stranded.go", "package x")
	stranded := runtime.Git("rev-parse", "HEAD")
	worktree := laneOnOrigin(t, runtime)

	out := captureStderr(t, func() {
		if ok, conflicts := rebaseCycleBranchOntoMain(context.Background(), runtime.Dir, worktree); ok || len(conflicts) != 0 {
			t.Errorf("rebase ok=%v conflicts=%v; want a refusal", ok, conflicts)
		}
	})

	if _, code, _ := gitCapture(context.Background(), worktree, "merge-base", "--is-ancestor", stranded, "HEAD"); code == 0 {
		t.Fatal("the fleet rebase put the unpushed commit under the lane, so the carry and ship treat it as a landed peer (cycle 1830)")
	}
	if !strings.Contains(out, stranded[:12]) {
		t.Errorf("stderr %q must name the unpushed commit", out)
	}
}

func TestRebaseCycleBranchOntoMain_BoundaryCommitsAndASyncMergeStillCarry(t *testing.T) {
	t.Parallel()
	seed, runtime := startRefFixture(t)
	commitPath(t, runtime, "knowledge-base/cycles/cycle-9.md", "closeout")
	writeCommit(t, seed, "peer.txt", "landed peer")
	seed.Git("push", "-q", "origin", "main")
	runtime.Git("fetch", "-q", "origin")
	runtime.Git("merge", "-q", "--no-edit", "origin/main")
	worktree := laneOnOrigin(t, runtime)

	if ok, conflicts := rebaseCycleBranchOntoMain(context.Background(), runtime.Dir, worktree); !ok || len(conflicts) != 0 {
		t.Fatalf("rebase ok=%v conflicts=%v; boundary commits and the sync merge are proven", ok, conflicts)
	}
	if got := gitIn(t, worktree, "rev-parse", "HEAD~1"); got != runtime.Git("rev-parse", "main") {
		t.Fatalf("the lane sits on %s, want the local main", got)
	}
}

func TestUnprovenAhead_AFaultIsNotAProof(t *testing.T) {
	t.Parallel()
	fault := errors.New("exec")
	script := func(revList string, revCode, ancestorCode int, ancestorErr error, diffCode int, paths ...string) gitFn {
		touched := "knowledge-base/cycles/c.md\x00"
		if len(paths) > 0 {
			touched = strings.Join(paths, "\x00") + "\x00"
		}
		return func(_ context.Context, _ string, args ...string) (string, int, error) {
			switch args[0] {
			case "rev-list":
				return revList, revCode, nil
			case "merge-base":
				return "", ancestorCode, ancestorErr
			default:
				return touched, diffCode, nil
			}
		}
	}
	for _, tc := range []struct {
		name    string
		git     gitFn
		want    []string
		wantErr bool
	}{
		{"an unreadable range", script("", 128, 0, nil, 0), nil, true},
		{"a merge of a commit origin holds", script("m p1 p2\n", 0, 0, nil, 0), nil, false},
		{"a merge of a commit origin lacks", script("m p1 p2\n", 0, 1, nil, 0), []string{"m"}, false},
		{"an unreadable ancestry", script("m p1 p2\n", 0, 0, fault, 0), nil, true},
		{"an unreadable commit diff", script("c p1\n", 0, 0, nil, 128), nil, true},
		{"a boundary commit", script("c p1\n", 0, 0, nil, 0), nil, false},
		{"an inbox stamp with a space in its name", script("c p1\n", 0, 0, nil, 0, ".evolve/inbox/a b.json"), nil, false},
		{"a closeout that also changes code", script("c p1\n", 0, 0, nil, 0, "knowledge-base/cycles/c.md", "go/x.go"), []string{"c"}, false},
		{"nothing ahead", script("\n", 0, 0, nil, 0), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := unprovenAhead(context.Background(), tc.git, "/p", "origin/main", "main")
			if (err != nil) != tc.wantErr || strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("unprovenAhead = %v, %v; want %v, err=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestOriginMainRef_NamesTheRemoteDefaultBranch(t *testing.T) {
	t.Parallel()
	script := func(url string, urlCode int, head string, headCode int) gitFn {
		return func(_ context.Context, _ string, args ...string) (string, int, error) {
			if args[0] == "config" {
				return url, urlCode, nil
			}
			return head, headCode, nil
		}
	}
	for _, tc := range []struct {
		name   string
		git    gitFn
		want   string
		wantOK bool
	}{
		{"no origin", script("", 1, "", 0), "", false},
		{"a blank origin url", script(" \n", 0, "", 0), "", false},
		{"origin with its HEAD", script("u\n", 0, "origin/trunk\n", 0), "origin/trunk", true},
		{"origin without a HEAD", script("u\n", 0, "", 1), "origin/main", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := originMainRef(context.Background(), tc.git, "/p")
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("originMainRef = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestRefuseUnprovenAhead_AnUnreadableRangeRefusesTooWithItsCause(t *testing.T) {
	t.Parallel()
	broken := func(context.Context, string, ...string) (string, int, error) { return "", 128, nil }
	if err := refuseUnprovenAhead(context.Background(), broken, "/p", "origin/main", "main"); err == nil || !strings.Contains(err.Error(), "rc=128") {
		t.Fatalf("err = %v; a range that cannot be read is no proof, so the base is refused with the git cause", err)
	}
}
