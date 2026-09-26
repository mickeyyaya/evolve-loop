package recoveryguard

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// worktree is a repository with one committed source file, as a build leaves the change's worktree.
func worktree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "t@example.com")
	git(t, dir, "config", "user.name", "test")
	write(t, filepath.Join(dir, "fix.go"), "package fix\n")
	git(t, dir, "add", "fix.go")
	git(t, dir, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "change")
	return dir
}

// workspace is a run directory as a phase leaves it: its report, the host's proof-of-read token, the lane
// pin and a host log.
func workspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	write(t, filepath.Join(ws, "build-report.md"), "## Summary\n")
	write(t, filepath.Join(ws, "challenge-token.txt"), "2bb098b9e2c61eb0\n")
	write(t, filepath.Join(ws, "lane-scope.json"), `{"todo_ids":["one"]}`)
	write(t, filepath.Join(ws, "signals.ndjson"), "{\"seq\":1}\n")
	return ws
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// run begins a guard over scope, lets act play the agent, and ends it.
func run(t *testing.T, scope Scope, act func()) Outcome {
	t.Helper()
	g, err := Begin(context.Background(), scope)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	act()
	return g.End(context.Background())
}

func TestGuard_KeepsTheDeliverableTheAgentWasAskedToRepair(t *testing.T) {
	ws := workspace(t)
	report := filepath.Join(ws, "build-report.md")

	out := run(t, Scope{Workspace: ws, Allowed: []string{report}}, func() {
		write(t, report, "## Summary\n\n## Changes\n- fix.go\n")
	})

	if !out.Clean() {
		t.Fatalf("an in-grant repair must leave the guard clean, got %+v", out)
	}
	if read(t, report) != "## Summary\n\n## Changes\n- fix.go\n" {
		t.Fatal("the repaired deliverable was not kept")
	}
}

func TestGuard_RestoresProofOfReadTheAgentForged(t *testing.T) {
	ws := workspace(t)
	token := filepath.Join(ws, "challenge-token.txt")

	out := run(t, Scope{Workspace: ws, Allowed: []string{filepath.Join(ws, "build-report.md")}}, func() {
		write(t, token, "forged\n")
	})

	if out.Clean() || !slices.Contains(out.Restored, token) {
		t.Fatalf("a forged token must be reported as restored, got %+v", out)
	}
	if read(t, token) != "2bb098b9e2c61eb0\n" {
		t.Fatalf("the host's token was not restored: %q", read(t, token))
	}
}

func TestGuard_RemovesAVerdictTheAgentPlanted(t *testing.T) {
	ws := workspace(t)
	verdict := filepath.Join(ws, "acs-verdict.json")

	out := run(t, Scope{Workspace: ws}, func() { write(t, verdict, `{"red_count":0}`) })

	if !slices.Contains(out.Restored, verdict) {
		t.Fatalf("a planted verdict must be reported, got %+v", out)
	}
	if _, err := os.Stat(verdict); !os.IsNotExist(err) {
		t.Fatalf("a planted verdict must be removed, stat err = %v", err)
	}
}

func TestGuard_RestoresTheLanePinTheAgentRemoved(t *testing.T) {
	ws := workspace(t)
	pin := filepath.Join(ws, "lane-scope.json")

	out := run(t, Scope{Workspace: ws}, func() {
		if err := os.Remove(pin); err != nil {
			t.Fatal(err)
		}
	})

	if !slices.Contains(out.Restored, pin) || read(t, pin) != `{"todo_ids":["one"]}` {
		t.Fatalf("the removed lane pin must be put back and reported, got %+v", out)
	}
}

func TestGuard_LeavesTheDispatchesOwnTelemetryUnfenced(t *testing.T) {
	ws := workspace(t)
	signals := filepath.Join(ws, "signals.ndjson")
	log := filepath.Join(ws, "deliverable-recovery-interactions.ndjson")

	out := run(t, Scope{Workspace: ws, Unfenced: []string{signals}, UnfencedStems: []string{"deliverable-recovery-"}}, func() {
		write(t, signals, "{\"seq\":1}\n{\"seq\":2}\n")
		write(t, log, "{\"kind\":\"nudge\"}\n")
	})

	if !out.Clean() || !slices.Equal(out.Unfenced, []string{log}) {
		t.Fatalf("appended telemetry must not count as leaving the grant, and the generated log is reported, got %+v", out)
	}
	if read(t, signals) != "{\"seq\":1}\n{\"seq\":2}\n" || read(t, log) == "" {
		t.Fatal("unfenced telemetry was rolled back")
	}
}

func TestGuard_RestoresTheChangesWorktree(t *testing.T) {
	tree, ws := worktree(t), workspace(t)
	src := filepath.Join(tree, "fix.go")

	out := run(t, Scope{Worktree: tree, Workspace: ws}, func() { write(t, src, "package fix\n\nfunc Rewritten() {}\n") })

	if !slices.Contains(out.Restored, src) {
		t.Fatalf("a source edit must be reported as restored, got %+v", out)
	}
	if read(t, src) != "package fix\n" {
		t.Fatalf("the change's code was not restored: %q", read(t, src))
	}
}

// Without its kernel evidence the rung must not run at all.
func TestBegin_FailsClosed(t *testing.T) {
	for name, scope := range map[string]Scope{
		"no workspace":                     {},
		"a workspace that is absent":       {Workspace: filepath.Join(t.TempDir(), "missing")},
		"a worktree that is no repository": {Workspace: workspace(t), Worktree: t.TempDir()},
	} {
		if _, err := Begin(context.Background(), scope); err == nil {
			t.Errorf("%s must refuse the guard", name)
		}
	}
}

// A path the guard could not put back is an error the caller must treat as a failed rung.
func TestGuard_ReportsAFileItCouldNotPutBack(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	ws := workspace(t)
	pin := filepath.Join(ws, "lane-scope.json")

	out := run(t, Scope{Workspace: ws}, func() {
		if err := os.Remove(pin); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(ws, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	})

	if out.Err == nil || out.Clean() {
		t.Fatalf("an unrestorable file must surface as an error, got %+v", out)
	}
}

// A fenced file swapped for a link, a directory or a device is no longer the file: every later reader would
// follow the swap, so the swap is undone and reported like any other change.
func TestGuard_UndoesAFencedFileSwappedForALinkOrDirectory(t *testing.T) {
	ws := workspace(t)
	token := filepath.Join(ws, "challenge-token.txt")
	pin := filepath.Join(ws, "lane-scope.json")
	planted := filepath.Join(ws, "acs-verdict.json")

	out := run(t, Scope{Workspace: ws}, func() {
		if err := os.Remove(token); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(ws, "build-report.md"), token); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(pin); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(pin, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/etc/hosts", planted); err != nil {
			t.Fatal(err)
		}
	})

	for _, p := range []string{token, pin, planted} {
		if !slices.Contains(out.Restored, p) {
			t.Errorf("%s must be reported as restored, got %+v", p, out)
		}
	}
	if info, err := os.Lstat(token); err != nil || !info.Mode().IsRegular() || read(t, token) != "2bb098b9e2c61eb0\n" {
		t.Errorf("the token must be a regular file with the host's bytes again (err %v)", err)
	}
	if info, err := os.Lstat(pin); err != nil || !info.Mode().IsRegular() {
		t.Errorf("the lane pin must be a regular file again (err %v)", err)
	}
	if _, err := os.Lstat(planted); !os.IsNotExist(err) {
		t.Errorf("a planted link must be removed, lstat err = %v", err)
	}
}
