//go:build integration

package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Run: input validation -------------------------------------------------

func TestRun_MissingCommitMessage_Errors(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Class:       ClassCycle,
		ProjectRoot: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "commit message required") {
		t.Fatalf("want 'commit message required'; got %v", err)
	}
}

func TestRun_InvalidClass_Errors(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Class:         Class("unknown"),
		CommitMessage: "msg",
		ProjectRoot:   t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "invalid --class") {
		t.Fatalf("want 'invalid --class'; got %v", err)
	}
}

func TestRun_EmptyProjectRoot_Errors(t *testing.T) {
	_, err := Run(context.Background(), Options{
		Class:         ClassCycle,
		CommitMessage: "msg",
	})
	if err == nil || !strings.Contains(err.Error(), "ProjectRoot required") {
		t.Fatalf("want 'ProjectRoot required'; got %v", err)
	}
}

// --- verifySelfSHA ---------------------------------------------------------

func TestVerifySelfSHA_BinaryUnreadable_Errors(t *testing.T) {
	dir := t.TempDir()
	opts := &Options{
		ProjectRoot:    dir,
		ShipBinaryPath: dir, // a directory, not a file — sha256File will fail
	}
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"), `{}`)
	res := &RunResult{}
	err := verifySelfSHA(context.Background(), opts, res)
	if err == nil {
		t.Fatal("sha256File on directory must return error")
	}
	if !strings.Contains(err.Error(), "cannot SHA ship binary") {
		t.Errorf("error should mention cannot SHA; got %q", err.Error())
	}
}

func TestVerifySelfSHA_StateMapReadError_Errors(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary\n")
	// Create a directory where state.json should be — readStateMap will fail.
	stateDir := filepath.Join(dir, ".evolve", "state.json")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	opts := &Options{
		ProjectRoot:    dir,
		ShipBinaryPath: bin,
	}
	res := &RunResult{}
	err := verifySelfSHA(context.Background(), opts, res)
	if err == nil {
		t.Fatal("readStateMap on directory must return error")
	}
	if !strings.Contains(err.Error(), "read state.json") {
		t.Errorf("error should mention read state.json; got %q", err.Error())
	}
}

func TestVerifySelfSHA_SchemaMigration_RepinsVersion(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-content\n")
	sha, _ := sha256File(bin)
	// expectedSHA matches; expectedVer is absent; pluginVer is set.
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"`+sha+`"}`)
	// Create plugin.json so pluginVer is non-empty.
	mustWrite(t, filepath.Join(dir, ".claude-plugin", "plugin.json"),
		`{"version":"12.5.0"}`)
	opts := &Options{
		ProjectRoot:    dir,
		PluginRoot:     dir,
		ShipBinaryPath: bin,
	}
	res := &RunResult{}
	if err := verifySelfSHA(context.Background(), opts, res); err != nil {
		t.Fatalf("schema migration must succeed; got %v", err)
	}
	if !anyContains(res.Logs, "schema migration") {
		t.Errorf("missing schema migration log; got %v", res.Logs)
	}
	m, _ := readStateMap(filepath.Join(dir, ".evolve", "state.json"))
	if stateString(m, "expected_ship_version") != "12.5.0" {
		t.Errorf("expected_ship_version not pinned; got %v", m["expected_ship_version"])
	}
}

func TestVerifySelfSHA_LegacySHAOnlyPin_Migrates(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-content-v2\n")
	// Stale SHA in state.json (no expectedVer).
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"stale-sha"}`)
	opts := &Options{
		ProjectRoot:    dir,
		PluginRoot:     dir,
		ShipBinaryPath: bin,
	}
	res := &RunResult{}
	if err := verifySelfSHA(context.Background(), opts, res); err != nil {
		t.Fatalf("legacy migration must succeed; got %v", err)
	}
	if !anyContains(res.Logs, "migrating legacy SHA-only pin") {
		t.Errorf("missing legacy migration log; got %v", res.Logs)
	}
}

