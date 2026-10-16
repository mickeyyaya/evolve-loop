package fanoutdispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_MkdirResultsDirFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, "a\techo hi\n")
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile: cmds,
		ResultsFile:  filepath.Join(blocker, "sub", "r.tsv"),
	}, &b)
	if rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d (log=%s)", rc, ExitSetupErr, b.String())
	}
}

func TestRun_ReadCommandsError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, "a\techo hi\n")
	if err := os.Chmod(cmds, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cmds, 0o644) })
	var b bytes.Buffer
	rc := Run(Config{CommandsFile: cmds, ResultsFile: filepath.Join(dir, "r.tsv")}, &b)
	if rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d (log=%s)", rc, ExitSetupErr, b.String())
	}
}

func TestRun_WriteResultsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, "a\techo hi\n")
	resultsDir := filepath.Join(dir, "results-as-dir")
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile: cmds,
		ResultsFile:  resultsDir,
		Concurrency:  1, TimeoutSecs: 10,
	}, &b)
	if rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d (log=%s)", rc, ExitSetupErr, b.String())
	}
}

func TestRun_ConsensusCancelTerminatesSurvivors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds,
		"f1\techo 'Verdict: FAIL'; exit 1\n"+
			"f2\techo 'Verdict: FAIL'; exit 1\n"+
			"survivor\tsleep 30\n")
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile:      cmds,
		ResultsFile:       results,
		Concurrency:       3,
		TimeoutSecs:       60,
		CancelOnConsensus: true,
		ConsensusK:        2,
		ConsensusPollSecs: 2,
	}, &b)
	if rc != ExitWorkerFail {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitWorkerFail, b.String())
	}
	body, _ := os.ReadFile(results)
	if !strings.Contains(string(body), "survivor\t143\t") {
		t.Errorf("survivor should be canceled (143), got: %s", body)
	}
	if !strings.Contains(b.String(), "consensus reached") {
		t.Errorf("missing consensus-reached log: %s", b.String())
	}
}

func TestRun_ConsensusPollExitsWhenAllDone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "a\techo ok\nb\techo ok\n")
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile:      cmds,
		ResultsFile:       results,
		Concurrency:       2,
		TimeoutSecs:       10,
		CancelOnConsensus: true,
		ConsensusK:        99,
		ConsensusPollSecs: 1,
	}, &b)
	if rc != ExitOK {
		t.Errorf("rc=%d, want %d (log=%s)", rc, ExitOK, b.String())
	}
}

func TestReadCommands_OpenError(t *testing.T) {
	t.Parallel()
	if _, err := ReadCommands(filepath.Join(t.TempDir(), "nope.tsv")); err == nil {
		t.Error("expected open error for nonexistent commands file")
	}
}

func TestReadCommands_ScannerError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "huge.tsv")
	huge := bytes.Repeat([]byte("x"), 11*1024*1024)
	if err := os.WriteFile(p, huge, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCommands(p); err == nil {
		t.Error("expected scanner error on oversized line")
	}
}

func TestRun_MissingMetaWritesSentinelRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	results := filepath.Join(dir, "r.tsv")
	writeFile(t, cmds, "sub/w\techo ok\n")
	var b bytes.Buffer
	rc := Run(Config{
		CommandsFile: cmds,
		ResultsFile:  results,
		Concurrency:  1,
		TimeoutSecs:  10,
	}, &b)
	if rc != ExitWorkerFail {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitWorkerFail, b.String())
	}
	body, _ := os.ReadFile(results)
	if got := string(body); got != "sub/w\t-1\t0\n" {
		t.Errorf("results=%q, want sentinel %q", got, "sub/w\t-1\t0\n")
	}
}

func TestCheckFailConsensus_GlobError(t *testing.T) {
	t.Parallel()
	badDir := filepath.Join(t.TempDir(), "ws[")
	if checkFailConsensus(badDir, 1) {
		t.Error("malformed glob pattern must yield no consensus (false)")
	}
}

func TestCheckFailConsensus_UnreadableOutFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good.out"), "Verdict: FAIL\n")
	bad := filepath.Join(dir, "bad.out")
	writeFile(t, bad, "Verdict: FAIL\n")
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })
	if checkFailConsensus(dir, 2) {
		t.Error("unreadable .out should be skipped, so k=2 is unmet")
	}
	if !checkFailConsensus(dir, 1) {
		t.Error("readable good.out should satisfy k=1")
	}
}
