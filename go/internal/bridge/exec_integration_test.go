package bridge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// exec_integration_test.go — covers the production exec glue (execRunner,
// execTmux) against real /bin/sh and real tmux. The tmux block skips when
// tmux is absent, so it covers tmux.go wherever tmux is installed.

func TestExecRunner_Real(t *testing.T) {
	var out bytes.Buffer
	// success (dir="" → inherit caller cwd, unchanged behavior)
	code, err := execRunner(context.Background(), "sh", "", []string{"-c", "exit 0"}, os.Environ(), nil, &out, &out)
	if err != nil || code != 0 {
		t.Fatalf("exit 0: code=%d err=%v", code, err)
	}
	// non-zero exit → (code, nil)
	code, err = execRunner(context.Background(), "sh", "", []string{"-c", "exit 7"}, nil, nil, &out, &out)
	if err != nil || code != 7 {
		t.Fatalf("exit 7: code=%d err=%v, want (7,nil)", code, err)
	}
	// binary not found → (-1, err)
	code, err = execRunner(context.Background(), "/no/such/binary-cov-xyz", "", nil, nil, nil, &out, &out)
	if err == nil || code != -1 {
		t.Fatalf("missing binary: code=%d err=%v, want (-1, err)", code, err)
	}
	// dir != "" → subprocess cwd is set to dir (the worktree-cwd fix).
	dir := t.TempDir()
	out.Reset()
	code, err = execRunner(context.Background(), "sh", dir, []string{"-c", "pwd"}, nil, nil, &out, &out)
	if err != nil || code != 0 {
		t.Fatalf("pwd: code=%d err=%v", code, err)
	}
	// macOS /tmp is a symlink to /private/tmp, so resolve both sides before compare.
	gotPwd, _ := filepath.EvalSymlinks(strings.TrimSpace(out.String()))
	wantPwd, _ := filepath.EvalSymlinks(dir)
	if gotPwd != wantPwd {
		t.Fatalf("subprocess cwd = %q, want dir %q", gotPwd, wantPwd)
	}
}

func TestExecTmux_Real(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed; skipping execTmux integration coverage")
	}
	tx := execTmux{}
	ctx := context.Background()
	sess := fmt.Sprintf("evolve-bridge-covtest-%d", os.Getpid())
	defer func() { _ = tx.KillSession(ctx, sess) }()

	if tx.HasSession(ctx, sess) {
		t.Fatalf("session %s should not exist yet", sess)
	}
	if err := tx.NewSession(ctx, sess, 80, 24); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if !tx.HasSession(ctx, sess) {
		t.Fatal("HasSession should be true after NewSession")
	}
	if err := tx.SendKeys(ctx, sess, "echo cov-marker", true); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}
	if err := tx.SendKeys(ctx, sess, "", true); err != nil { // Enter only
		t.Fatalf("SendKeys(enter): %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	if _, err := tx.CapturePane(ctx, sess, 0); err != nil {
		t.Fatalf("CapturePane visible: %v", err)
	}
	if _, err := tx.CapturePane(ctx, sess, 200); err != nil {
		t.Fatalf("CapturePane scrollback: %v", err)
	}
	bufFile := filepath.Join(t.TempDir(), "buf.txt")
	if err := os.WriteFile(bufFile, []byte("paste-cov\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := tx.LoadBuffer(ctx, sess, bufFile); err != nil {
		t.Fatalf("LoadBuffer: %v", err)
	}
	if err := tx.PasteBuffer(ctx, sess); err != nil {
		t.Fatalf("PasteBuffer: %v", err)
	}
	if err := tx.KillSession(ctx, sess); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	if tx.HasSession(ctx, sess) {
		t.Fatal("HasSession should be false after KillSession")
	}
}

func TestStripANSI(t *testing.T) {
	in := "\x1b[31mred\x1b[0m plain \x1b]0;title\x07end"
	if got := stripANSI(in); got != "red plain end" {
		t.Fatalf("stripANSI = %q, want %q", got, "red plain end")
	}
}