func TestVerifySelfSHA_PluginVersionChange_Repins(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-v3\n")
	// State has old SHA + old version.
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"old-sha","expected_ship_version":"11.0.0"}`)
	// New plugin version.
	mustWrite(t, filepath.Join(dir, ".claude-plugin", "plugin.json"),
		`{"version":"12.0.0"}`)
	opts := &Options{
		ProjectRoot:    dir,
		PluginRoot:     dir,
		ShipBinaryPath: bin,
	}
	res := &RunResult{}
	if err := verifySelfSHA(context.Background(), opts, res); err != nil {
		t.Fatalf("version change repin must succeed; got %v", err)
	}
	if !anyContains(res.Logs, "plugin version changed") {
		t.Errorf("missing plugin-version-changed log; got %v", res.Logs)
	}
}

func TestVerifySelfSHA_SameVersionSHATamper_IntegrityError(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-v3\n")
	// State: correct version, wrong SHA (tampering scenario).
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"expected-but-different","expected_ship_version":"12.0.0"}`)
	mustWrite(t, filepath.Join(dir, ".claude-plugin", "plugin.json"),
		`{"version":"12.0.0"}`)
	opts := &Options{
		ProjectRoot:    dir,
		PluginRoot:     dir,
		ShipBinaryPath: bin,
	}
	res := &RunResult{}
	err := verifySelfSHA(context.Background(), opts, res)
	var ie *IntegrityError
	if !errors.As(err, &ie) {
		t.Fatalf("same-version SHA mismatch must be IntegrityError; got %T: %v", err, err)
	}
	if !strings.Contains(ie.Msg, "modified WITHIN plugin version") {
		t.Errorf("error should mention 'modified WITHIN plugin version'; got %q", ie.Msg)
	}
}

// --- advanceLastCycleNumber: state.json read error -------------------------

func TestAdvanceLastCycleNumber_StateReadError_ReturnsError(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":5}`)
	// Create state.json as a directory — readStateMap will error.
	stateDir := filepath.Join(root, ".evolve", "state.json")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	opts := &Options{ProjectRoot: root}
	if err := advanceLastCycleNumber(opts, &RunResult{}); err == nil {
		t.Fatal("read error on state.json must propagate")
	}
}

// --- postShip: cycle class with inbox-promote error silently WARNs ----------

// TestPostShip_ClassCycle_InboxPromoteErrorIsWarn: an unreadable (present but
// corrupt/dir) triage-decision.json must NOT block ship — it logs a WARN and
// still proceeds to the DONE log. Distinct from an ABSENT companion (which
// logs INFO).
func TestPostShip_ClassCycle_InboxPromoteErrorIsWarn(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "fake bin\n")
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":11}`)
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"), `{}`)
	// A directory in place of the file forces ReadFile to error, simulating an
	// unreadable companion.
	triagePath := filepath.Join(root, ".evolve", "runs", "cycle-11", "triage-decision.json")
	if err := os.MkdirAll(triagePath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	opts := &Options{
		Class:          ClassCycle,
		ProjectRoot:    root,
		ShipBinaryPath: bin,
		Stderr:         io.Discard,
	}
	res := &RunResult{ClassUsed: ClassCycle, CommitSHA: "abc"}
	err := postShip(context.Background(), opts, res)
	if err != nil {
		t.Fatalf("postShip must not fail on inbox-promote error; got %v", err)
	}
	if !containsLog(*res, "DONE: shipped cycle at abc") {
		t.Errorf("missing DONE log after inbox-promote warn; got %v", res.Logs)
	}
	if !containsLog(*res, "WARN: triage-decision.json unreadable") {
		t.Errorf("expected unreadable-companion WARN; got %v", res.Logs)
	}
}

// --- Run: cleanExitError path (no staged changes in manual class) ----------

func TestRun_ManualClass_NoStagedChanges_ExitOK(t *testing.T) {
	repo := makeRepo(t) // clean tree, no staged changes
	// No seedAudit needed — manual class skips audit.
	res, err := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "manual: clean exit",
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if err != nil {
		t.Fatalf("clean-exit manual should not error; got %v", err)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
}

// --- readActiveWorktree: corrupt cycle-state.json --------------------------

func TestReadActiveWorktree_CorruptState_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), "not json{")
	opts := &Options{ProjectRoot: root}
	got := readActiveWorktree(opts)
	if got != "" {
		t.Errorf("corrupt state must return empty; got %q", got)
	}
}

// --- captureGitOutput: runner error propagates ----------------------------

func TestCaptureGitOutput_RunnerError_Propagates(t *testing.T) {
	errRunner := func(ctx context.Context, name, cwd string, args, env []string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return -1, errors.New("runner exploded")
	}
	opts := &Options{ProjectRoot: t.TempDir(), Runner: errRunner}
	_, err := captureGitOutput(context.Background(), opts, "rev-parse", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "ship: git") {
		t.Fatalf("runner error must propagate; got %v", err)
	}
}

// --- writeShipBinding: cycle_id present, successful write -----------------

func TestWriteShipBinding_ValidCycleID_WritesFile(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":42}`)
	opts := &Options{ProjectRoot: root}
	if err := writeShipBinding(opts, "treetree", "commitcommit"); err != nil {
		t.Fatalf("writeShipBinding errored: %v", err)
	}
	bindPath := filepath.Join(root, ".evolve", "runs", "cycle-42", "ship-binding.json")
	if _, err := os.Stat(bindPath); err != nil {
		t.Fatalf("ship-binding.json not created: %v", err)
	}
}
