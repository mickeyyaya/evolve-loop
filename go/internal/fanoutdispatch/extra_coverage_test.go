package fanoutdispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_MkdirResultsDirFails covers the MkdirAll(resultsDir) error branch:
// the results dir's parent is a regular file, so the dir cannot be created.
func TestRun_MkdirResultsDirFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cmds := filepath.Join(dir, "cmds.tsv")
	writeFile(t, cmds, "a\techo hi\n")
	// "blocker" is a file; results dir would need to live underneath it.
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

// TestRun_ReadCommandsError covers Run's read-commands error branch: the
// commands file passes Stat but is unreadable, so the open inside fails.
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

// TestRun_WriteResultsError covers the write-results error branch: the results
// path is a directory, so the final WriteFile fails after workers complete.
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
		ResultsFile:  resultsDir, // a directory → WriteFile fails
		Concurrency:  1, TimeoutSecs: 10,
	}, &b)
	if rc != ExitSetupErr {
		t.Errorf("rc=%d, want %d (log=%s)", rc, ExitSetupErr, b.String())
	}
}

// TestRun_ConsensusCancelTerminatesSurvivors covers the consensus-cancel
// polling goroutine and runWorker's context.Canceled → 143 branch: two workers
// vote FAIL quickly, a third sleeps; once K=2 FAILs are observed the survivor
// is SIGTERM'd.
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
		Concurrency:       3, // all start immediately
		TimeoutSecs:       60,
		CancelOnConsensus: true,
		ConsensusK:        2,
		ConsensusPollSecs: 2, // margin over fast FAIL-voters' write latency on loaded CI
	}, &b)
	if rc != ExitWorkerFail {
		t.Fatalf("rc=%d, want %d (log=%s)", rc, ExitWorkerFail, b.String())
	}
	body, _ := os.ReadFile(results)
	// survivor was SIGTERM'd via canceled context → exit code 143.
	if !strings.Contains(string(body), "survivor\t143\t") {
		t.Errorf("survivor should be canceled (143), got: %s", body)
	}
	if !strings.Contains(b.String(), "consensus reached") {
		t.Errorf("missing consensus-reached log: %s", b.String())
	}
}

// TestRun_ConsensusPollExitsWhenAllDone covers the poller's all-done exit:
// consensus is enabled but K is unreachable, so the poll loop exits because
// every worker finished rather than because consensus was reached.
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
		ConsensusK:        99, // never reached
		ConsensusPollSecs: 1,
	}, &b)
	if rc != ExitOK {
		t.Errorf("rc=%d, want %d (log=%s)", rc, ExitOK, b.String())
	}
}

// TestReadCommands_OpenError covers ReadCommands' open-error branch.
func TestReadCommands_OpenError(t *testing.T) {
	t.Parallel()
	if _, err := ReadCommands(filepath.Join(t.TempDir(), "nope.tsv")); err == nil {
		t.Error("expected open error for nonexistent commands file")
	}
}

// TestReadCommands_ScannerError covers the scanner.Err() branch: a single line
// larger than the 10 MB scanner buffer triggers bufio.ErrTooLong.
func TestReadCommands_ScannerError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "huge.tsv")
	// 11 MB with no newline — exceeds the 10 MB max token size.
	huge := bytes.Repeat([]byte("x"), 11*1024*1024)
	if err := os.WriteFile(p, huge, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCommands(p); err == nil {
		t.Error("expected scanner error on oversized line")
	}
}

// TestCheckFailConsensus_UnreadableOutFile covers the per-file Open-error
// continue: a glob-matched .out file that cannot be opened is skipped.
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
	// good.out counts (1 FAIL); bad.out is skipped on open error.
	if checkFailConsensus(dir, 2) {
		t.Error("unreadable .out should be skipped, so k=2 is unmet")
	}
	if !checkFailConsensus(dir, 1) {
		t.Error("readable good.out should satisfy k=1")
	}
}
