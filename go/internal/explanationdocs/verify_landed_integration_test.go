//go:build integration

package explanationdocs

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func landedFixture(t *testing.T) (fixture, CycleBinding, string) {
	t.Helper()
	f := newFixture(t)
	if err := os.WriteFile(filepath.Join(f.worktree, ".git", "info", "exclude"), []byte(".evolve/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.workspace = filepath.Join(f.worktree, ".evolve", "runs", "cycle-42")
	binding := f.binding()
	binding.ProjectRoot = f.worktree
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	f.prepareRequired(t)
	if failures := CheckBuild(context.Background(), binding); len(failures) != 0 {
		t.Fatalf("CheckBuild: %v", failures)
	}
	if err := SealResult(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	f.git(t, "add", "config/app.yaml", cycleDocumentPath(f.cycle, f.runID))
	f.git(t, "commit", "-q", "-m", "land Build explanation")
	return f, binding, f.git(t, "rev-parse", "HEAD")
}

func worktreeCount(t *testing.T, f fixture) int {
	t.Helper()
	f.git(t, "worktree", "prune")
	return strings.Count(f.git(t, "worktree", "list", "--porcelain"), "worktree ")
}

func TestVerifyLanded_InactiveOrUnresolvableBindingNeverTouchesGit(t *testing.T) {
	failGit(t, "*")
	for _, tc := range []struct {
		name       string
		binding    CycleBinding
		wantActive bool
		wantErr    string
	}{
		{"no project root", CycleBinding{Cycle: 42}, false, ""},
		{"no cycle", CycleBinding{ProjectRoot: t.TempDir()}, false, ""},
		{"contract without run id", CycleBinding{ProjectRoot: t.TempDir(), Cycle: 42, ContractVersion: CurrentContractVersion}, true, "active contract requires project root, positive cycle, and run ID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view, active, err := VerifyLanded(context.Background(), tc.binding, zeroSHA)

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if view != nil || active != tc.wantActive || gotErr != tc.wantErr {
				t.Fatalf("VerifyLanded = %v, %v, %q; want nil, %v, %q", view, active, gotErr, tc.wantActive, tc.wantErr)
			}
		})
	}
}

func TestVerifyLanded_RefusesALandedCommitThatIsNotHead(t *testing.T) {
	f, binding, _ := landedFixture(t)

	view, active, err := VerifyLanded(context.Background(), binding, f.base)

	if view != nil || !active || err == nil || err.Error() != "landed commit does not match ProjectRoot HEAD" {
		t.Fatalf("VerifyLanded = %v, %v, %v; want the HEAD mismatch", view, active, err)
	}
}

func TestVerifyLanded_PlumbingFailuresAreActiveErrorsAndLeaveNoWorktree(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		arrange    func(t *testing.T)
	}{
		{"temp root unavailable", "create landed verification path", func(t *testing.T) {
			t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
		}},
		{"temp root not removable", "prepare landed verification path: busy", func(t *testing.T) {
			swapSeam(t, &removePath, func(string) error { return errors.New("busy") })
		}},
		{"worktree add fails", "create landed verification worktree: exit status 1: injected git failure", func(t *testing.T) {
			failGit(t, "worktree:add")
		}},
		{"worktree remove fails", "remove landed verification worktree: exit status 1: injected git failure", func(t *testing.T) {
			failGit(t, "worktree:remove")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, binding, landed := landedFixture(t)
			tc.arrange(t)

			view, active, err := VerifyLanded(context.Background(), binding, landed)

			if view != nil || !active || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("VerifyLanded = %v, %v, %v; want an active error naming %q", view, active, err, tc.want)
			}
			if tc.name != "worktree remove fails" {
				if n := worktreeCount(t, f); n != 1 {
					t.Fatalf("worktrees after the failure = %d, want only the main one", n)
				}
			}
		})
	}
}

func TestVerifyLanded_APartialWorktreeAddLeavesNoVerificationDirectory(t *testing.T) {
	_, binding, landed := landedFixture(t)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"$3:$4\" = \"worktree:add\" ]; then mkdir -p \"$7\"; echo partial > \"$7/half\"; echo 'add died midway' >&2; exit 1; fi\nexec '" + gitPath + "' \"$@\"\n"
	fakeclitest.Install(t, filepath.Join(bin, "git"), script)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	view, active, verr := VerifyLanded(context.Background(), binding, landed)

	if view != nil || !active || verr == nil || !strings.Contains(verr.Error(), "add died midway") {
		t.Fatalf("VerifyLanded = %v, %v, %v; want the worktree add failure", view, active, verr)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("verification directory left behind: %v", left)
	}
}
