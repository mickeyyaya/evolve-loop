package inboxmover

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func cycleWorktreeOf(t *testing.T, plane string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "cycle-9")
	if err := os.MkdirAll(filepath.Join(wt, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"state.json", "ledger.jsonl"} {
		target := filepath.Join(plane, ".evolve", f)
		if err := os.WriteFile(target, []byte("{}\n"), 0o644); err != nil && f == "state.json" {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(wt, ".evolve", f)); err != nil {
			t.Fatal(err)
		}
	}
	return wt
}

func TestMutators_RefuseACycleWorktreeInboxCopyAndNameThePlane(t *testing.T) {
	plane := makeRepo(t)
	dropInboxFile(t, plane, "task-1.json", "task-1")
	wt := cycleWorktreeOf(t, plane)
	wtCopy := dropInboxFile(t, wt, "task-1.json", "task-1")
	dropProcessingFile(t, wt, "9", "task-2.json", "task-2")
	ledgerBefore, _ := os.ReadFile(filepath.Join(plane, ".evolve", "ledger.jsonl"))
	opts := Options{ProjectRoot: wt}

	calls := map[string]func() error{
		"Claim":             func() error { _, err := Claim(opts, "task-1", "9"); return err },
		"ClaimDispatchable": func() error { _, err := ClaimDispatchable(opts, "task-1", "9"); return err },
		"ClaimPending":      func() error { return ClaimPending(opts, 9, []string{"task-1"}) },
		"Promote":           func() error { _, err := Promote(opts, "task-2", "consumed", PromoteOpts{}); return err },
		"ReconcileSuperseded": func() error {
			_, err := ReconcileSuperseded(opts, []string{"task-2"}, "superseded", PromoteOpts{})
			return err
		},
		"ReleaseFromQuarantine":  func() error { _, err := ReleaseFromQuarantine(opts, "task-1"); return err },
		"RecoverOrphans":         func() error { _, err := RecoverOrphans(opts); return err },
		"ReleaseCycleProcessing": func() error { _, err := ReleaseCycleProcessing(opts, 9); return err },
		"ReleaseClaim":           func() error { _, err := ReleaseClaim(opts, "task-2", "stale"); return err },
		"AbsorbRootCopies":       func() error { _, err := AbsorbRootCopies(opts); return err },
		"RouteLane":              func() error { _, err := RouteLane(opts, "task-1", "r"); return err },
		"RouteConsole":           func() error { _, err := RouteConsole(opts, "task-1", "r", 9); return err },
		"File":                   func() error { _, err := File(opts, []byte(`{"id":"task-3"}`)); return err },
		"Edit":                   func() error { _, err := Edit(opts, "task-1", nil); return err },
		"EditHeld":               func() error { _, err := EditHeld(opts, "task-1", 9, nil); return err },
		"Withdraw":               func() error { _, err := Withdraw(opts, "task-1", "r"); return err },
		"VerifyPremise":          func() error { _, err := VerifyPremise(opts, "task-1", "e"); return err },
		"ReleaseContinuationBinding": func() error {
			_, _, err := ReleaseContinuationBinding(opts, "task-1", "r", "test")
			return err
		},
		"RetireUnbacked": func() error { _, err := RetireUnbacked(opts, 9, "consumed", "r", "", []string{"task-2"}); return err },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call()
			if err == nil || !strings.Contains(err.Error(), "cycle worktree") || !strings.Contains(err.Error(), plane) {
				t.Fatalf("err = %v, want a refusal that names the cycle worktree and the plane root %s", err, plane)
			}
		})
	}
	if _, err := os.Stat(wtCopy); err != nil {
		t.Errorf("the worktree copy moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(plane, ".evolve", "inbox", "task-1.json")); err != nil {
		t.Errorf("the plane item moved: %v", err)
	}
	if after, _ := os.ReadFile(filepath.Join(plane, ".evolve", "ledger.jsonl")); string(after) != string(ledgerBefore) {
		t.Errorf("a refusal wrote the plane ledger:\n%s", after)
	}
}

func TestMutators_AnExplicitPlaneInboxFromACycleWorktreeIsAllowed(t *testing.T) {
	plane := makeRepo(t)
	dropInboxFile(t, plane, "task-1.json", "task-1")
	wt := cycleWorktreeOf(t, plane)
	res, err := Claim(Options{ProjectRoot: wt, InboxDir: filepath.Join(plane, ".evolve", "inbox")}, "task-1", "9")
	if err != nil {
		t.Fatalf("a claim on the plane inbox must pass: %v", err)
	}
	if !strings.HasPrefix(res.DestPath, plane) {
		t.Errorf("DestPath = %q, want under the plane %s", res.DestPath, plane)
	}
}

func TestReadFailureCount_ReadsAWorktreeCopyWithoutRefusal(t *testing.T) {
	plane := makeRepo(t)
	wt := cycleWorktreeOf(t, plane)
	dropInboxFile(t, wt, "task-1.json", "task-1")
	if n, ok := ReadFailureCount(Options{ProjectRoot: wt}, "task-1"); !ok || n != 0 {
		t.Errorf("ReadFailureCount = (%d, %v), want (0, true): a read moves nothing", n, ok)
	}
}

