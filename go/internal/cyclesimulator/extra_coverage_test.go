package cyclesimulator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// fixedNow is a deterministic clock for ledger-entry tests.
func fixedNow() time.Time { return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC) }

// TestRunGit_SuccessInRealRepo covers runGit's success return — every other
// test runs against a non-git tempdir so git always errors and only the
// failure branch was exercised.
func TestRunGit_SuccessInRealRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	// --is-inside-work-tree succeeds right after init (no commit/author needed).
	got := runGit(dir, "rev-parse", "--is-inside-work-tree")
	if got != "true" {
		t.Errorf("runGit success branch: got %q, want \"true\"", got)
	}
}

// TestReadChainLink_WhitespaceOnlyFile covers the lines[0]=="" branch: a file
// of non-zero size that trims to empty (passes the Size()==0 guard).
func TestReadChainLink_WhitespaceOnlyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev, seq, err := readChainLink(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if prev != zeroSeed || seq != 0 {
		t.Errorf("whitespace-only: got prev=%s seq=%d, want zero/0", prev, seq)
	}
}

// TestReadChainLink_UnreadableFile covers the ReadFile error branch: a
// non-empty file that stat sees but ReadFile cannot open.
func TestReadChainLink_UnreadableFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, []byte(`{"x":1}`+"\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	_, _, err := readChainLink(path)
	if err == nil {
		t.Error("expected ReadFile error on unreadable non-empty ledger")
	}
}

// TestAppendSimLedger_MissingArtifact covers the artifactSHA="" branch: the
// artifact path does not exist, so ReadFile fails and the SHA stays empty but
// the entry is still written.
func TestAppendSimLedger_MissingArtifact(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
	err := appendSimLedger(ledgerPath, 1, "scout",
		filepath.Join(root, "does-not-exist.md"), "tok", root, fixedNow)
	if err != nil {
		t.Fatalf("missing artifact should not abort append: %v", err)
	}
	data, rerr := os.ReadFile(ledgerPath)
	if rerr != nil {
		t.Fatalf("ledger not written: %v", rerr)
	}
	if !bytes.Contains(data, []byte(`"artifact_sha256":""`)) {
		t.Errorf("missing-artifact entry should carry empty sha, got: %s", data)
	}
}

// TestAppendSimLedger_MkdirFails covers the MkdirAll error branch: the parent
// .evolve path is a regular file, so the ledger dir cannot be created.
func TestAppendSimLedger_MkdirFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Occupy .evolve with a file so MkdirAll(.evolve) fails with ENOTDIR.
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(root, "a.md")
	if err := os.WriteFile(art, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := appendSimLedger(filepath.Join(root, ".evolve", "ledger.jsonl"),
		1, "scout", art, "tok", root, fixedNow)
	if err == nil {
		t.Error("expected MkdirAll error when .evolve is a file")
	}
}

// TestAppendSimLedger_ReadChainLinkErr covers the readChainLink error branch:
// the existing ledger is non-empty but unreadable, so chain-link read fails.
func TestAppendSimLedger_ReadChainLinkErr(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ledgerPath := filepath.Join(evolveDir, "ledger.jsonl")
	if err := os.WriteFile(ledgerPath, []byte(`{"x":1}`+"\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ledgerPath, 0o644) })
	art := filepath.Join(root, "a.md")
	if err := os.WriteFile(art, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendSimLedger(ledgerPath, 1, "scout", art, "tok", root, fixedNow); err == nil {
		t.Error("expected readChainLink error on unreadable existing ledger")
	}
}

// TestAppendSimLedger_OpenFileErr covers the OpenFile error branch: the ledger
// path itself is a directory, so append-mode open fails.
func TestAppendSimLedger_OpenFileErr(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
	if err := os.MkdirAll(ledgerPath, 0o755); err != nil { // ledger.jsonl is a dir
		t.Fatal(err)
	}
	art := filepath.Join(root, "a.md")
	if err := os.WriteFile(art, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendSimLedger(ledgerPath, 1, "scout", art, "tok", root, fixedNow); err == nil {
		t.Error("expected OpenFile error when ledger path is a directory")
	}
}

// TestRun_WriteAndLedgerFailBranches covers four Run error branches that share
// identical stubs and differ only in how they block a write path:
//   - artifact-write:        intent.md is a directory → first phase WriteFile fails
//   - ledger-append:         .evolve is a file → MkdirAll for the ledger dir fails
//   - retro-write:           retrospective-report.md is a directory → retro WriteFile fails
//   - simulator-report-write: simulator-report.md is a directory → final WriteFile fails
func TestRun_WriteAndLedgerFailBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, root, ws string)
	}{
		{
			// workspace pre-exists with intent.md as a dir → WriteFile to that path fails.
			name: "artifact-write",
			setup: func(t *testing.T, _, ws string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(ws, "intent.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			// .evolve is a regular file → MkdirAll(.evolve) inside appendSimLedger fails.
			name: "ledger-append",
			setup: func(t *testing.T, root, _ string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			// four pipeline phases succeed; retro artifact path is a dir → WriteFile fails.
			name: "retro-write",
			setup: func(t *testing.T, _, ws string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(ws, "retrospective-report.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			// all six phases succeed; simulator-report path is a dir → final WriteFile fails.
			name: "simulator-report-write",
			setup: func(t *testing.T, _, ws string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(ws, "simulator-report.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			ws := filepath.Join(root, "ws")
			tc.setup(t, root, ws)
			var stderr bytes.Buffer
			rc := Run(Inputs{
				Cycle:        1,
				Workspace:    ws,
				ProjectRoot:  root,
				AdvanceFn:    func(string, string) error { return nil },
				ShipDryRunFn: func(string) (int, error) { return 0, nil },
				VerifyFn:     func() error { return nil },
			}, &stderr)
			if rc != ExitRuntimeErr {
				t.Errorf("rc=%d, want %d (log=%s)", rc, ExitRuntimeErr, stderr.String())
			}
		})
	}
}

// TestAppendSimLedger_TipWriteFails covers the ledger.tip temp-write error
// branch: the ledger line is appended, then the tip temp path is a directory
// so the atomic tip write fails.
func TestAppendSimLedger_TipWriteFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Occupy the tip temp path with a directory so WriteFile(tmp) fails.
	if err := os.MkdirAll(filepath.Join(evolveDir, "ledger.tip.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(root, "a.md")
	if err := os.WriteFile(art, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := appendSimLedger(filepath.Join(evolveDir, "ledger.jsonl"),
		1, "scout", art, "tok", root, fixedNow)
	if err == nil {
		t.Error("expected tip temp-write error when ledger.tip.tmp is a directory")
	}
}
