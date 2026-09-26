package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeArtifact(t *testing.T, verdict string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "audit-report.md")
	body := "# Audit Report\n\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"" + verdict +
		"\",\"schema_version\":2} -->\n\n## Verdict\n\n**" + verdict + "**\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRequireReusableAudit_RefusesFAIL(t *testing.T) {
	t.Parallel()
	err := requireReusableAudit(auditLedgerEntry{ArtifactPath: writeArtifact(t, "FAIL")})
	if err == nil {
		t.Fatal("a FAILed audit was accepted as the carry-forward snapshot — a rejection must never be carried forward")
	}
	if !strings.Contains(err.Error(), "FAIL") {
		t.Errorf("refusal must name the offending verdict (ADR-0084 I3), got: %v", err)
	}
}

func TestRequireReusableAudit_AcceptsShippableVerdicts(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"PASS", "WARN"} {
		if err := requireReusableAudit(auditLedgerEntry{ArtifactPath: writeArtifact(t, v)}); err != nil {
			t.Errorf("verdict %s must be carry-forward eligible, got: %v", v, err)
		}
	}
}

func TestRequireReusableAudit_UnreadableArtifactFailsClosed(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "gone.md")
	if err := requireReusableAudit(auditLedgerEntry{ArtifactPath: missing}); err == nil {
		t.Error("a missing artifact must fail closed to a full re-audit, not pass")
	}

	noVerdict := filepath.Join(t.TempDir(), "audit-report.md")
	if err := os.WriteFile(noVerdict, []byte("# Audit Report\n\nno verdict here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireReusableAudit(auditLedgerEntry{ArtifactPath: noVerdict}); err == nil {
		t.Error("an artifact with no parseable verdict must fail closed, not pass")
	}

	if err := requireReusableAudit(auditLedgerEntry{ArtifactPath: ""}); err == nil {
		t.Error("an entry with no artifact_path must fail closed")
	}
}

// The worktree is deliberately not a git repo: a refusal here also proves the
// guard runs before gitDiffCapture and spares the composed-tree gate run.
func TestReadCompositionSnapshot_RefusesFailBeforeAnyGitWork(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	artifact := writeArtifact(t, "FAIL")
	if err := os.MkdirAll(filepath.Join(worktree, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"ts":"2026-08-27T12:00:00Z","cycle":1574,"run_id":"RUN-A","role":"auditor",` +
		`"kind":"agent_subprocess","exit_code":1,"artifact_path":"` + artifact + `","git_head":"deadbeef"}` + "\n"
	if err := os.WriteFile(filepath.Join(worktree, ".evolve", "ledger.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readCompositionSnapshot(context.Background(), worktree, "RUN-A")
	if err == nil {
		t.Fatal("readCompositionSnapshot accepted a FAILed audit — the verdict guard is not wired into the production path")
	}
	if !strings.Contains(err.Error(), "carry forward") {
		t.Errorf("expected the verdict refusal, got a different failure (guard may be positioned after git work): %v", err)
	}
}

func TestRequireReusableAudit_ForeignPhaseSentinelRefused(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "audit-report.md")
	body := "# Audit Report\n\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\",\"schema_version\":2} -->\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := requireReusableAudit(auditLedgerEntry{ArtifactPath: p})
	if err == nil {
		t.Fatal("a build-phase sentinel must not satisfy the audit carry-forward guard")
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("refusal should name the offending phase, got: %v", err)
	}
}
