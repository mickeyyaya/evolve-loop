package main

// cmd_inbox_mover_exitcodes_test.go — ADR-0103 unit 06 step 0 (test 14): the
// exit map's 2 (mv failed) and 3 (console-routed) arms, which the existing
// cmd tests never reached, pinned before the mover moves.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInboxMover_ExitCodes2And3(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	inbox := filepath.Join(d, ".evolve", "inbox")
	// A FILE at processing/ makes the claim's mkdir fail → rc 2.
	if err := os.WriteFile(filepath.Join(inbox, "processing"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runInboxMover([]string{"claim", "task-1", "5"}, nil, &stdout, &stderr); rc != 2 {
		t.Errorf("claim with a FILE at processing/: rc = %d, want 2\nstderr=%s", rc, stderr.String())
	}
	if err := os.Remove(filepath.Join(inbox, "processing")); err != nil {
		t.Fatal(err)
	}
	// A route:console-* item → rc 3.
	if err := os.WriteFile(filepath.Join(inbox, "ops.json"), []byte(`{"id":"ops","route":"console-manual"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if rc := runInboxMover([]string{"claim", "ops", "5"}, nil, &stdout, &stderr); rc != 3 {
		t.Errorf("claim of a console-routed item: rc = %d, want 3\nstderr=%s", rc, stderr.String())
	}
	// A FILE at processed/ makes the promote's mkdir fail → rc 2 with the cmd
	// layer's own non-delivery line.
	if err := os.WriteFile(filepath.Join(inbox, "processed"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if rc := runInboxMover([]string{"promote", "task-1", "processed", "5"}, nil, &stdout, &stderr); rc != 2 {
		t.Errorf("promote with a FILE at processed/: rc = %d, want 2\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "promote did not deliver 'task-1'") {
		t.Errorf("the cmd layer's non-delivery line: %q", stderr.String())
	}
}
