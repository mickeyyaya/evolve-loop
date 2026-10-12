package aggregator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCrossCLIVote_ReadFailureAbortsBeforeWritingTheReport(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "consensus.md")
	boom := errors.New("worker report unreadable")
	read := func(string) ([]byte, error) { return nil, boom }

	code, err := writeCrossCLIVote(out, []string{filepath.Join(dir, "claude.md")}, "2026-10-13T00:00:00Z", read)
	if !errors.Is(err, boom) || code != ExitUsageErr {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Errorf("no consensus file may exist after a read failure: %v", statErr)
	}
}

func TestWriteCrossCLIVote_UnwritableOutputReturnsUsageError(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "missing-dir", "consensus.md")
	read := func(string) ([]byte, error) { return []byte("Verdict: PASS"), nil }

	code, err := writeCrossCLIVote(out, []string{filepath.Join(dir, "claude.md")}, "now", read)
	if err == nil || code != ExitUsageErr {
		t.Fatalf("code=%d err=%v", code, err)
	}
}

func TestWriteCrossCLIVote_FailVetoWritesReportAndReturnsVerdictBad(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "consensus.md")
	workers := []string{filepath.Join(dir, "claude.md"), filepath.Join(dir, "codex.md")}
	for i, body := range []string{"Verdict: PASS\n", "Verdict: FAIL\nreason\n"} {
		if err := os.WriteFile(workers[i], []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := os.ReadFile

	code, err := writeCrossCLIVote(out, workers, "now", read)
	if err != nil || code != ExitVerdictBad {
		t.Fatalf("code=%d err=%v", code, err)
	}
	body, _ := os.ReadFile(out)
	for _, want := range []string{"Verdict: FAIL", "claude=PASS, codex=FAIL", "### Worker: codex"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("report missing %q:\n%s", want, body)
		}
	}
}
