package cyclesimulator

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC) }

func TestRunGit_SuccessInRealRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	got := runGit(dir, "rev-parse", "--is-inside-work-tree")
	if got != "true" {
		t.Errorf("runGit success branch: got %q, want \"true\"", got)
	}
}

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

func TestAppendSimLedger_MkdirFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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

func TestAppendSimLedger_OpenFileErr(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ledgerPath := filepath.Join(root, ".evolve", "ledger.jsonl")
	if err := os.MkdirAll(ledgerPath, 0o755); err != nil {
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

func TestRun_WriteAndLedgerFailBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(t *testing.T, root, ws string)
	}{
		{
			name: "artifact-write",
			setup: func(t *testing.T, _, ws string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(ws, "intent.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "ledger-append",
			setup: func(t *testing.T, root, _ string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "retro-write",
			setup: func(t *testing.T, _, ws string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(ws, "retrospective-report.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
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

func TestAppendSimLedger_TipWriteFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
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