func TestCycleWorktreeRefusal_ARelativeStateLinkResolvesThePlane(t *testing.T) {
	base := t.TempDir()
	plane := filepath.Join(base, "plane")
	wtEvolve := filepath.Join(base, "wt", ".evolve")
	for _, d := range []string{filepath.Join(plane, ".evolve"), filepath.Join(wtEvolve, "inbox")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rel, err := filepath.Rel(wtEvolve, filepath.Join(plane, ".evolve", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, filepath.Join(wtEvolve, "state.json")); err != nil {
		t.Fatal(err)
	}
	err = cycleWorktreeRefusal(filepath.Join(wtEvolve, "inbox"))
	if !errors.Is(err, errCycleWorktreeInbox) || !strings.Contains(err.Error(), "EVOLVE_PROJECT_ROOT="+plane) {
		t.Fatalf("err = %v, want the refusal naming the plane %s", err, plane)
	}
	if err := cycleWorktreeRefusal(filepath.Join(plane, ".evolve", "inbox")); err != nil {
		t.Errorf("a plane inbox with a regular or absent state.json is no cycle worktree: %v", err)
	}
}

func TestReconcileSuperseded_SkipsABlankIDAndStopsAtTheFirstFailedPromote(t *testing.T) {
	repo := makeRepo(t)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	retired, err := ReconcileSuperseded(Options{ProjectRoot: repo}, []string{"", "task-1"}, "no-such-state", PromoteOpts{})
	if !errors.Is(err, ErrBadState) || !strings.Contains(err.Error(), `promote "task-1"`) || len(retired) != 0 {
		t.Fatalf("retired = %v, err = %v, want the blank id skipped and the bad state of task-1 reported", retired, err)
	}
}

func TestCycleWorktreeRefusal_AnUnreadableStateLinkRefuses(t *testing.T) {
	evolve := filepath.Join(t.TempDir(), ".evolve")
	if err := os.MkdirAll(evolve, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nowhere/state.json", filepath.Join(evolve, "state.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(evolve, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(evolve, 0o755) })
	err := cycleWorktreeRefusal(filepath.Join(evolve, "inbox"))
	if !errors.Is(err, errCycleWorktreeInbox) || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want a refusal that carries the permission error", err)
	}
}

func TestReleaseContinuationBinding_TheBranchesBesideTheRefusal(t *testing.T) {
	bound := continuation.Continuation{SnapshotSHA: "snap7", Cycle: 7}

	t.Run("an unreadable registry is an error", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(continuation.RegistryPath(root), []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, released, err := ReleaseContinuationBinding(Options{ProjectRoot: root}, "s", "r", "test")
		if err == nil || released || !strings.Contains(err.Error(), "registry unreadable") {
			t.Fatalf("released = %v, err = %v, want the registry read error", released, err)
		}
	})

	t.Run("a bound scope with no item file still releases and says so", func(t *testing.T) {
		root := t.TempDir()
		releaseFixtureBind(t, root, "s", bound)
		var stderr strings.Builder
		_, released, err := ReleaseContinuationBinding(Options{ProjectRoot: root, Stderr: &stderr}, "s", "r", "test")
		if err != nil || !released || !strings.Contains(stderr.String(), "no item file found") {
			t.Fatalf("released = %v, err = %v, stderr = %q", released, err, stderr.String())
		}
	})

	t.Run("an item file that cannot take the pointer still releases and says so", func(t *testing.T) {
		root := t.TempDir()
		inbox := filepath.Join(root, ".evolve", "inbox")
		releaseFixtureItem(t, inbox, "s")
		releaseFixtureBind(t, root, "s", bound)
		if err := os.Chmod(inbox, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
		var stderr strings.Builder
		_, released, err := ReleaseContinuationBinding(Options{ProjectRoot: root, Stderr: &stderr}, "s", "r", "test")
		if err != nil || !released || !strings.Contains(stderr.String(), "NOT written") {
			t.Fatalf("released = %v, err = %v, stderr = %q", released, err, stderr.String())
		}
	})
}

func TestCycleWorktreeRefusal_AnUncleanInboxPathStillRefuses(t *testing.T) {
	plane := makeRepo(t)
	wt := cycleWorktreeOf(t, plane)
	wtCopy := dropInboxFile(t, wt, "task-1.json", "task-1")
	inbox := filepath.Join(wt, ".evolve", "inbox")
	for _, dir := range []string{inbox + "/", inbox + "/.", inbox + "//"} {
		_, err := Claim(Options{ProjectRoot: wt, InboxDir: dir}, "task-1", "9")
		if !errors.Is(err, errCycleWorktreeInbox) {
			t.Errorf("InboxDir %q: err = %v, want the cycle-worktree refusal", dir, err)
		}
	}
	if _, err := os.Stat(wtCopy); err != nil {
		t.Errorf("the worktree copy moved: %v", err)
	}
}
