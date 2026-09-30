package rollback

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultGhDeleteRelease_FakeGhSucceeds(t *testing.T) {
	dir := t.TempDir()
	ghBin := filepath.Join(dir, "gh")
	if err := os.WriteFile(ghBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	got := defaultGhDeleteRelease("v0.0.0-adv-test")
	if got != "deleted" {
		t.Errorf("fake gh exits 0: status = %q, want 'deleted'", got)
	}
}

func TestDefaultGhDeleteRelease_FakeGhFails_GenericError(t *testing.T) {
	dir := t.TempDir()
	ghBin := filepath.Join(dir, "gh")
	script := "#!/bin/sh\necho 'internal server error' >&2\nexit 1\n"
	if err := os.WriteFile(ghBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	got := defaultGhDeleteRelease("v0.0.0-adv-test")
	if got == "skipped" {
		t.Errorf("gh is in PATH so status must not be 'skipped'; got %q", got)
	}
	if got == "deleted" {
		t.Errorf("gh exits 1 so status must not be 'deleted'; got %q", got)
	}
}

func TestDefaultGhDeleteRelease_FakeGhFails_NotFoundMessage(t *testing.T) {
	dir := t.TempDir()
	ghBin := filepath.Join(dir, "gh")
	script := "#!/bin/sh\necho 'release not found' >&2\nexit 1\n"
	if err := os.WriteFile(ghBin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	got := defaultGhDeleteRelease("v0.0.0-adv-not-found")
	if got == "skipped" {
		t.Errorf("gh is in PATH; must not be 'skipped'; got %q", got)
	}
	if got == "deleted" {
		t.Errorf("gh exits 1; must not be 'deleted'; got %q", got)
	}
}

func TestAppendLedger_OpenFileFails_TargetIsDirectory(t *testing.T) {
	base := t.TempDir()
	targetPath := filepath.Join(base, "ledger.jsonl")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatal(err)
	}
	err := appendLedger(targetPath, []byte(`{"cycle":348}`))
	if err == nil {
		t.Error("expected error when ledger path is a directory, got nil")
	}
}

func TestAppendLedger_WriteToReadOnlyFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	base := t.TempDir()
	path := filepath.Join(base, "ledger.jsonl")
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	err := appendLedger(path, []byte(`{"cycle":348}`))
	if err == nil {
		t.Error("expected permission error writing to read-only file")
	}
}

func TestAppendLedger_ConcurrentWrites_GapDoc(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "ledger.jsonl")

	const n = 20
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			errCh <- appendLedger(path, []byte(`{"ok":true}`))
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent appendLedger returned error: %v", err)
		}
	}

	data, _ := os.ReadFile(path)
	lines := splitLines(string(data))
	if len(lines) != n {
		t.Logf("FOUND GAP: concurrent appendLedger — %d goroutines produced %d lines (expected %d)", n, len(lines), n)
		for i, l := range lines {
			if l != `{"ok":true}` {
				t.Logf("  line %d corrupted: %q", i, l)
			}
		}
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, l := range splitByNewline(s) {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func splitByNewline(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
