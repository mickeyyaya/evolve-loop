//go:build integration

package bridge

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A line-reading shell fixture cannot expose paste corruption in a TUI. This
// child requests bracketed paste and treats Enter outside a paste as submission.
// Drive the real shared REPL path and compare every byte, including newlines.
func TestRealTmux_BracketedMultilinePromptPreservesBytes(t *testing.T) {
	requireTmux(t)
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 required for raw-terminal fixture")
	}
	const marker = "RAW-INPUT-READY"
	prompt := "HEAD: author the complete task\n" + strings.Repeat("middle: 台灣 λ 'quoted' $HOME `literal`\n", 1200) + "TAIL: end of contract"
	cfg := itConfig(t, prompt)
	script := fmt.Sprintf(`import os, sys, tty
from pathlib import Path
tty.setraw(sys.stdin.fileno())
os.write(1, b'\x1b[?2004h' + %q.encode() + b'\r\n')
buf = bytearray()
in_paste = False
escape = bytearray()
while True:
    ch = os.read(0, 1)
    if not ch:
        raise SystemExit(2)
    if escape or ch == b'\x1b':
        escape += ch
        if escape == b'\x1b[200~':
            in_paste = True
            escape.clear()
        elif escape == b'\x1b[201~':
            in_paste = False
            escape.clear()
        continue
    if ch in (b'\r', b'\n') and not in_paste:
        Path(%q).write_bytes(buf)
        os.write(1, b'\r\n' + %q.encode() + b'\r\n')
        # Preserve the first submitted message; cleanup may send more input.
        while os.read(0, 1):
            pass
        break
    buf += ch
`, marker, cfg.Artifact, marker)
	path := filepath.Join(cfg.Worktree, "raw-repl.py")
	mustWrite(t, path, script)
	session := itSession("prompt-bytes")
	defer itTmuxCtl.KillSession(context.Background(), session)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rc, err := runTmuxREPL(ctx, cfg, itDeps(150*time.Millisecond), itLaunch(session, shellQuotePOSIX(python)+" "+shellQuotePOSIX(path), marker, 0, false))
	if err != nil || rc != ExitOK {
		t.Fatalf("REPL launch: rc=%d err=%v", rc, err)
	}
	// runTmuxREPL appends one LF when materializing resolved-prompt.txt.
	want := prompt + "\n"
	got := readFile(t, cfg.Artifact)
	if got != want {
		t.Fatalf("prompt changed in transit: received %d bytes, want all %d bytes (head, body, tail and LF intact)", len(got), len(want))
	}
}
