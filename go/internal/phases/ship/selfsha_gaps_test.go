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

func TestRun_MissingCommitMessage_Errors(t *testing.T) {
	t.Parallel()
	_, err := Run(context.Background(), Options{
		Class:       ClassCycle,
		ProjectRoot: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "commit message required") {
		t.Fatalf("want 'commit message required'; got %v", err)
	}
}

func TestRun_InvalidClass_Errors(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	_, err := Run(context.Background(), Options{
		Class:         ClassCycle,
		CommitMessage: "msg",
	})
	if err == nil || !strings.Contains(err.Error(), "ProjectRoot required") {
		t.Fatalf("want 'ProjectRoot required'; got %v", err)
	}
}

func TestVerifySelfSHA_BinaryUnreadable_Errors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opts := &Options{
		ProjectRoot:    dir,
		ShipBinaryPath: dir,
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
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary\n")
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
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-content\n")
	sha, _ := sha256File(bin)
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"`+sha+`"}`)
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
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-content-v2\n")
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
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-v3\n")
	mustWrite(t, filepath.Join(dir, ".evolve", "state.json"),
		`{"expected_ship_sha":"old-sha","expected_ship_version":"11.0.0"}`)
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
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	mustWrite(t, bin, "binary-v3\n")
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

func TestAdvanceLastCycleNumber_StateReadError_ReturnsError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":5}`)
	stateDir := filepath.Join(root, ".evolve", "state.json")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	opts := &Options{ProjectRoot: root}
	if err := advanceLastCycleNumber(opts, &RunResult{}); err == nil {
		t.Fatal("read error on state.json must propagate")
	}
}

func TestPostShip_ClassCycle_InboxPromoteErrorIsWarn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	bin := filepath.Join(root, "evolve-bin")
	mustWrite(t, bin, "fake bin\n")
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), `{"cycle_id":11}`)
	mustWrite(t, filepath.Join(root, ".evolve", "state.json"), `{}`)
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

func TestRun_ManualClass_NoStagedChanges_ExitOK(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
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

func TestReadActiveWorktree_CorruptState_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".evolve", "cycle-state.json"), "not json{")
	opts := &Options{ProjectRoot: root}
	got := readActiveWorktree(opts)
	if got != "" {
		t.Errorf("corrupt state must return empty; got %q", got)
	}
}

func TestCaptureGitOutput_RunnerError_Propagates(t *testing.T) {
	t.Parallel()
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

func TestWriteShipBinding_ValidCycleID_WritesFile(t *testing.T) {
	t.Parallel()
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
