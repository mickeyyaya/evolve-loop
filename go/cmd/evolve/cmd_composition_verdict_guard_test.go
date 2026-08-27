package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmd_composition_verdict_guard_test.go — cycle-1571 H2.
//
// PR #503 run-scoped latestAuditEntry and its own comment named the full
// hazard: an unscoped lookup "can be a sibling lane's — OR A FAILED — audit".
// The filter it added closed only the sibling half (Kind/Role/GitHEAD/RunID);
// no verdict was consulted. So a FAILed audit from THIS run was still taken as
// "the audited snapshot", and RUNG 0 carry-forward would run the entire
// composed-tree gate set and write a composition-verdict record certifying the
// carry-forward of a REJECTION. Ship blocks the result downstream, so the cost
// is a wasted gate pass plus a dishonest entry in a hash-chained ledger.
//
// The verdict lives in the bound artifact, not the ledger: exit_code is 1 for
// WARN and FAIL alike, and phase_bindings.go states the artifact is where the
// severity lives. So the guard reads the artifact the entry already points at.

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

// TestRequireReusableAudit_RefusesFAIL is the pin for the live defect.
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

// TestRequireReusableAudit_AcceptsShippableVerdicts: WARN ships by default
// under the fluent-audit posture, so carrying a WARN forward is legitimate.
// Excluding it would silently narrow the fast path.
func TestRequireReusableAudit_AcceptsShippableVerdicts(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"PASS", "WARN"} {
		if err := requireReusableAudit(auditLedgerEntry{ArtifactPath: writeArtifact(t, v)}); err != nil {
			t.Errorf("verdict %s must be carry-forward eligible, got: %v", v, err)
		}
	}
}

// TestRequireReusableAudit_UnreadableArtifactFailsClosed: the snapshot's whole
// contract is that any error routes to a full re-audit. An artifact that cannot
// be read or carries no recognizable verdict must therefore REFUSE, never
// default to eligible.
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

// TestReadCompositionSnapshot_RefusesFailBeforeAnyGitWork is the WIRING proof
// (§3.3) for the guard above. Asserting requireReusableAudit in isolation would
// stay green if nothing ever called it — the exact unit-green/live-dark shape
// this PR exists to fix. So drive the real production entry point.
//
// The temp worktree is deliberately NOT a git repo: requireReusableAudit runs
// before gitDiffCapture, so a refusal here also proves the check is positioned
// early enough to spare the composed-tree gate run that motivated the fix.
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

// TestRequireReusableAudit_ForeignPhaseSentinelRefused — architect review MUST.
// ParseVerdictSentinelFull is tail-anchored, so a sentinel from another phase
// quoted into the audit artifact (a build-report block pasted as evidence) would
// otherwise be able to satisfy a carry-forward. Ship's reader states the rule
// explicitly — only an exact "audit" phase is trusted — and this reader was
// modelled on it, so it must honour the same contract.
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
