package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestResolveEvolveBinForRollback_TheEnvironmentNamesNoBinary(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-evolve")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_GO_BIN", binPath)
	t.Setenv("PATH", "/nonexistent/path")
	if got := resolveEvolveBinForRollback(t.TempDir()); got != "" {
		t.Errorf("got %q, want empty: EVOLVE_GO_BIN is not read", got)
	}
}

func installRepoEvolve(t *testing.T, root, script string) {
	t.Helper()
	bin := filepath.Join(root, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeclitest.Install(t, bin, script)
}

func TestResolveEvolveBinForRollback_RepoRootCandidate(t *testing.T) {
	repo := t.TempDir()
	candidate := filepath.Join(repo, "go", "bin", "evolve")
	if err := os.MkdirAll(filepath.Dir(candidate), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveEvolveBinForRollback(repo); got != candidate {
		t.Errorf("got %q, want %q", got, candidate)
	}
}

func TestResolveEvolveBinForRollback_NotFound_ReturnsEmpty(t *testing.T) {
	t.Setenv("PATH", "/nonexistent/path")
	if got := resolveEvolveBinForRollback("/no/such/repo/root"); got != "" {
		t.Errorf("got %q, want empty (no binary anywhere)", got)
	}
}

func TestAppendLedger_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "ledger.jsonl")
	if err := appendLedger(path, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{\"a\":1}\n" {
		t.Errorf("got %q", string(b))
	}
}

func TestAppendLedger_AppendsMultiple(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	_ = appendLedger(path, []byte(`one`))
	_ = appendLedger(path, []byte(`two`))
	b, _ := os.ReadFile(path)
	if string(b) != "one\ntwo\n" {
		t.Errorf("got %q, want 'one\\ntwo\\n'", string(b))
	}
}

func TestAppendLedger_MkdirFailure_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "child", "ledger.jsonl")
	err := appendLedger(path, []byte("data"))
	if err == nil {
		t.Fatal("expected mkdir error when parent is a file")
	}
	if !strings.Contains(err.Error(), blocker) {
		t.Errorf("error %q does not name the blocking path %q", err, blocker)
	}
	if _, statErr := os.Stat(filepath.Join(blocker, "child")); statErr == nil {
		t.Error("child directory was created under a regular file")
	}
}

func TestRevertAndShipWith_RevertOK_BinaryFails_LocalOnly(t *testing.T) {
	binDir := t.TempDir()
	installRepoEvolve(t, binDir, "#!/bin/sh\nexit 1\n")

	fake := &fixtures.FakeExec{}
	g := gitexec.Git{Dir: t.TempDir(), Exec: fake.Run}

	got := revertAndShipWith(g, binDir, "deadbeef", "test-reason", "1.0.0")
	if got != "local-only" {
		t.Errorf("status = %q, want local-only (binary exits 1)", got)
	}
}

func TestRevertAndShipWith_RevertOK_BinarySucceeds_Reverted(t *testing.T) {
	binDir := t.TempDir()
	installRepoEvolve(t, binDir, "#!/bin/sh\nexit 0\n")

	fake := &fixtures.FakeExec{}
	g := gitexec.Git{Dir: t.TempDir(), Exec: fake.Run}

	got := revertAndShipWith(g, binDir, "deadbeef", "test-reason", "1.0.0")
	if got != "reverted" {
		t.Errorf("status = %q, want reverted (binary exits 0)", got)
	}
}

func TestRun_NilSteps_FallbacksAssigned(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	fakes := installFakeTools(t,
		"case \"$2\" in view) pwd >> \"$FAKE_GH_PWD\"; exit 0;; *) exit 0;; esac\n",
		"case \"$1\" in ls-remote) echo refs/tags/v1.2.3;; esac\nexit 0\n")
	var buf strings.Builder
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       Steps{},
		Stderr:      &buf,
		Now:         func() time.Time { return time.Unix(0, 0) },
	})
	if res.ReleaseDelete != "deleted" || res.TagDelete != "deleted" {
		t.Errorf("fallback steps not wired: release=%q tag=%q", res.ReleaseDelete, res.TagDelete)
	}
	if res.Revert == "" {
		t.Error("RevertAndShip fallback did not run")
	}
	if err != nil && !errors.Is(err, ErrPartial) {
		t.Errorf("unexpected error kind: %v", err)
	}
	if got := fakes.ghWorkingDirs(t); len(got) == 0 {
		t.Error("fake gh was never invoked; the default release step is not hermetic")
	}
}

func TestRevertAndShipWith_ShipsWithAnAuditedAutoConfirmReason(t *testing.T) {
	binDir := t.TempDir()
	record := filepath.Join(t.TempDir(), "args")
	installRepoEvolve(t, binDir, "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+record+"'\n")
	fake := &fixtures.FakeExec{}
	g := gitexec.Git{Dir: t.TempDir(), Exec: fake.Run}

	if got := revertAndShipWith(g, binDir, "deadbeef", "bad build", "1.0.0"); got != "reverted" {
		t.Fatalf("status = %q, want reverted", got)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "--auto-confirm=rollback of v1.0.0: bad build\n") {
		t.Errorf("ship args = %q, want the --auto-confirm reason", raw)
	}
}
