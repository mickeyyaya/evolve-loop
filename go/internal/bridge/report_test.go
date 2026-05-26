package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildReport_Verdicts(t *testing.T) {
	now := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	ws := t.TempDir()
	art := filepath.Join(ws, "artifact.md")

	// empty workspace → incomplete
	r, err := BuildReport(ws, "artifact.md", now)
	if err != nil {
		t.Fatalf("BuildReport err: %v", err)
	}
	if r.Verdict != "incomplete" {
		t.Fatalf("empty ws verdict = %q, want incomplete", r.Verdict)
	}
	if r.ScannedAt != "2026-05-26T00:00:00Z" {
		t.Fatalf("scanned_at = %q", r.ScannedAt)
	}

	// artifact present, no token file → complete
	mustWriteFile(t, art, "hello world")
	if r, _ = BuildReport(ws, "artifact.md", now); r.Verdict != "complete" || !r.Artifact.Exists {
		t.Fatalf("artifact-no-token verdict = %q (exists=%v), want complete", r.Verdict, r.Artifact.Exists)
	}

	// token file present but value not in artifact → mismatch
	mustWriteFile(t, filepath.Join(ws, "challenge-token.txt"), "TOK123\n")
	if r, _ = BuildReport(ws, "artifact.md", now); r.Verdict != "incomplete-token-mismatch" {
		t.Fatalf("token-mismatch verdict = %q", r.Verdict)
	}

	// token now present in artifact → complete + has_challenge_token
	mustWriteFile(t, art, "result has TOK123 inside")
	if r, _ = BuildReport(ws, "artifact.md", now); r.Verdict != "complete" || !r.Artifact.HasChallengeToken {
		t.Fatalf("token-match verdict = %q (hasToken=%v)", r.Verdict, r.Artifact.HasChallengeToken)
	}

	// escalation report present → escalated (takes precedence)
	mustWriteFile(t, filepath.Join(ws, "escalation-report.json"), "{}")
	if r, _ = BuildReport(ws, "artifact.md", now); r.Verdict != "escalated" {
		t.Fatalf("escalated verdict = %q", r.Verdict)
	}
	if !r.EscalationReport.Exists {
		t.Fatal("escalation_report.exists should be true")
	}
}

func TestBuildReport_NotADirectory(t *testing.T) {
	if _, err := BuildReport(filepath.Join(t.TempDir(), "missing"), "artifact.md", time.Now()); err == nil {
		t.Fatal("expected error for a non-directory workspace")
	}
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
