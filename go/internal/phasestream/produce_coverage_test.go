package phasestream

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProduce_StdoutUnreadable covers the classifyLines open-error branch:
// the stdout log exists (stat succeeds) but can't be opened. Skips as root,
// where chmod 000 doesn't block reads.
func TestProduce_StdoutUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	ws := t.TempDir()
	logPath := filepath.Join(ws, "build-stdout.log")
	writeRaw(t, ws, "build-stdout.log", `{"type":"result","total_cost_usd":0.1,"usage":{}}`+"\n")
	if err := os.Chmod(logPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(logPath, 0o644)

	if err := Produce(ProduceConfig{Workspace: ws, Phase: "build", CLI: "claude-p", Cycle: 1}); err == nil {
		t.Fatal("expected an error when the stdout log is unreadable")
	}
}

// TestProduce_WorkspaceUnwritable covers the writeEventsFile CreateTemp
// error branch: the stdout log is readable but the workspace dir can't be
// written (so the atomic temp can't be created). Skips as root.
func TestProduce_WorkspaceUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — dir perms don't block writes")
	}
	ws := t.TempDir()
	writeRaw(t, ws, "scout-stdout.log", `{"type":"result","total_cost_usd":0.1,"usage":{}}`+"\n")
	if err := os.Chmod(ws, 0o500); err != nil { // r-x: can read the log, can't create temp
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(ws, 0o755)

	if err := Produce(ProduceConfig{Workspace: ws, Phase: "scout", CLI: "claude-p", Cycle: 1}); err == nil {
		t.Fatal("expected an error when the workspace is unwritable")
	}
}

// TestProduce_RenameFails covers the writeEventsFile rename-error branch:
// the destination <phase>-events.ndjson already exists as a directory, so the
// atomic os.Rename of the temp file over it fails. Verifies Produce surfaces
// the error and removes the orphaned temp (no .tmp left behind).
func TestProduce_RenameFails(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeRaw(t, ws, "audit-stdout.log", `{"type":"result","total_cost_usd":0.1,"usage":{}}`+"\n")
	// Make the destination a directory — rename(file, dir) fails.
	if err := os.Mkdir(filepath.Join(ws, "audit-events.ndjson"), 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}

	if err := Produce(ProduceConfig{Workspace: ws, Phase: "audit", CLI: "claude-p", Cycle: 1}); err == nil {
		t.Fatal("expected a rename error when the events path is a directory")
	}
	entries, _ := os.ReadDir(ws)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || (len(e.Name()) > 0 && e.Name()[0] == '.') {
			t.Errorf("orphaned temp left behind: %s", e.Name())
		}
	}
}

// TestProduce_StderrUnreadable covers the classifyLines open-error branch on
// the stderr leg specifically (stdout fine, stderr present but unreadable).
func TestProduce_StderrUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 000 doesn't block reads")
	}
	ws := t.TempDir()
	writeRaw(t, ws, "tdd-stdout.log", `{"type":"result","total_cost_usd":0.1,"usage":{}}`+"\n")
	errPath := filepath.Join(ws, "tdd-stderr.log")
	writeRaw(t, ws, "tdd-stderr.log", "EPERM\n")
	if err := os.Chmod(errPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(errPath, 0o644)

	if err := Produce(ProduceConfig{Workspace: ws, Phase: "tdd", CLI: "claude-p", Cycle: 1}); err == nil {
		t.Fatal("expected an error when the stderr log is unreadable")
	}
}
