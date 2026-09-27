package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pinGoalHash / misGoalHash are canonical 64-hex goal hashes differing by a
// single digit, mirroring a real single-digit transcription flip.
var (
	pinGoalHash = strings.Repeat("a", 57) + "c05376e"
	misGoalHash = strings.Repeat("a", 57) + "c05356e"
)

// writeScoutReportFixture writes <workspace>/scout-report.md whose Decision
// Trace fenced-json block carries the given goal_hash. An empty goalHash writes
// a trace with NO goal_hash key (the fail-open "report without the echo" case).
func writeScoutReportFixture(t *testing.T, workspace, goalHash string) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	trace := `{"mode": "incremental"}`
	if goalHash != "" {
		trace = `{"mode": "incremental", "goal_hash": "` + goalHash + `"}`
	}
	report := "# Scout Report — test\n\n## Decision Trace\n\n```json\n" + trace + "\n```\n"
	if err := os.WriteFile(filepath.Join(workspace, "scout-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("write scout-report.md: %v", err)
	}
}

func readScoutReport(t *testing.T, workspace string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(workspace, "scout-report.md"))
	if err != nil {
		t.Fatalf("read scout-report.md: %v", err)
	}
	return string(b)
}

func TestNormalizeScoutGoalHash_StampsMismatchToPin(t *testing.T) {
	ws := t.TempDir()
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
	writeScoutReportFixture(t, ws, misGoalHash)

	normalizeScoutGoalHash(ws)

	if got := scoutReportGoalHash(ws); got != pinGoalHash {
		t.Errorf("post-normalize report goal_hash = %q, want the machine-stamped pin %q", got, pinGoalHash)
	}
}

func TestNormalizeScoutGoalHash_CoherentReportUnchanged(t *testing.T) {
	ws := t.TempDir()
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
	writeScoutReportFixture(t, ws, pinGoalHash)
	before := readScoutReport(t, ws)

	normalizeScoutGoalHash(ws)

	if after := readScoutReport(t, ws); after != before {
		t.Errorf("coherent report was mutated:\nbefore=%q\nafter =%q", before, after)
	}
}

func TestNormalizeScoutGoalHash_NonCanonicalEchoRefused(t *testing.T) {
	ws := t.TempDir()
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
	writeScoutReportFixture(t, ws, "goal-1") // detected mismatch, but not 64-hex
	before := readScoutReport(t, ws)

	normalizeScoutGoalHash(ws)

	if after := readScoutReport(t, ws); after != before {
		t.Errorf("non-canonical echo triggered a rewrite:\nbefore=%q\nafter =%q", before, after)
	}
}

// Two distinct equal-length hex strings can't be substrings of each other,
// so a canonical needle only ever replaces genuine echoes.
func TestNormalizeScoutGoalHash_CorrectsEveryOccurrence(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
	report := "# Scout\n\nSelected goal " + misGoalHash + " for this lane.\n\n" +
		"## Decision Trace\n\n```json\n{\"goal_hash\": \"" + misGoalHash + "\"}\n```\n"
	if err := os.WriteFile(filepath.Join(ws, "scout-report.md"), []byte(report), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	normalizeScoutGoalHash(ws)

	after := readScoutReport(t, ws)
	if strings.Contains(after, misGoalHash) {
		t.Error("a mis-echoed occurrence survived — the correction was not global")
	}
	if n := strings.Count(after, pinGoalHash); n != 2 {
		t.Errorf("pin appears %d times, want 2 (prose + Decision Trace both corrected)", n)
	}
}

func TestNormalizeScoutGoalHash_FailOpen(t *testing.T) {
	t.Run("no pin leaves report untouched", func(t *testing.T) {
		ws := t.TempDir()
		writeScoutReportFixture(t, ws, misGoalHash) // no lane-scope.json
		before := readScoutReport(t, ws)

		normalizeScoutGoalHash(ws)

		if after := readScoutReport(t, ws); after != before {
			t.Errorf("report mutated with no pin present:\nbefore=%q\nafter =%q", before, after)
		}
	})

	t.Run("no report is a safe no-op", func(t *testing.T) {
		ws := t.TempDir()
		writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash) // no scout-report.md

		normalizeScoutGoalHash(ws) // must not panic / create a file

		if _, err := os.Stat(filepath.Join(ws, "scout-report.md")); !os.IsNotExist(err) {
			t.Errorf("normalize fabricated a scout-report.md from a bare pin (err=%v)", err)
		}
	})

	t.Run("report without a goal_hash echo is left untouched", func(t *testing.T) {
		ws := t.TempDir()
		writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
		writeScoutReportFixture(t, ws, "") // trace present, goal_hash key absent
		before := readScoutReport(t, ws)

		normalizeScoutGoalHash(ws)

		if after := readScoutReport(t, ws); after != before {
			t.Errorf("report with no goal_hash echo was mutated:\nbefore=%q\nafter =%q", before, after)
		}
	})
}

func TestNormalizeScoutGoalHash_WriteFailureIsLoudNoOp(t *testing.T) {
	ws := t.TempDir()
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
	writeScoutReportFixture(t, ws, misGoalHash)
	reportPath := filepath.Join(ws, "scout-report.md")
	before := readScoutReport(t, ws)

	if err := os.Chmod(reportPath, 0o444); err != nil {
		t.Fatalf("chmod read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(reportPath, 0o644) }) // let TempDir clean up

	normalizeScoutGoalHash(ws) // write fails → loud no-op, no panic

	_ = os.Chmod(reportPath, 0o644)
	if after := readScoutReport(t, ws); after != before {
		t.Errorf("report changed despite a failed write:\nbefore=%q\nafter =%q", before, after)
	}
}
