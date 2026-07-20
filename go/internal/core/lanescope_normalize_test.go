package core

// lanescope_normalize_test.go — focused unit coverage for
// normalizeScoutGoalHash, the scout→triage lane-identity reconciliation that
// SUPERSEDED the cycle-640 hard-abort gate. The orchestrator-level pin test
// (lanescope_pin_test.go) exercises only the mismatch→stamp→proceed happy path;
// this file pins the branches that matter most for NOT regressing to a
// false-abort AND for the blast-radius / no-silent-failure guards the reviewer
// flagged: every degraded input (no pin / no report / no echo) is a silent
// no-op, a coherent report is left byte-identical, a NON-canonical echo is
// refused (never a blind whole-file replace), a valid mis-echo is corrected at
// every occurrence, and a write failure is a loud no-op — never a swallowed
// mismatch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pinGoalHash / misGoalHash are canonical 64-hex goal hashes differing by a
// single digit — mirroring the real deterministic transcription flip
// (…c05376e pinned vs …c05356e echoed) that motivated the whole fix.
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

// TestNormalizeScoutGoalHash_StampsMismatchToPin: the core reconciliation — a
// mis-echoed canonical goal_hash is rewritten to the authoritative pin, so a
// subsequent scoutReportGoalHash reads the pin, not the wrong transcription.
func TestNormalizeScoutGoalHash_StampsMismatchToPin(t *testing.T) {
	ws := t.TempDir()
	writeLaneScopeFixture(t, ws, []string{"todo-a"}, pinGoalHash)
	writeScoutReportFixture(t, ws, misGoalHash)

	normalizeScoutGoalHash(ws)

	if got := scoutReportGoalHash(ws); got != pinGoalHash {
		t.Errorf("post-normalize report goal_hash = %q, want the machine-stamped pin %q", got, pinGoalHash)
	}
}

// TestNormalizeScoutGoalHash_CoherentReportUnchanged: when the echo already
// matches the pin there is nothing to reconcile — the report must be left
// byte-identical (no needless rewrite, no spurious WARN-triggering mutation).
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

// TestNormalizeScoutGoalHash_NonCanonicalEchoRefused pins the blast-radius
// guard: a real mismatch whose echoed value is NOT a canonical 64-hex hash
// (truncation / placeholder / hallucination) must NOT trigger the whole-file
// replace — the report is left untouched so a short/generic needle can never
// corrupt unrelated content.
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

// TestNormalizeScoutGoalHash_CorrectsEveryOccurrence documents (and proves safe)
// the whole-file replace of a CANONICAL mis-echo: when the wrong hash appears in
// both the Decision Trace and report prose, every occurrence is corrected to the
// pin. Two distinct equal-length hex strings can't be substrings of each other,
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

// TestNormalizeScoutGoalHash_FailOpen pins the degraded inputs that MUST be
// no-ops — the cycle-760..762 lesson: a coherence step that touches a
// healthy/absent cycle is worse than the incoherence it chases.
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

// TestNormalizeScoutGoalHash_WriteFailureIsLoudNoOp: when the stamp write fails
// (read-only report), the function must NOT panic and must leave the on-disk
// bytes intact — the pin still governs the lane identity, and the failure is
// WARNed (not swallowed into a false green).
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
